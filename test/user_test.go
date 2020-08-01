package test

import (
	"bytes"
	"fmt"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	"net"
	"testing"
	"time"
)

//平台号
const PLATFORM_IOS = 1
const PLATFORM_ANDROID = 2
const PLATFORM_WEB = 3

var redisPool *redis.Pool

func TestSetToken(t *testing.T) {
	initRedisPool("127.0.0.1:6379", "mingchaonaxieshi", 0)
	conn := redisPool.Get()
	conn.Do("HMSET", "access_token_zxcvbnm", "user_id", 1, "app_id", 1)
}

func TestAuth(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:23000")
	if err != nil {
		log.Error(err)
		return
	}
	defer conn.Close()


	auth := &AuthenticationToken{token: "zxcvbnm", platformId: PLATFORM_IOS, deviceId: "deviceId"}

	m := &Message{cmd: MSG_AUTH_TOKEN, seq: 1, version: DEFAULT_VERSION, body: auth}

	buffer := new(bytes.Buffer)
	WriteMessage(buffer, m)
	conn.Write(buffer.Bytes())

	r := ReceiveClientMessage(conn)

	messageChan := make(chan *Message, 1)
	messageChan <- r

	select {
	case msg := <-messageChan:
		if msg == nil {
			fmt.Println("msg is null")
		} else {
			fmt.Println(msg)
			if status, ok := msg.body.(*AuthenticationStatus); ok {
				fmt.Println(status)
			}
		}
	case <-time.After(10 * time.Second):
		fmt.Println("timeout")
	}
}



func initRedisPool(server, password string, db int) {
	redisPool = &redis.Pool{
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
