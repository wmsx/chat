package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/valyala/gorpc"
	"sync/atomic"
)

func SaveMessage(appId, uid, deviceID int64, m *Message) (int64, int64, error) {
	dc := GetStorageRPCClient(uid)

	pm := &PeerMessage{
		AppID:    appId,
		UID:      uid,
		DeviceID: deviceID,
		Cmd:      int32(m.cmd),
		Raw:      m.ToData(),
	}

	resp, err := dc.Call("SavePeerMessage", pm)
	if err != nil {
		log.WithField("err", err).Error("save peer message err:")
		return 0, 0, err
	}

	r := resp.([2]int64)
	msgId := r[0]
	prevMsgId := r[1]
	log.Infof("save peer message:%d %d %d %d\n", appId, uid, deviceID, msgId)
	return msgId, prevMsgId, nil
}

func SavePeerGroupMessage(members []int64, deviceID int64, m *Message) ([]int64, error) {
	if len(members) == 0 {
		return nil, nil
	}

	// members 都是同一台ims上的
	dc := GetStorageRPCClient(members[0])

	pm := &PeerGroupMessage{
		Members:  members,
		DeviceID: deviceID,
		Cmd:      int32(m.cmd),
		Raw:      m.ToData(),
	}

	resp, err := dc.Call("SavePeerGroupMessage", pm)
	if err != nil {
		log.Error("save peer group message err:", err)
		return nil, err
	}

	r := resp.([]int64)
	log.Infof("save peer group message:%v %d %v\n", members, deviceID, r)
	return r, nil
}

func SaveGroupMessage(appId, gid int64, deviceID int64, msg *Message) (int64, int64, error) {
	dc := GetGroupStorageRPCClient(gid)

	gm := &GroupMessage{
		AppId:    appId,
		GroupId:  gid,
		DeviceID: deviceID,
		Cmd:      int32(msg.cmd),
		Raw:      msg.ToData(),
	}

	resp, err := dc.Call("SaveGroupMessage", gm)
	if err != nil {
		log.WithField("err", err).Warning("保存群组消息失败")
		return 0, 0, nil
	}
	r := resp.([2]int64)
	msgId := r[0]
	prevMsgId := r[1]
	log.WithFields(log.Fields{"appId": appId, "gid": gid, "msgId": msgId}).Info("保存群组消息成功")
	return msgId, prevMsgId, nil
}

func GetGroupStorageRPCClient(gid int64) *gorpc.DispatcherClient {
	index := gid % int64(len(groupRpcClients))
	return groupRpcClients[index]
}

func PublishMessage(uid int64, msg *Message) {
	amsg := &AppMessage{receiver: uid, msg: msg}
	if msg.meta != nil {
		amsg.msgId = msg.meta.syncKey
		amsg.prevMsgId = msg.meta.prevSyncKey
	}
	channel := GetChannel(uid)
	channel.Publish(amsg)
}

func PublishGroupMessage(appId, gid int64, msg *Message) {
	amsg := &AppMessage{appId: appId, receiver: gid, msg: msg}
	if msg.meta != nil {
		amsg.msgId = msg.meta.syncKey
		amsg.prevMsgId = msg.meta.prevSyncKey
	}
	channel := GetGroupChannel(gid)
	channel.PublishGroup(amsg)
}

func GetGroupChannel(gid int64) *Channel {
	index := gid % int64(len(groupRouteChannels))
	return groupRouteChannels[index]
}

func DispatchMessageToPeer(msg *Message, uid int64, client *Client) bool {
	clients := route.FindClientSet(uid)
	if len(clients) == 0 {
		return false
	}
	for c, _ := range clients {
		if c == client { // 有可能是自己发给自己的其他登录点，此时可能c==client
			continue
		}
		c.EnqueueMessage(msg)
	}
	return true
}

func PushMessage(appId, receiver int64, m *Message) {
	channel := GetChannel(receiver)
	channel.Push(appId, []int64{receiver}, m)
}

// 根据用户id到一台route server上，
func GetChannel(receiver int64) *Channel {
	index := receiver % int64(len(routeChannels))
	return routeChannels[index]
}

func GetStorageRPCClient(uid int64) *gorpc.DispatcherClient {
	index := uid % int64(len(rpcClients))
	return rpcClients[index]
}

func GetStorageRPCIndex(uid int64) int64 {
	index := uid % int64(len(rpcClients))
	return index
}

func GetGroupMessageDeliver(groupId int64) *GroupMessageDeliver {
	deliverIndex := atomic.AddUint64(&currentDeliverIndex, 1)
	index := deliverIndex % uint64(len(groupMessageDelivers))
	return groupMessageDelivers[index]
}

func DispatchAppMessage(amsg *AppMessage) {
	if amsg.msgId > 0 {
		if amsg.msg.flag&MESSAGE_FLAG_PUSH == 0 {
			log.Fatal("invalid message flag", amsg.msg.flag)
		}
		meta := &Metadata{syncKey: amsg.msgId, prevSyncKey: amsg.prevMsgId}
		amsg.msg.meta = meta
	}
	DispatchMessageToPeer(amsg.msg, amsg.receiver, nil)
}

func DispatchGroupMessage(amsg *AppMessage) {
	if amsg.msgId > 0 {
		if amsg.msg.flag&MESSAGE_FLAG_PUSH == 0 {
			log.Fatal("invalid message flag", amsg.msg.flag)
		}
		if (amsg.msg.flag & MESSAGE_FLAG_SUPER_GROUP) == 0 {
			log.Fatal("invalid message flag", amsg.msg.flag)
		}

		meta := &Metadata{syncKey: amsg.msgId, prevSyncKey: amsg.prevMsgId}
		amsg.msg.meta = meta
	}

	deliver := GetGroupMessageDeliver(amsg.receiver)
	deliver.DispatchMessage(amsg)
}

func DispatchMessageToGroup(msg *Message, group *Group, appId int64, client *Client) bool {
	if group == nil {
		return false
	}

	members := group.Members()
	for member := range members {
		clients := route.FindClientSet(member)
		if len(clients) == 0 {
			continue
		}
		for c, _ := range clients {
			if c == client {
				continue
			}
			c.EnqueueMessage(msg)
		}
	}
	return true
}
