package main

type PeerMessage struct {
	AppID    int64
	Uid      int64
	DeviceID int64
	Cmd      int32
	Raw      []byte
}

type SyncHistory struct {
	AppID     int64
	UID       int64
	DeviceID  int64
	LastMsgID int64
}

type PeerHistoryMessage struct {
	Messages  []*HistoryMessage
	LastMsgId int64
	HasMore   bool
}

type HistoryMessage struct {
	MsgID    int64
	DeviceID int64
	Cmd      int32
	Raw      []byte
}

func SavePeerMessageInterface(addr string, m *PeerMessage) ([2]int64, error) {
	return [2]int64{}, nil
}

func SyncMessageInterface(addr string, syncKey *SyncHistory) *PeerHistoryMessage {
	return nil
}
