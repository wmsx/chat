package main

import (
	"bytes"
	"encoding/binary"
)

const MSG_PUSH = 134
const MSG_PUBLISH = 132
const MSG_SUBSCRIBE = 130
const MSG_UNSUBSCRIBE = 131
const MSG_PUBLISH_GROUP = 135

func init() {
	messageCreators[MSG_SUBSCRIBE] = func() IMessage { return new(SubscribeMessage) }
	messageCreators[MSG_UNSUBSCRIBE] = func() IMessage { return new(UserID) }

	messageCreators[MSG_PUBLISH] = func() IMessage { return new(AppMessage) }
	messageCreators[MSG_PUBLISH_GROUP] = func() IMessage { return new(AppMessage) }

}

type AppMessage struct {
	receiver  int64
	msgId     int64
	prevMsgId int64
	deviceID  int64
	timestamp int64
	msg       *Message
}

func (amsg *AppMessage) ToData() []byte {
	if amsg.msg == nil {
		return nil
	}
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, amsg.receiver)
	binary.Write(buffer, binary.BigEndian, amsg.msgId)
	binary.Write(buffer, binary.BigEndian, amsg.deviceID)
	binary.Write(buffer, binary.BigEndian, amsg.timestamp)

	mbuffer := new(bytes.Buffer)
	WriteMessage(mbuffer, amsg.msg)
	msgBuf := mbuffer.Bytes()
	l := int16(len(msgBuf))
	binary.Write(buffer, binary.BigEndian, l)
	buffer.Write(msgBuf)

	return buffer.Bytes()
}

func (amsg *AppMessage) FromData(buff []byte) bool {
	if len(buff) < 42 {
		return false
	}

	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &amsg.receiver)
	binary.Read(buffer, binary.BigEndian, &amsg.msgId)
	binary.Read(buffer, binary.BigEndian, &amsg.deviceID)
	binary.Read(buffer, binary.BigEndian, &amsg.timestamp)

	var l int16
	binary.Read(buffer, binary.BigEndian, &l)
	if int(l) > buffer.Len() || l < 0 {
		return false
	}

	msgBuf := make([]byte, l)
	buffer.Read(msgBuf)

	mbuffer := bytes.NewBuffer(msgBuf)
	//recusive
	msg := ReceiveMessage(mbuffer)
	if msg == nil {
		return false
	}
	amsg.msg = msg

	return true
}

type SubscribeMessage struct {
	uid    int64
	online int8
}

func (sub *SubscribeMessage) ToData() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, sub.uid)
	binary.Write(buffer, binary.BigEndian, sub.online)
	return buffer.Bytes()
}

func (sub *SubscribeMessage) FromData(buff []byte) bool {
	if len(buff) < 9 {
		return false
	}
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &sub.uid)
	binary.Read(buffer, binary.BigEndian, &sub.online)
	return true
}
