package main

import (
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net"
	"time"
	"unsafe"
)

const CLIENT_TIMEOUT = 6 * 60

type Connection struct {
	conn interface{}

	online bool

	wt chan *Message

	// 离线消息
	pwt chan []*Message

	sequence int // 发送给客户端的消息序号
	version  int //客户端协议版本号

	appId      int64
	uid        int64
	deviceId   string
	deviceID   int64
	platformId int8
}

func (client *Connection) read() *Message {
	if conn, ok := client.conn.(net.Conn); ok {
		conn.SetReadDeadline(time.Now().Add(CLIENT_TIMEOUT * time.Second))
		return ReceiveClientMessage(conn)
	} else if conn, ok := client.conn.(*websocket.Conn); ok {
		conn.SetReadDeadline(time.Now().Add(CLIENT_TIMEOUT * time.Second))
		//return ReadWebsocketMessage(conn)
	}
	return nil
}

func (client *Connection) EnqueueMessage(msg *Message) bool {
	select {
	case client.wt <- msg:
		return true
	case <-time.After(60 * time.Second):
		log.Infof("send message to wt timed out:%d", client.uid)
		return false
	}
}

func (client *Connection) EnqueueMessages(msgs []*Message) bool {
	select {
	case client.pwt <- msgs:
		return true
	case <-time.After(60 * time.Second):
		log.Infof("send messages to pwt timed out:%d", client.uid)
		return false
	}
}

func (client *Connection) SendMessage(uid int64, msg *Message) {
	appId := client.appId
	PublishMessage(appId, uid, msg)
	DispatchMessageToPeer(msg, uid, appId, client.Client())
}



func (client *Connection) Client() *Client {
	p := unsafe.Pointer(client)
	return (*Client)(p)
}

func (client *Connection) close() {
	if conn, ok := client.conn.(net.Conn); ok {
		conn.Close()
	} else if conn, ok := client.conn.(*websocket.Conn); !ok {
		conn.Close()
	}
}
