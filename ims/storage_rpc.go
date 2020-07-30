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

func SavePeerMessageInterface(addr string, m *PeerMessage) ([2]int64, error) {
	return [2]int64{}, nil
}
