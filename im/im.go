package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/valyala/gorpc"
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

func GetStorageRPCClient(uid int64) *gorpc.DispatcherClient {
	index := uid % int64(len(rpcClients))
	return rpcClients[index]
}
