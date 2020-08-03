package main

import (
	"bytes"
	"encoding/binary"
)

//离线消息由当前登录的用户在当前设备发出 c <- s
const MESSAGE_FLAG_SELF = 0x08

//消息由服务器主动推到客户端 c <- s
const MESSAGE_FLAG_PUSH = 0x10

//超级群消息 c <- s
const MESSAGE_FLAG_SUPER_GROUP = 0x20

const MSG_IM = 4
const MSG_ACK = 5

const MSG_GROUP_IM = 8

const MSG_PING = 13
const MSG_PONG = 14

const MSG_AUTH_STATUS = 3
const MSG_AUTH_TOKEN = 15

//客户端->服务端
const MSG_SYNC = 26 //同步消息
//服务端->客服端
const MSG_SYNC_BEGIN = 27
const MSG_SYNC_END = 28

//通知客户端有新消息
const MSG_SYNC_NOTIFY = 29

//客服端->服务端,更新服务器的synckey
const MSG_SYNC_KEY = 34

//消息的meta信息
const MSG_METADATA = 37

type MessageCreator func() IMessage

var messageCreators map[int]MessageCreator = make(map[int]MessageCreator)

type VersionMessageCreator func() IVersionMessage

var vmessageCreators map[int]VersionMessageCreator = make(map[int]VersionMessageCreator)

func init() {
	messageCreators[MSG_AUTH_TOKEN] = func() IMessage { return new(AuthenticationToken) }
	messageCreators[MSG_AUTH_STATUS] = func() IMessage { return new(AuthenticationStatus) }
	messageCreators[MSG_SYNC] = func() IMessage { return new(SyncKey) }
	messageCreators[MSG_SYNC_BEGIN] = func() IMessage { return new(SyncKey) }
	messageCreators[MSG_SYNC_END] = func() IMessage { return new(SyncKey) }
	messageCreators[MSG_SYNC_NOTIFY] = func() IMessage { return new(SyncKey) }

	vmessageCreators[MSG_IM] = func() IVersionMessage { return new(IMMessage) }
	vmessageCreators[MSG_ACK] = func() IVersionMessage { return new(MessageACK) }
	vmessageCreators[MSG_GROUP_IM] = func() IVersionMessage { return new(IMMessage) }

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

func (sync *Metadata) ToData() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, sync.syncKey)
	binary.Write(buffer, binary.BigEndian, sync.prevSyncKey)
	padding := [16]byte{}
	buffer.Write(padding[:])
	buf := buffer.Bytes()
	return buf
}

func (sync *Metadata) FromData(buff []byte) bool {
	if len(buff) < 32 {
		return false
	}

	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &sync.syncKey)
	binary.Read(buffer, binary.BigEndian, &sync.prevSyncKey)
	return true
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

type AuthenticationToken struct {
	token      string
	platformId int8
	deviceId   string
}

func (auth *AuthenticationToken) ToData() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, auth.platformId)

	l := int8(len(auth.token))
	binary.Write(buffer, binary.BigEndian, l)
	buffer.Write([]byte(auth.token))

	l = int8(len(auth.deviceId))
	binary.Write(buffer, binary.BigEndian, l)
	buffer.Write([]byte(auth.deviceId))

	return buffer.Bytes()
}

func (auth *AuthenticationToken) FromData(buff []byte) bool {
	if len(buff) <= 3 {
		return false
	}

	auth.platformId = int8(buff[0])

	buffer := bytes.NewBuffer(buff[1:])

	var l uint8
	binary.Read(buffer, binary.BigEndian, &l)
	if int(l) > buffer.Len() || int(l) < 0 {
		return false
	}
	token := make([]byte, l)
	buffer.Read(token)

	binary.Read(buffer, binary.BigEndian, &l)
	if int(l) > buffer.Len() || int(l) < 0 {
		return false
	}

	deviceId := make([]byte, l)
	buffer.Read(deviceId)

	auth.token = string(token)
	auth.deviceId = string(deviceId)
	return true
}

type AuthenticationStatus struct {
	status int32
}

func (auth *AuthenticationStatus) ToData() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, auth.status)
	buf := buffer.Bytes()
	return buf
}

func (auth *AuthenticationStatus) FromData(buff []byte) bool {
	if len(buff) < 4 {
		return false
	}
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &auth.status)
	return true
}

type MessageACK struct {
	seq    int32
	status int8
}

func (ack *MessageACK) ToData(version int) []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, ack.seq)
	if version > 1 {
		binary.Write(buffer, binary.BigEndian, ack.status)
	}
	buf := buffer.Bytes()
	return buf
}

func (ack *MessageACK) FromData(version int, buff []byte) bool {
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &ack.seq)
	if version > 1 {
		binary.Read(buffer, binary.BigEndian, &ack.status)
	}
	return true
}

type SyncKey struct {
	syncKey int64
}

func (id *SyncKey) ToData() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, id.syncKey)
	buf := buffer.Bytes()
	return buf
}

func (id *SyncKey) FromData(buff []byte) bool {
	if len(buff) < 8 {
		return false
	}

	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &id.syncKey)
	return true
}

type AppUserID struct {
	appId int64
	uid   int64
}

func (id *AppUserID) ToData() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, id.appId)
	binary.Write(buffer, binary.BigEndian, id.uid)
	return buffer.Bytes()
}

func (id *AppUserID) FromData(buff []byte) bool {
	if len(buff) < 16 {
		return false
	}
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &id.appId)
	binary.Read(buffer, binary.BigEndian, &id.uid)
	return true
}



