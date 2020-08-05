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

func NewClient(conn *net.TCPConn) *Client {
	client := new(Client)
	client.conn = conn
	client.wt = make(chan *Message, 10)
	client.appRoute = NewAppRoute()
	return client
}

func (client *Client) Run() {
	go client.Write()
	go client.Read()
	go client.Push()
}

func (client *Client) Read() {
	AddClient(client)
	for {
		msg := client.read()
		if msg == nil { // im下线 msg就会为空，移除client
			RemoveClient(client)
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
	case MSG_SUBSCRIBE:
		client.HandleSubscribe(msg.body.(*SubscribeMessage))
	case MSG_UNSUBSCRIBE:
		client.HandleUnsubscribe(msg.body.(*AppUserID))
	case MSG_PUBLISH:
		client.HandlePublish(msg.body.(*AppMessage))
	case MSG_PUBLISH_GROUP:
		client.HandlePublishGroup(msg.body.(AppMessage))
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

func (client *Client) HandlePublish(amsg *AppMessage) {
	log.Infof("publish message appid:%d uid:%d msgid:%d cmd:%d", amsg.appId, amsg.receiver, amsg.msgId, Command(amsg.msg.cmd))

	receiver := &AppUserID{appId: amsg.appId, uid: amsg.receiver}
	s := FindClientSet(receiver)

	msg := &Message{cmd: MSG_PUBLISH, body: amsg}
	for c := range s {
		if client == c { // 如果从im过来的，消息已经从im发送出去了，这里是发送给其他im机器上的
			continue
		}
		c.wt <- msg
	}
}

func (client *Client) ContainAppUserID(id *AppUserID) bool {
	route := client.appRoute.FindRoute(id.appId)
	if route == nil {
		return false
	}
	return route.ContainUserID(id.uid)
}

func (client *Client) Write() {
	seq := 0
	for {
		msg := <-client.wt
		if msg == nil {
			client.close()
			log.Infof("client socket closed")
			break
		}
		seq++
		msg.seq = seq
		client.send(msg)
	}
}

func (client *Client) close() {
	client.conn.Close()
}

func (client *Client) send(msg *Message) {
	SendMessage(client.conn, msg)
}

func (client *Client) HandleSubscribe(id *SubscribeMessage) {
	log.Infof("subscribe appid:%d uid:%d online:%d", id.appId, id.uid, id.online)
	route := client.appRoute.FindOrAddRoute(id.appId)
	on := id.online != 0
	route.AddUserID(id.uid, on)
}

func (client *Client) HandleUnsubscribe(id *AppUserID) {
	log.Infof("unsubscribe appid:%d uid:%d", id.appId, id.uid)

	route := client.appRoute.FindOrAddRoute(id.appId)
	route.RemoveUserID(id.uid)
}

func (client *Client) HandlePublishGroup(amsg AppMessage) {
	log.WithFields(log.Fields{"appId": amsg.appId, "msgId": amsg.msgId, "receiver": amsg.receiver, "cmd": amsg.msg.cmd}).Info("分发群组消息")
	// 群发给所有接入服务器
	s := GetClientSet()

	msg := &Message{cmd: MSG_PUBLISH_GROUP, body: amsg}
	for c := range s {
		//不发送给自身
		if client == c {
			continue
		}
		c.wt <- msg
	}
}
