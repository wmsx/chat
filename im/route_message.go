package main

import (
	"bytes"
	"encoding/binary"
)

const MSG_PUSH = 134
const MSG_PUBLISH = 132

func init() {
	messageCreators[MSG_PUSH] = func() IMessage { return new(BatchPushMessage) }
}

type BatchPushMessage struct {
	appId     int64
	receivers []int64
	msg       *Message
}

func (amsg *BatchPushMessage) ToData() []byte {
	if amsg.msg == nil {
		return nil
	}

	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, amsg.appId)

	count := uint16(len(amsg.receivers))
	binary.Write(buffer, binary.BigEndian, count)

	for _, receiver := range amsg.receivers {
		binary.Write(buffer, binary.BigEndian, receiver)
	}

	mbuffer := new(bytes.Buffer)
	WriteMessage(mbuffer, amsg.msg)
	msgBuf := mbuffer.Bytes()

	l := int16(len(msgBuf))
	binary.Write(buffer, binary.BigEndian, l)
	buffer.Write(msgBuf)

	return buffer.Bytes()
}

func (amsg *BatchPushMessage) FromData(buff []byte) bool {
	if len(buff) < 12 {
		return false
	}

	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &amsg.appId)

	var count uint16
	binary.Read(buffer, binary.BigEndian, &count)

	if len(buff) < 8+2+int(count)*8+2 {
		return false
	}
	receivers := make([]int64, 0, count)
	for i := 0; i < int(count); i++ {
		var receiver int64
		binary.Read(buffer, binary.BigEndian, &receiver)
		receivers = append(receivers, receiver)
	}

	amsg.receivers = receivers

	var l int16
	binary.Read(buffer, binary.BigEndian, &l)

	if int(l) > buffer.Len() || l < 0 {
		return false
	}

	msgBuf := make([]byte, l)
	buffer.Read(msgBuf)

	mbuffer := bytes.NewBuffer(msgBuf)

	msg := ReceiveMessage(mbuffer)
	if msg == nil {
		return false
	}
	amsg.msg = msg
	return true
}

type AppMessage struct {
	appId     int64
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
	binary.Write(buffer, binary.BigEndian, amsg.appId)
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
	binary.Read(buffer, binary.BigEndian, &amsg.appId)
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
