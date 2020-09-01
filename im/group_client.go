package main

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"time"
)

type GroupClient struct {
	*Connection
}

func (client *GroupClient) HandleMessage(msg *Message) {
	switch msg.cmd {
	case MSG_GROUP_IM:
		client.HandleGroupIMMessage(msg)
	case MSG_SYNC_GROUP:
		client.HandleGroupSync(msg.body.(*GroupSyncKey))
	case MSG_GROUP_SYNC_KEY:
		client.HandleGroupSyncKey(msg.body.(*GroupSyncKey))
	}
}

func (client *GroupClient) HandleGroupIMMessage(message *Message) {
	msg := message.body.(*IMMessage)
	seq := message.seq

	msg.timestamp = int32(time.Now().Unix())

	deliver := GetGroupMessageDeliver(msg.receiver)
	group := deliver.LoadGroup(msg.receiver)
	if group == nil {
		log.Warning("can't find group:", msg.receiver)
		return
	}

	if group.GetMemberMute(msg.sender) {
		log.Warningf("sender:%d被禁言", msg.sender)
		return
	}

	var meta *Metadata
	var flag int
	if group.super {
		msgId, prevMsgId, err := client.HandleSuperGroupMessage(msg, group)
		if err == nil {
			meta = &Metadata{syncKey: msgId, prevSyncKey: prevMsgId}
		}
		flag = MESSAGE_FLAG_SUPER_GROUP
	} else {
		msgId, prevMsgId, err := client.HandleGroupMessage(msg, group)
		if err == nil {
			meta = &Metadata{syncKey: msgId, prevSyncKey: prevMsgId}
		}
	}

	ack := &Message{cmd: MSG_ACK, flag: flag, body: &MessageACK{seq: int32(seq)}, meta: meta}
	r := client.EnqueueMessage(ack)
	if !r {
		log.Warning("发送群组消息ack失败")
	}
	log.WithFields(log.Fields{"sender": msg.sender, "receiver": msg.receiver, "是否超级群": group.super}).Info("发送群组消息成功")
	if meta != nil {
		log.WithFields(log.Fields{"syncKey": meta.syncKey, "prevSyncKey": meta.prevSyncKey}).Info("发送群组消息ack meta数据")
	}
}

func (client *GroupClient) HandleGroupMessage(im *IMMessage, group *Group) (int64, int64, error) {
	gm := &PendingGroupMessage{}
	gm.sender = im.sender
	gm.deviceID = client.deviceID
	gm.gid = im.receiver
	gm.timestamp = im.timestamp

	members := group.Members()
	gm.members = make([]int64, len(members))

	i := 0
	for uid := range members {
		gm.members[i] = uid
		i++
	}

	gm.content = im.content

	deliver := GetGroupMessageDeliver(group.gid)
	m := &Message{cmd: MSG_PENDING_GROUP_MESSAGE, body: gm}

	c := make(chan *Metadata, 1)
	callbackId := deliver.SaveMessage(m, c)
	defer deliver.RemoveCallback(callbackId)
	select {
	case meta := <-c:
		return meta.syncKey, meta.prevSyncKey, nil
	case <-time.After(2 * time.Second):
		log.WithFields(log.Fields{"sender": im.sender, "receiver": im.receiver}).Error("save group message超时")
		return 0, 0, errors.New("timeout")
	}
}

func (client *GroupClient) HandleSuperGroupMessage(msg *IMMessage, group *Group) (int64, int64, error) {
	m := &Message{cmd: MSG_GROUP_IM, version: DEFAULT_VERSION, body: msg}
	msgId, prevMsgId, err := SaveGroupMessage( msg.receiver, client.deviceID, m)
	if err != nil {
		log.WithFields(log.Fields{"sender:": msg.sender, "receiver": msg.receiver, "err": err}).Error("保存群组消息失败")
		return 0, 0, nil
	}

	m.meta = &Metadata{syncKey: msgId, prevSyncKey: prevMsgId}
	m.flag = MESSAGE_FLAG_PUSH | MESSAGE_FLAG_SUPER_GROUP
	client.sendGroupMessage(group, m)

	notify := &Message{cmd: MSG_SYNC_GROUP_NOTIFY, body: &GroupSyncKey{groupId: msg.receiver, syncKey: msgId}}
	client.sendGroupMessage(group, notify)
	return msgId, prevMsgId, nil
}

func (client *GroupClient) HandleGroupSync(groupSyncKey *GroupSyncKey) {
	groupId := groupSyncKey.groupId
	group := groupManager.LoadGroup(groupId)
	if group == nil {
		log.WithField("groupId", groupId).Warning("不能找到群组")
		return
	}

	ts := group.GetMemberTimestamp(client.uid)

	lastId := groupSyncKey.syncKey

	syncGroupHistory := &SyncGroupHistory{
		UID:       client.uid,
		DeviceID:  client.deviceID,
		GroupId:   groupId,
		LastMsgId: lastId,
		Timestamp: int32(ts),
	}

	rpc := GetGroupStorageRPCClient(groupId)
	resp, err := rpc.Call("SyncGroupMessage", syncGroupHistory)
	if err != nil {
		log.WithField("err", err).Warning("同步群组消息失败")
		return
	}

	gh := resp.(*GroupHistoryMessage)
	messages := gh.Messages

	sk := &GroupSyncKey{syncKey: lastId, groupId: groupId}
	client.EnqueueMessage(&Message{cmd: MSG_SYNC_GROUP_BEGIN, body: sk})

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		m := &Message{cmd:int(msg.Cmd), version:DEFAULT_VERSION}
		m.FromData(msg.Raw)
		sk.syncKey = msg.MsgID
		if client.isSender(m, msg.DeviceID) {
			m.flag |= MESSAGE_FLAG_SELF
		}
		client.EnqueueMessage(m)
	}

	if  gh.LastMsgId < lastId && gh.LastMsgId > 0 {
		sk.syncKey = gh.LastMsgId
		log.WithFields(log.Fields{"groupId": groupId, "lastId": lastId, "lastMsgId": gh.LastMsgId}).Warning("群组同步消息的最新id大于服务端消息的最新id")
	}
	client.EnqueueMessage(&Message{cmd:MSG_SYNC_GROUP_END, body:sk})
}

func (client *GroupClient) HandleGroupSyncKey(groupSyncKey *GroupSyncKey) {
	groupId := groupSyncKey.groupId
	lastId := groupSyncKey.syncKey

	if lastId > 0 {
		s := &SyncGroupHistory{
			UID:       client.uid,
			GroupId:   groupId,
			LastMsgId: lastId,
		}
		syncGroupChan <- s
	}
}
