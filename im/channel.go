package main

import (
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

type Channel struct {
	addr string
	wt   chan *Message

	mutex sync.Mutex

	dispatch func(*AppMessage)
}

func NewChannel(addr string, f func(*AppMessage)) *Channel {
	channel := new(Channel)
	channel.addr = addr
	channel.dispatch = f

	channel.wt = make(chan *Message, 10)
	return channel
}

func (channel *Channel) Start() {
	go channel.Run()
}

func (channel *Channel) Run() {
	for {
		conn, err := net.Dial("tcp", channel.addr)
		if err != nil {
			log.Info("connect route server error:", err)
		}
		tconn := conn.(*net.TCPConn)
		tconn.SetKeepAlive(true)
		tconn.SetKeepAlivePeriod(10 * time.Minute)
		log.Info("channel connected")
		channel.RunOnce(tconn)
	}
}

func (channel *Channel) RunOnce(conn *net.TCPConn) {
	defer conn.Close()

	closedCh := make(chan bool)

	go func() {
		for {
			msg := ReceiveMessage(conn)
			if msg == nil {
				close(closedCh)
				return
			}
			log.Info("channel recv message:", Command(msg.cmd))
			if msg.cmd == MSG_PUBLISH {
				amsg := msg.body.(*AppMessage)
				if channel.dispatch != nil {
					channel.dispatch(amsg)
				}
			}
		}
	}()

	for {
		select {
		case _ = <-closedCh:
			log.Info("channel closed")
			return
		case msg := <-channel.wt:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := SendMessage(conn, msg)
			if err != nil {
				log.Info("channel send message err: ", err)
			}
			log.Info("send to route server: ", conn.RemoteAddr())
		}
	}
}

func (channel *Channel) Push(appId int64, receivers []int64, msg *Message) {
	p := &BatchPushMessage{appId: appId, receivers: receivers, msg: msg}
	m := &Message{cmd: MSG_PUSH, body: p}
	channel.wt <- m
}

func (channel *Channel) Publish(amsg *AppMessage) {
	msg := &Message{cmd: MSG_PUBLISH, body: amsg}
	channel.wt <- msg
}
