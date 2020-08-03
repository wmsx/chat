package main

import (
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

type Subscriber struct {
	uids map[int64]int
}

func NewSubscriber() *Subscriber {
	s := new(Subscriber)
	s.uids = make(map[int64]int)
	return s
}

type Channel struct {
	addr string
	wt   chan *Message

	mutex       sync.Mutex
	subscribers map[int64]*Subscriber // key 为appId

	dispatch func(*AppMessage)
}

func NewChannel(addr string, f func(*AppMessage)) *Channel {
	channel := new(Channel)
	channel.addr = addr
	channel.dispatch = f
	channel.subscribers = make(map[int64]*Subscriber)

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

//online表示用户不再接受推送通知(apns, gcm)
func (channel *Channel) Subscribe(appId, uid int64, online bool) {
	count, onlineCount := channel.AddSubscribe(appId, uid, online)
	log.Info("subscribe count:", count, onlineCount)

	if count == 0 {
		//新用户上线
		on := 0
		if online {
			on = 1
		}
		id := SubscribeMessage{appId: appId, uid: uid, online: int8(on)}
		msg := &Message{cmd: MSG_SUBSCRIBE, body: id}
		channel.wt <- msg
	} else if onlineCount == 0 && online {
		// 手机端上线
		id := &SubscribeMessage{appId: appId, uid: uid, online: 1}
		msg := &Message{cmd: MSG_SUBSCRIBE, body: id}
		channel.wt <- msg
	}
}

func (channel *Channel) Unsubscribe(appId int64, uid int64, online bool) {
	count, onlineCount := channel.RemoveSubscribe(appId, uid, online)
	log.Info("unsub count:", count, onlineCount)
	if count == 1 {
		// 用户断开全部连接
		id := &AppUserID{appId: appId, uid: uid}
		msg := &Message{cmd: MSG_UNSUBSCRIBE, body: id}
		channel.wt <- msg
	} else if count > 1 && onlineCount == 1 && online {
		//手机端断开连接,pc/web端还未断开连接
		id := &SubscribeMessage{appId: appId, uid: uid, online: 0}
		msg := &Message{cmd: MSG_SUBSCRIBE, body: id}
		channel.wt <- msg
	}
}

func (channel *Channel) AddSubscribe(appId, uid int64, online bool) (int, int) {
	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	subscriber, ok := channel.subscribers[appId]
	if !ok {
		subscriber = NewSubscriber()
		channel.subscribers[appId] = subscriber
	}

	//不存在时count==0
	count := subscriber.uids[uid]

	//低16位表示总数量 高16位表示online的数量
	c1 := count & 0xffff
	c2 := count >> 16 & 0xffff

	if online {
		c2 += 1
	}

	c1 += 1
	subscriber.uids[uid] = c2<<16 | c1
	return count & 0xffff, count >> 16 & 0xffff
}

func (channel *Channel) RemoveSubscribe(appId int64, uid int64, online bool) (int, int) {
	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	subscriber, ok := channel.subscribers[appId]
	if !ok {
		return 0, 0
	}

	count, ok := subscriber.uids[uid]
	//低16位表示总数量 高16位表示online的数量
	c1 := count & 0xffff
	c2 := count >> 16 & 0xffff

	if ok {
		if online {
			c2 -= 1
			if c2 <= 0 {
				log.Warning("online count < 0")
			}
		}
		c1 -= 1
		if c1 > 0 {
			subscriber.uids[uid] = c2<<16 | c1
		} else {
			delete(subscriber.uids, uid)
		}
	}
	return count & 0xffff, count >> 16 & 0xffff
}
