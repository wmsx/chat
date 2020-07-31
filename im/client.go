package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"time"
)

//平台号
const PLATFORM_IOS = 1
const PLATFORM_ANDROID = 2
const PLATFORM_WEB = 3

type Client struct {
	Connection
	*PeerClient
}

func NewClient(conn interface{}) *Client {
	client := new(Client)

	client.conn = conn //初始化 connection

	client.wt = make(chan *Message, 300) // write的Message chan，client close的时候需要将wt中的消息发送完

	client.PeerClient = &PeerClient{&client.Connection}
	return client
}

func ListenClient(port int) {
	listenAddr := fmt.Sprintf("0.0.0.0:%d", port)
	listen, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorf("监听端口失败:%s", err)
		return
	}
	tcpListener, ok := listen.(*net.TCPListener)
	if !ok {
		log.Error("监听端口失败")
		return
	}

	for {
		conn, err := tcpListener.AcceptTCP()
		if err != nil {
			log.Errorf("accept err:%s", err)
			return
		}
		log.Infoln("handle new connection, remote address:", conn.RemoteAddr())
		handlerClient(conn)
	}
}

func handlerClient(conn interface{}) {
	client := NewClient(conn)
	client.Run()
}

func (client *Client) Run() {
	go client.Read()
	go client.Write()
}

func (client *Client) Read() {
	for {
		msg := client.read()
		if msg == nil {
			client.HandleClientClosed()
			break
		}
		client.HandleMessage(msg)
	}
}

func (client *Client) Write() {
	running := true
	for running {
		select {
		case msg := <-client.wt:
			if msg == nil {
				client.close()
				running = false
				log.Infof("client:%d socket closed", client.uid)
				break
			}
			client.send(msg)
		}
	}
}

func (client *Client) send(m *Message) {
	client.sequence++
	msg := m

	msg.seq = client.sequence

	if conn, ok := client.conn.(net.Conn); ok {
		conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		err := SendMessage(conn, msg)
		if err != nil {
			log.Info("send msg:", Command(msg.cmd), " tcp err:", err)
		}
	}
}

func (client *Client) HandleClientClosed() {
	close(client.wt)
}

func (client *Client) HandleMessage(msg *Message) {
	log.Info("msg cmd:", Command(msg.cmd))

	switch msg.cmd {
	case MSG_AUTH_TOKEN:
		client.HandleAuthToken(msg.body.(*AuthenticationToken), msg.version)
	}

	client.PeerClient.HandleMessage(msg)
}

func (client *Client) HandleAuthToken(login *AuthenticationToken, version int) {
	var err error
	appId, uid, _, _, err := client.AuthToken(login.token)

	// 鉴权没通过
	if err != nil {
		log.Infof("auth token:%s err:%s", login.token, err)
		msg := &Message{cmd: MSG_AUTH_STATUS, version: version, body: &AuthenticationStatus{status: 1}}
		client.EnqueueMessage(msg)
	}

	if login.platformId != PLATFORM_WEB && len(login.deviceId) > 0 {
		client.deviceID, err = GetDeviceID(login.deviceId, int(login.platformId))
		if err != nil {
			msg := &Message{cmd: MSG_AUTH_STATUS, version: version, body: &AuthenticationStatus{1}}
			client.EnqueueMessage(msg)
			return
		}
	}
	client.appId = appId
	client.uid = uid
	client.deviceId = login.deviceId
	client.platformId = login.platformId

	msg := &Message{cmd: MSG_AUTH_STATUS, version: version, body: &AuthenticationStatus{0}}
	client.EnqueueMessage(msg)
}

func (client *Client) AuthToken(token string) (int64, int64, int, bool, error) {
	appId, uid, err := LoadUserAccessToken(token)
	if err != nil {
		return 0, 0, 0, false, err
	}

	forbidden, notificationOn, err := GetUserPreferences(appId, uid)
	if err != nil {
		return 0, 0, 0, false, err
	}

	return appId, uid, forbidden, notificationOn, nil
}

func GetUserPreferences(appId int64, uid int64) (int, bool, error) {
	return 0, false, nil
}
