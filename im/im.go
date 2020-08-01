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
		Uid:      uid,
		DeviceID: deviceID,
		Cmd:      int32(m.cmd),
		Raw:      m.ToData(),
	}

	resp, err := dc.Call("SavePeerMessage", pm)
	if err != nil {
		log.Error("save peer message err:", err)
		return 0, 0, err
	}

	r := resp.([2]int64)
	msgId := r[0]
	prevMsgId := r[1]
	log.Infof("save peer message:%d %d %d %d\n", appId, uid, deviceID, msgId)
	return msgId, prevMsgId, nil
}

func SavePeerGroupMessage(appId int64, members []int64, deviceID int64, m *Message) ([]int64, error) {
	if len(members) == 0 {
		return nil, nil
	}

	dc := GetStorageRPCClient(members[0])

	pm := &PeerGroupMessage{
		AppId:    appId,
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
	log.Infof("save peer group message:%d %v %d %v\n", appId, members, deviceID, r)
	return r, nil
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
	index := deliverIndex%uint64(len(groupMessageDelivers))
	return groupMessageDelivers[index]
}
