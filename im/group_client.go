package main

import (
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
		msgId, prevMsgId, err := client.HandleSuperGroupMessgae(msg, group)
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
