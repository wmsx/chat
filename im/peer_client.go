package main

import log "github.com/sirupsen/logrus"

type PeerClient struct {
	*Connection
}

func (client *PeerClient) Login() {
	channel := GetChannel(client.uid)
	channel.Subscribe(client.uid, client.online)
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
	log.WithFields(log.Fields{
		"uid":      client.uid,
		"deviceID": client.deviceID,
		"lastId":   lastId,
	}).Info("HandleSyncKey")

	if lastId > 0 {
		s := &SyncHistory{
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
		UID:       client.uid,
		DeviceID:  client.deviceID,
		LastMsgID: lastId,
	}

	log.WithFields(log.Fields{
		"uid":      client.uid,
		"deviceID": client.deviceID,
		"lastId":   lastId,
	}).Info("syncing message")

	resp, err := rpc.Call("SyncMessage", s)
	if err != nil {
		log.WithField("err", err).Warning("sync message")
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
	msgId, prevMsgId, err := SaveMessage(msg.receiver, client.deviceID, m)
	if err != nil {
		log.WithFields(log.Fields{"sender": msg.sender, "receiver": msg.receiver, "err": err}).Error("保存peer消息失败")
		return
	}

	// 保存到自己的消息队列，用户的其他登录点也能接受到自己发出的消息
	msgId2, prevMsgId2, err := SaveMessage(msg.sender, client.deviceID, m)
	if err != nil {
		log.WithFields(log.Fields{"sender": msg.sender, "receiver": msg.receiver, "err": err}).Error("保存peer消息失败")
		return
	}

	// 推送给接受方
	meta := &Metadata{syncKey: msgId, prevSyncKey: prevMsgId}
	m1 := &Message{cmd: MSG_IM, version: DEFAULT_VERSION, flag: message.flag | MESSAGE_FLAG_PUSH, body: msg, meta: meta}
	client.SendMessage(msg.receiver, m1)
	notify := &Message{cmd: MSG_SYNC_NOTIFY, body: &SyncKey{syncKey: msgId}}
	client.SendMessage(msg.receiver, notify)

	// 给发送发发送ack
	meta = &Metadata{syncKey: msgId2, prevSyncKey: prevMsgId2}
	ack := &Message{cmd: MSG_ACK, body: &MessageACK{seq: int32(seq)}, meta: meta}
	r := client.EnqueueMessage(ack)
	if !r {
		log.Warning("发送peer message ack失败")
	}
	log.WithFields(log.Fields{"sender": msg.sender, "receiver": msg.receiver, "msgId": msgId}).Infof("保存peer消息成功")
}

func (client *PeerClient) Logout() {
	if client.uid > 0 {
		channel := GetChannel(client.uid)
		channel.Unsubscribe(client.uid, client.online)
	}
}
