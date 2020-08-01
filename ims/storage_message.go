package main

import (
	"bytes"
	"encoding/binary"
)

type EMessage struct {
	msgId    int64
	deviceId int64
	msg      *Message
}

func (emsg *EMessage) ToData() []byte {
	if emsg.msg == nil {
		return nil
	}

	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, emsg.msgId)
	binary.Write(buffer, binary.BigEndian, emsg.deviceId)
	mbuffer := new(bytes.Buffer)
	WriteMessage(mbuffer, emsg.msg)
	msgBuf := mbuffer.Bytes()
	var l = int16(len(msgBuf))
	binary.Write(buffer, binary.BigEndian, l)
	buffer.Write(msgBuf)
	buf := buffer.Bytes()
	return buf
}

func (emsg *EMessage) FromData(buff []byte) bool {
	if len(buff) < 18 {
		return false
	}

	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &emsg.msgId)
	binary.Read(buffer, binary.BigEndian, &emsg.deviceId)
	var l int16
	binary.Read(buffer, binary.BigEndian, &l)
	if int(l) > buffer.Len() {
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
	emsg.msg = msg

	return true
}
