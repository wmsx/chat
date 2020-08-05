package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"time"
)

var config *RouteConfig
var redisPool *redis.Pool
var clients ClientSet
var mutex sync.Mutex

func init() {
	clients = NewClientSet()
}

func main() {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	config = readConfig()
	initLog()

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
	log.WithField("客户端地址", conn.RemoteAddr()).Info("新客户端连接:")
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

func FindClientSet(id *AppUserID) ClientSet {
	mutex.Lock()
	defer mutex.Unlock()

	s := NewClientSet()

	for c := range clients {
		if c.ContainAppUserID(id) {
			s.Add(c)
		}
	}
	return s
}

func AddClient(client *Client) {
	mutex.Lock()
	defer mutex.Unlock()

	clients.Add(client)
}

func RemoveClient(client *Client) {
	mutex.Lock()
	defer mutex.Unlock()

	clients.Remove(client)
}

func GetClientSet() ClientSet {
	mutex.Lock()
	defer mutex.Unlock()

	s := NewClientSet()

	for c := range clients {
		s.Add(c)
	}
	return s
}

func initLog() {
	if config.logFilename != "" {
		writer := &lumberjack.Logger{
			Filename:   config.logFilename,
			MaxSize:    1024,
			MaxAge:     config.logAge,
			MaxBackups: config.logBackup,
			Compress:   false,
		}
		log.SetOutput(writer)
		log.SetFormatter(&log.TextFormatter{DisableColors: true})
		log.StandardLogger().SetNoLock()
	}
	log.SetReportCaller(config.logCaller)
}
