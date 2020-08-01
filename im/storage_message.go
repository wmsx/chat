package main

import (
	"bytes"
	"encoding/binary"
)

//im实例使用
const MSG_PENDING_GROUP_MESSAGE = 251

//待发送的群组消息临时存储结构
type PendingGroupMessage struct {
	appId     int64
	sender    int64
	deviceID  int64 //发送者的设备id
	gid       int64
	timestamp int32

	members []int64 //需要接受此消息的成员列表
	content string
}

func (gm *PendingGroupMessage) ToData() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, gm.appId)
	binary.Write(buffer, binary.BigEndian, gm.sender)
	binary.Write(buffer, binary.BigEndian, gm.deviceID)
	binary.Write(buffer, binary.BigEndian, gm.gid)
	binary.Write(buffer, binary.BigEndian, gm.timestamp)

	count := int16(len(gm.members))
	binary.Write(buffer, binary.BigEndian, count)
	for _, uid := range gm.members {
		binary.Write(buffer, binary.BigEndian, uid)
	}

	buffer.Write([]byte(gm.content))
	buf := buffer.Bytes()
	return buf
}

func (gm *PendingGroupMessage) FromData(buff []byte) bool {
	if len(buff) < 38 {
		return false
	}
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &gm.appId)
	binary.Read(buffer, binary.BigEndian, &gm.sender)
	binary.Read(buffer, binary.BigEndian, &gm.deviceID)
	binary.Read(buffer, binary.BigEndian, &gm.gid)
	binary.Read(buffer, binary.BigEndian, &gm.timestamp)

	var count int16
	binary.Read(buffer, binary.BigEndian, &count)

	if len(buff) < int(38+count*8) {
		return false
	}

	gm.members = make([]int64, count)
	for i := 0; i < int(count); i++ {
		var uid int64
		binary.Read(buffer, binary.BigEndian, &uid)
		gm.members[i] = uid
	}
	offset := 38 + count*8
	gm.content = string(buff[offset:])

	return true
}
