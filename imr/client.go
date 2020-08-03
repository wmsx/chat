package main

import (
	log "github.com/sirupsen/logrus"
	"net"
)

type Push struct {
	queueName string
	content   []byte
}

type Client struct {
	wt chan *Message

	pwt chan *Push

	conn *net.TCPConn

	appRoute *AppRoute
}

func (client *Client) Run() {
	go client.Read()
	go client.Push()
}

func (client *Client) Read() {

	for {
		msg := client.read()
		if msg == nil {
			close(client.wt)
			break
		}
		client.HandleMessage(msg)
	}
}

func (client *Client) read() *Message {
	return ReceiveMessage(client.conn)
}

func (client *Client) HandleMessage(msg *Message) {
	log.Info("msg cmd:", Command(msg.cmd))

	switch msg.cmd {
	case MSG_PUSH:
		client.HandlePush(msg.body.(*BatchPushMessage))
	default:
		log.Warning("unknown message cmd:", msg.cmd)
	}
}

func (client *Client) HandlePush(pmsg *BatchPushMessage) {
	log.Infof("push message appId:%d cmd:%s", pmsg.appId, Command(pmsg.msg.cmd))

	offMembers := make([]int64, 0)
	for _, uid := range pmsg.receivers {
		if !IsUserOnline(pmsg.appId, uid) {
			offMembers = append(offMembers, uid)
		}
	}

	cmd := pmsg.msg.cmd
	if len(offMembers) > 0 {
		if cmd == MSG_IM {
			client.PublishPeerMessage(pmsg.appId, pmsg.msg.body.(*IMMessage))
		}
	}

}

func (client *Client) IsAppUserOnline(id *AppUserID) bool {
	route := client.appRoute.FindRoute(id.appId)
	if route == nil {
		return false
	}
	return route.IsUserOnline(id.uid)
}



func NewClient(conn *net.TCPConn) *Client {
	client := new(Client)
	client.conn = conn
	client.wt = make(chan *Message, 10)
	return client
}
