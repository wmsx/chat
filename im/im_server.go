package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/valyala/gorpc"
	"time"
)
import "github.com/gomodule/redigo/redis"

var config *Config
var redisPool *redis.Pool
var rpcClients []*gorpc.DispatcherClient
var syncChan chan *SyncHistory

func init() {
	syncChan = make(chan *SyncHistory, 100)
}

func main() {
	config = readConfig()

	redisPool = NewRedisPool(config.redisAddress, config.redisPassword, config.redisDB)

	rpcClients = make([]*gorpc.DispatcherClient, 0)

	if len(config.storageRpcAddrs) > 0 {
		for _, addr := range config.storageRpcAddrs {
			c := &gorpc.Client{Conns: 4, Addr: addr}
			c.Start()

			dispatcher := gorpc.NewDispatcher()
			dispatcher.AddFunc("SyncMessage", SyncMessageInterface)
			dispatcher.AddFunc("SavePeerMessage", SavePeerMessageInterface)

			dc := dispatcher.NewFuncClient(c)
			rpcClients = append(rpcClients, dc)
		}
	}

	go SyncKeyService()

	ListenClient(config.port)
	log.Infof("exit")
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

func SyncKeyService() {
	for {
		select {
		case s := <-syncChan:
			origin := GetSyncKey(s.AppID, s.UID)
			if s.LastMsgID > origin {
				log.Infof("save sync key:%d %d %d", s.AppID, s.UID, s.LastMsgID)
				SaveSyncKey(s.AppID, s.UID, s.LastMsgID)
			}
		}
	}
}
