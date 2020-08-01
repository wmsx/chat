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
	if group.super {
		msgId, prevMsgId, err := client.HandleSuperGroupMessage(msg, group)
		if err == nil {
			meta = &Metadata{syncKey: msgId, prevSyncKey: prevMsgId}
		}
	} else {
		msgId, prevMsgId, err := client.HandleGroupMessage(msg, group)
		if err == nil {
			meta = &Metadata{syncKey: msgId, prevSyncKey: prevMsgId}
		}
	}

	ack := &Message{cmd: MSG_ACK, body: &MessageACK{seq: int32(seq)}, meta: meta}
	r := client.EnqueueMessage(ack)
	if !r {
		log.Warning("send group message ack error")
	}
	log.Infof("group message sender:%d group id:%d super:%v", msg.sender, msg.receiver, group.super)
	if meta != nil {
		log.Info("group message ack meta:", meta.syncKey, meta.prevSyncKey)
	}
}

func (client *GroupClient) HandleGroupMessage(im *IMMessage, group *Group) (int64, int64, error) {
	gm := PendingGroupMessage{}
	gm.appId = client.appId
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
		log.Errorf("save group message:%d %d timeout", im.sender, im.receiver)
		return 0, 0, errors.New("timeout")
	}
}

func (client *GroupClient) HandleSuperGroupMessage(msg *IMMessage, group *Group) (int64, int64, error) {

	return 0, 0, nil
}
