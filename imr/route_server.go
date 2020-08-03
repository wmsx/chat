package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"net"
	"runtime"
	"time"
)

var config *RouteConfig
var redisPool *redis.Pool
var clients ClientSet

func init()  {
	clients = NewClientSet()
}

func main() {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	config = readConfig()
	redisPool = NewRedisPool(config.redisAddr, config.redisPassword, config.redisDB)

	ListenClient()
}

func ListenClient() {
	Listen(handleClient, config.listen)
}

func Listen(f func(conn *net.TCPConn), listenAddr string) {
	listen, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Println("初始化失败", err.Error())
		return
	}

	tcpListener, ok := listen.(*net.TCPListener)
	if !ok {
		fmt.Println("listen error")
		return
	}

	for {
		client, err := tcpListener.AcceptTCP()
		if err != nil {
			return
		}
		f(client)
	}
}

func handleClient(conn *net.TCPConn) {
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(10 * time.Minute)
	client := NewClient(conn)
	log.Info("new client:", conn.RemoteAddr())
	client.Run()
}

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

func IsUserOnline(appId, uid int64) bool {
	id := &AppUserID{appId: appId, uid: uid}
	for c := range clients {
		if c.IsAppUserOnline(id) {
			return true
		}
	}
	return false
}
