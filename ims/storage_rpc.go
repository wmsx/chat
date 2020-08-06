package main

// storage rpc的接口定义文件

// 点对点消息
type PeerMessage struct {
	AppID    int64
	UID      int64
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

type PeerGroupMessage struct {
	AppId    int64
	Members  []int64
	DeviceID int64
	Cmd      int32
	Raw      []byte
}

type GroupMessage struct {
	AppId    int64
	GroupId  int64
	DeviceID int64
	Cmd      int32
	Raw      []byte
}

type SyncGroupHistory struct {
	AppId     int64
	UID       int64
	DeviceID  int64
	GroupId   int64
	LastMsgId int64
	Timestamp int32
}

type GroupHistoryMessage PeerHistoryMessage

func SavePeerMessageInterface(addr string, m *PeerMessage) ([2]int64, error) {
	return [2]int64{}, nil
}

func SyncMessageInterface(addr string, syncKey *SyncHistory) *PeerHistoryMessage {
	return nil
}

func SavePeerGroupMessageInterface(addr string, m *PeerGroupMessage) ([]int64, error) {
	return nil, nil
}

func SaveGroupMessageInterface(addr string, m *GroupMessage) ([2]int64, error) {
	return [2]int64{}, nil
}


func SyncGroupMessageInterface(addr string, syncKey *SyncGroupHistory) *GroupHistoryMessage {
	return nil
}