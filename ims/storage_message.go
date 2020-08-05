package main

import (
	"bytes"
	"encoding/binary"
)

//内部文件存储使用
//超级群消息队列
const MSG_GROUP_OFFLINE = 247

//个人消息队列
const MSG_OFFLINE = 248

func init() {
	messageCreators[MSG_GROUP_OFFLINE] = func() IMessage { return new(OfflineMessage) }
}

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

type OfflineMessage struct {
	appId          int64
	receiver       int64 //用户id or 群组id
	msgId          int64 //消息本体的id
	deviceID       int64
	seqId          int64 //消息序号, 1,2,3...
	prevMsgId      int64 //个人消息队列(点对点消息，群组消息)
	prevPeerMsgId  int64 //点对点消息队列
	prevBatchMsgId int64 //0<-1000<-2000<-3000...构成一个消息队列
}

func (off *OfflineMessage) ToData() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, off.appId)
	binary.Write(buffer, binary.BigEndian, off.receiver)
	binary.Write(buffer, binary.BigEndian, off.msgId)
	binary.Write(buffer, binary.BigEndian, off.deviceID)
	binary.Write(buffer, binary.BigEndian, off.seqId)
	binary.Write(buffer, binary.BigEndian, off.prevMsgId)
	binary.Write(buffer, binary.BigEndian, off.prevPeerMsgId)
	binary.Write(buffer, binary.BigEndian, off.prevBatchMsgId)
	return buffer.Bytes()
}

func (off *OfflineMessage) FromData(buff []byte) bool {
	if len(buff) < 64 {
		return false
	}
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &off.appId)
	binary.Read(buffer, binary.BigEndian, &off.receiver)
	binary.Read(buffer, binary.BigEndian, &off.msgId)
	binary.Read(buffer, binary.BigEndian, &off.deviceID)
	binary.Read(buffer, binary.BigEndian, &off.seqId)
	binary.Read(buffer, binary.BigEndian, &off.prevMsgId)
	binary.Read(buffer, binary.BigEndian, &off.prevPeerMsgId)
	binary.Read(buffer, binary.BigEndian, &off.prevBatchMsgId)
	return true
}
