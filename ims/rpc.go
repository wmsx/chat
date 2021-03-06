package main

func SavePeerMessage(addr string, m *PeerMessage) ([2]int64, error) {
	msg := &Message{cmd: int(m.Cmd), version: DEFAULT_VERSION}
	msg.FromData(m.Raw)
	msgId, prevMsgId := storage.SavePeerMessage(m.UID, m.DeviceID, msg)
	return [2]int64{msgId, prevMsgId}, nil
}

func SyncMessage(addr string, syncKey *SyncHistory) *PeerHistoryMessage {
	messages, lastMsgId, hasMore := storage.LoadHistoryMessages( syncKey.UID, syncKey.LastMsgID, config.Limit, config.hardLimit)

	historyMessages := make([]*HistoryMessage, 0, 10)

	for _, emsg := range messages {
		hm := &HistoryMessage{
			MsgID:    emsg.msgId,
			DeviceID: emsg.deviceId,
			Cmd:      int32(emsg.msg.cmd),
		}
		emsg.msg.version = DEFAULT_VERSION
		hm.Raw = emsg.msg.ToData()
		historyMessages = append(historyMessages, hm)
	}

	return &PeerHistoryMessage{Messages: historyMessages, LastMsgId: lastMsgId, HasMore: hasMore}
}

func SavePeerGroupMessage(addr string, m *PeerGroupMessage) ([]int64, error) {
	msg := &Message{cmd: int(m.Cmd), version: DEFAULT_VERSION}
	msg.FromData(m.Raw)
	r := storage.SavePeerGroupMessage(m.Members, m.DeviceID, msg)
	return r, nil
}

func SaveGroupMessage(addr string, m *GroupMessage) ([2]int64, error) {
	msg := &Message{cmd: int(m.Cmd), version: DEFAULT_VERSION}
	msg.FromData(m.Raw)

	msgId, prevMsgId := storage.SaveGroupMessage(m.GroupId, m.DeviceID, msg)
	return [2]int64{msgId, prevMsgId}, nil
}

func SyncGroupMessage(addr string, syncKey *SyncGroupHistory) *GroupHistoryMessage {
	messages, lastMsgId := storage.LoadGroupHistoryMessage(syncKey.UID, syncKey.GroupId, syncKey.LastMsgId, syncKey.Timestamp, GROUP_OFFLINE_LIMIT)

	historyMessages := make([]*HistoryMessage, 0, 10)
	for _, emsg := range messages {
		hm := &HistoryMessage{}
		hm.MsgID = emsg.msgId
		hm.DeviceID = emsg.deviceId
		hm.Cmd = int32(emsg.msg.cmd)

		emsg.msg.version = DEFAULT_VERSION
		hm.Raw = emsg.msg.ToData()
		historyMessages = append(historyMessages, hm)
	}
	return &GroupHistoryMessage{Messages:historyMessages, LastMsgId:lastMsgId, HasMore:false}
}
