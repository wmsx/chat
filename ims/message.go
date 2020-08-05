package main

import (
	"bytes"
	"encoding/binary"
)

const MSG_IM = 4

const MSG_GROUP_IM = 8

//群组消息 c -> s
const MESSAGE_FLAG_GROUP = 0x04

type MessageCreator func() IMessage

var messageCreators map[int]MessageCreator = make(map[int]MessageCreator)

type VersionMessageCreator func() IVersionMessage

var vmessageCreators map[int]VersionMessageCreator = make(map[int]VersionMessageCreator)

func init() {
	messageCreators[MSG_OFFLINE] = func() IMessage { return new(OfflineMessage) }

	vmessageCreators[MSG_IM] = func() IVersionMessage { return new(IMMessage) }
}

type Command int

// Message是消息的统一格式
// 根据cmd 可以将具体的消息，比如IMMessage,SystemMessage 存储在body字段
type Message struct {
	cmd     int
	seq     int
	version int
	flag    int

	body     interface{}
	bodyData []byte

	meta *Metadata
}

func (message *Message) ToData() []byte {
	if message.bodyData != nil {
		return message.bodyData
	} else if message.body != nil {
		if m, ok := message.body.(IMessage); ok {
			return m.ToData()
		}
		if m, ok := message.body.(IVersionMessage); ok {
			return m.ToData(message.version)
		}
		return nil
	} else {
		return nil
	}
}

func (message *Message) FromData(buff []byte) bool {
	cmd := message.cmd
	if creator, ok := messageCreators[cmd]; ok {
		c := creator()
		r := c.FromData(buff)
		message.body = c
		return r
	}
	if creator, ok := vmessageCreators[cmd]; ok {
		c := creator()
		r := c.FromData(message.version, buff)
		message.body = c
		return r
	}
	return len(buff) == 0
}

type Metadata struct {
	syncKey     int64
	prevSyncKey int64
}

// 消息转换的接口 相当于适配器
type IMessage interface {
	ToData() []byte
	FromData([]byte) bool
}

// 带版本消息转换的接口 相当于适配器
type IVersionMessage interface {
	ToData(version int) []byte
	FromData(version int, buff []byte) bool
}

type IMMessage struct {
	sender    int64
	receiver  int64
	timestamp int32
	msgId     int32
	content   string
}

func (m *IMMessage) ToData(version int) []byte {
	if version == 0 {
		return m.ToDataV0()
	} else {
		return m.ToDataV2()
	}
}

func (m *IMMessage) FromData(version int, buff []byte) bool {
	if version == 0 {
		return m.FromDataV0(buff)
	} else {
		return m.FromDataV2(buff)
	}
}

func (m *IMMessage) ToDataV0() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, m.sender)
	binary.Write(buffer, binary.BigEndian, m.receiver)
	binary.Write(buffer, binary.BigEndian, m.msgId)
	buffer.Write([]byte(m.content))
	buf := buffer.Bytes()
	return buf
}

func (m *IMMessage) ToDataV2() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, m.sender)
	binary.Write(buffer, binary.BigEndian, m.receiver)
	binary.Write(buffer, binary.BigEndian, m.timestamp)
	binary.Write(buffer, binary.BigEndian, m.msgId)
	buffer.Write([]byte(m.content))
	buf := buffer.Bytes()
	return buf
}

func (m *IMMessage) FromDataV0(buff []byte) bool {
	if len(buff) < 20 {
		return false
	}
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &m.sender)
	binary.Read(buffer, binary.BigEndian, &m.receiver)
	binary.Read(buffer, binary.BigEndian, &m.msgId)
	m.content = string(buff[20:])
	return true
}

func (m *IMMessage) FromDataV2(buff []byte) bool {
	if len(buff) < 24 {
		return false
	}
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &m.sender)
	binary.Read(buffer, binary.BigEndian, &m.receiver)
	binary.Read(buffer, binary.BigEndian, &m.timestamp)
	binary.Read(buffer, binary.BigEndian, &m.msgId)
	m.content = string(buff[24:])
	return true
}
