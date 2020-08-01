package main

import log "github.com/sirupsen/logrus"

type PeerClient struct {
	*Connection
}

func (client *PeerClient) Login() {

}

func (client *PeerClient) HandleMessage(msg *Message) {
	switch msg.cmd {
	case MSG_IM:
		client.HandleIMMessage(msg)
	case MSG_SYNC:
		client.HandleSync(msg.body.(*SyncKey))
	case MSG_SYNC_KEY: //客服端->服务端,更新服务器的syncKey
		client.HandleSyncKey(msg.body.(*SyncKey))
	}
}

func (client *PeerClient) HandleSyncKey(syncKey *SyncKey) {
	lastId := syncKey.syncKey
	log.Infof("sync key:%d %d %d %d", client.appId, client.uid, client.deviceID, lastId)

	if lastId > 0 {
		s := &SyncHistory{
			AppID:     client.appId,
			UID:       client.uid,
			LastMsgID: lastId,
		}
		syncChan <- s
	}
}

func (client *PeerClient) HandleSync(syncKey *SyncKey) {
	lastId := syncKey.syncKey

	rpc := GetStorageRPCClient(client.uid)

	s := &SyncHistory{
		AppID:     client.appId,
		UID:       client.uid,
		DeviceID:  client.deviceID,
		LastMsgID: lastId,
	}

	log.Infof("syncing message:%d %d %d %d", client.appId, client.uid, client.deviceID, lastId)

	resp, err := rpc.Call("SyncMessage", s)
	if err != nil {
		log.Warning("sync message err:", err)
		return
	}

	ph := resp.(*PeerHistoryMessage)
	messages := ph.Messages

	msgs := make([]*Message, 0, len(messages)+2)

	sk := &SyncKey{syncKey: lastId}
	msgs = append(msgs, &Message{cmd: MSG_SYNC_BEGIN, body: sk})

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		log.Info("message:", msg.MsgID, Command(msg.Cmd))
		m := &Message{cmd: int(msg.Cmd), version: DEFAULT_VERSION}
		m.FromData(msg.Raw)
		sk.syncKey = msg.MsgID
		msgs = append(msgs, m)
	}

	msgs = append(msgs, &Message{cmd: MSG_SYNC_END, body: sk})

	client.EnqueueMessages(msgs)

	if ph.HasMore {
		notify := &Message{cmd: MSG_SYNC_NOTIFY, body: &SyncKey{ph.LastMsgId + 1}}
		client.EnqueueMessage(notify)
	}
}

func (client *PeerClient) HandleIMMessage(message *Message) {
	msg := message.body.(*IMMessage)
	seq := message.seq

	m := &Message{cmd: MSG_IM, version: DEFAULT_VERSION, body: msg}
	msgId, _, err := SaveMessage(client.appId, msg.receiver, client.deviceID, m)
	if err != nil {
		log.Errorf("保存peer消息: %d %d 失败 err: ", msg.sender, msg.receiver, err)
		return
	}

	// 保存到自己的消息队列，用户的其他登录点也能接受到自己发出的消息
	msgId2, prevMsgId2, err := SaveMessage(client.appId, msg.sender, client.deviceID, m)
	if err != nil {
		log.Errorf("save peer message:%d %d err:", msg.sender, msg.receiver, err)
		return
	}

	meta := &Metadata{syncKey: msgId2, prevSyncKey: prevMsgId2}
	ack := &Message{cmd: MSG_ACK, body: &MessageACK{seq: int32(seq)}, meta: meta}
	r := client.EnqueueMessage(ack)
	if !r {
		log.Warning("send peer message ack error")
	}
	log.Infof("peer message sender:%d receiver:%d msgId:%d\n", msg.sender, msg.receiver, msgId)
}
