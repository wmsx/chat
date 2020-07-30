package main

func SavePeerMessage(addr string, m *PeerMessage) ([2]int64, error) {
	msg := &Message{cmd: int(m.Cmd), version: DEFAULT_VERSION}
	msg.FromData(m.Raw)
	msgId, prevMsgId := storage.SavePeerMessage(m.AppID, m.UID, m.DeviceID, msg)
	return [2]int64{msgId, prevMsgId}, nil
}
