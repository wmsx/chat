package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log"
	"math/rand"
	"net"
	"time"
)

const redisAddress = "192.168.0.199:6379"
const redisPassword = "mingchaonaxieshi"
const redisDB = 0

var redisPool *redis.Pool
var seededRand *rand.Rand

func NewRedisPool(server, password string, db int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     100,
		MaxActive:   500,
		IdleTimeout: 400 * time.Second,
		Dial: func() (redis.Conn, error) {
			timeout := time.Duration(2) * time.Second
			c, err := redis.Dial("tcp", server, redis.DialConnectTimeout(timeout))
			if err != nil {
				return nil, err
			}
			if len(password) > 0 {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			if db > 0 && db < 16 {
				if _, err := c.Do("SELECT", db); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, nil
		},
	}
}



func sendGroup(sender, groupId int64) {
	token, err := login(sender)
	if err != nil {
		log.Println("login error err: ", err)
		return
	}

	addr := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 23000,}
	conn, err := net.DialTCP("tcp", nil, &addr)
	if err != nil {
		log.Println("connect error")
		return
	}

	seq := 1

	auth := &AuthenticationToken{token: token, platformId: 1, deviceId: "0000000"}
	SendMessage(conn, &Message{cmd: MSG_AUTH_TOKEN, seq: seq, version: DEFAULT_VERSION, body: auth})
	m := ReceiveMessage(conn)
	fmt.Println(m)
	fmt.Println(m.body)

	ticker := time.NewTicker(10 * time.Second)

	for t := range ticker.C {
		content := fmt.Sprintf("test....%d", t.Unix())
		seq++
		msg := &Message{cmd: MSG_GROUP_IM, seq: seq, version: DEFAULT_VERSION, flag: 0, body: &IMMessage{sender, groupId, 0, 0, content}}
		SendMessage(conn, msg)
		for {
			ack := ReceiveMessage(conn)
			fmt.Println(ack)
			fmt.Println(ack.body)
			if ack.cmd == MSG_ACK {
				break
			}
		}
	}

	conn.Close()
	log.Printf("%d send complete", sender)
}


func send(sender, receiver int64) {
	token, err := login(sender)
	if err != nil {
		log.Println("login error err: ", err)
		return
	}

	addr := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 23000,}
	conn, err := net.DialTCP("tcp", nil, &addr)
	if err != nil {
		log.Println("connect error")
		return
	}

	seq := 1

	auth := &AuthenticationToken{token: token, platformId: 1, deviceId: "0000000"}
	SendMessage(conn, &Message{cmd: MSG_AUTH_TOKEN, seq: seq, version: DEFAULT_VERSION, body: auth})
	m := ReceiveMessage(conn)
	fmt.Println(m)
	fmt.Println(m.body)

	ticker := time.NewTicker(1 * time.Second)

	for t := range ticker.C {
		content := fmt.Sprintf("test....%d", t.Unix())
		seq++
		msg := &Message{cmd: MSG_IM, seq: seq, version: DEFAULT_VERSION, flag: 0, body: &IMMessage{sender, receiver, 0, 0, content}}
		SendMessage(conn, msg)
		for {
			ack := ReceiveMessage(conn)
			fmt.Println(ack)
			fmt.Println(ack.body)
			if ack.cmd == MSG_ACK {
				break
			}
		}
	}

	conn.Close()
	log.Printf("%d send complete", sender)
}

func sync(uid int64) {
	syncKey := int64(0)

	token, err := login(uid)

	addr := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 23000,}
	conn, err := net.DialTCP("tcp", nil, &addr)
	if err != nil {
		log.Println("connect error")
		return
	}

	seq := 1
	auth := &AuthenticationToken{token: token, platformId: 1, deviceId: "00000000"}
	SendMessage(conn, &Message{cmd: MSG_AUTH_TOKEN, seq: seq, version: DEFAULT_VERSION, flag: 0, body: auth})
	ReceiveMessage(conn)

	seq++
	ss := &Message{cmd: MSG_SYNC, seq: seq, version: DEFAULT_VERSION, flag: 0, body: &SyncKey{syncKey: syncKey}}
	SendMessage(conn, ss)

	for {
		msg := ReceiveMessage(conn)
		if msg == nil {
			log.Println("sync nil message")
			break
		}
		if msg.cmd == MSG_SYNC_BEGIN {
			m := msg.body.(*SyncKey)
			log.Printf("syncKey:%d", m.syncKey)
		}

		if msg.cmd == MSG_SYNC_END {
			m := msg.body.(*SyncKey)
			log.Printf("syncKey:%d", m.syncKey)

			if m.syncKey > syncKey {
				syncKey = m.syncKey
				seq++
				//sk := &Message{cmd:MSG_SYNC_KEY, seq:seq, version:DEFAULT_VERSION, flag:0, body:&SyncKey{syncKey}}
				//SendMessage(conn, sk)
			}
		}
		if msg.cmd == MSG_IM {
			m := msg.body.(*IMMessage)
			log.Printf("sender:%d receiver:%d content:%s", m.sender, m.receiver, m.content)
		}
	}
	conn.Close()
}

func receive(receiver int64) {
	syncKey := int64(0)

	token, err := login(receiver)

	addr := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 23000,}
	conn, err := net.DialTCP("tcp", nil, &addr)
	if err != nil {
		log.Println("connect error")
		return
	}

	seq := 1
	auth := &AuthenticationToken{token: token, platformId: 1, deviceId: "00000000"}
	SendMessage(conn, &Message{cmd: MSG_AUTH_TOKEN, seq: seq, version: DEFAULT_VERSION, flag: 0, body: auth})

	for {
		msg := ReceiveMessage(conn)
		if msg == nil {
			log.Println("receiver nil message")
			break
		}
		if msg.cmd == MSG_SYNC_BEGIN {
			m := msg.body.(*SyncKey)
			log.Printf("syncKey:%d", m.syncKey)
		}

		if msg.cmd == MSG_SYNC_END {
			m := msg.body.(*SyncKey)
			log.Printf("syncKey:%d", m.syncKey)

			if m.syncKey > syncKey {
				syncKey = m.syncKey
				seq++
			}
		}
		if msg.cmd == MSG_IM || msg.cmd == MSG_GROUP_IM {
			m := msg.body.(*IMMessage)
			log.Printf("sender:%d receiver:%d content:%s", m.sender, m.receiver, m.content)
		}
	}
	conn.Close()
}
