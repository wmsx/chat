package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/gorpc"
	"gopkg.in/natefinch/lumberjack.v2"
	"path"
	"time"
)
import "github.com/gomodule/redigo/redis"

var config *Config
var redisPool *redis.Pool

var rpcClients []*gorpc.DispatcherClient
var groupRpcClients []*gorpc.DispatcherClient

var syncChan chan *SyncHistory
var syncGroupChan chan *SyncGroupHistory

//round-robin
var currentDeliverIndex uint64
var groupMessageDelivers []*GroupMessageDeliver

var groupManager *GroupManager

//route server
var routeChannels []*Channel
var groupRouteChannels []*Channel
var appRoute *AppRoute

func init() {
	appRoute = NewAppRoute()
	syncChan = make(chan *SyncHistory, 100)
	syncGroupChan = make(chan *SyncGroupHistory, 100)
}

func main() {
	config = readConfig()

	initLog()

	redisPool = NewRedisPool(config.redisAddress, config.redisPassword, config.redisDB)

	rpcClients = make([]*gorpc.DispatcherClient, 0)

	if len(config.storageRpcAddrs) > 0 {
		for _, addr := range config.storageRpcAddrs {
			c := &gorpc.Client{Conns: 4, Addr: addr}
			c.Start()

			dispatcher := gorpc.NewDispatcher()
			dispatcher.AddFunc("SyncMessage", SyncMessageInterface)
			dispatcher.AddFunc("SavePeerMessage", SavePeerMessageInterface)
			dispatcher.AddFunc("SavePeerGroupMessage", SavePeerGroupMessageInterface)

			dc := dispatcher.NewFuncClient(c)
			rpcClients = append(rpcClients, dc)
		}
	}

	if len(config.groupStorageRpcAddrs) > 0 {
		groupRpcClients = make([]*gorpc.DispatcherClient, 0)
		for _, addr := range config.groupStorageRpcAddrs {
			c := &gorpc.Client{
				Addr:  addr,
				Conns: 4,
			}
			c.Start()

			dispatcher := gorpc.NewDispatcher()
			dispatcher.AddFunc("SaveGroupMessage", SaveGroupMessageInterface)

			dc := dispatcher.NewFuncClient(c)
			groupRpcClients = append(groupRpcClients, dc)
		}
	}

	if len(config.routeAddrs) > 0 {
		routeChannels = make([]*Channel, 0)
		for _, addr := range config.routeAddrs {
			channel := NewChannel(addr, DispatchAppMessage, DispatchGroupMessage)
			channel.Start()
			routeChannels = append(routeChannels, channel)
		}
	} else {
		log.Fatal("route服务器配置为空")
	}

	if len(config.groupRouteAddrs) > 0 {
		groupRouteChannels = make([]*Channel, 0)
		for _, addr := range config.groupRouteAddrs {
			channel := NewChannel(addr, DispatchAppMessage, DispatchGroupMessage)
			channel.Start()
			groupRouteChannels = append(groupRouteChannels, channel)
		}
	} else {
		log.Fatal("群组route服务器配置为空")
	}

	if len(config.mysqlDatasource) > 0 {
		groupManager = NewGroupManager()
		groupManager.Start()
	}

	groupMessageDelivers = make([]*GroupMessageDeliver, config.groupDeliverCount)
	for i := 0; i < config.groupDeliverCount; i++ {
		q := fmt.Sprintf("q%d", i)
		r := path.Join(config.pendingRoot, q)
		deliver := NewGroupMessageDeliver(r)
		deliver.Start()
		groupMessageDelivers[i] = deliver
	}
	go SyncKeyService()

	log.WithField("测试", "xx").Info("测试....")

	ListenClient(config.port)
	log.Info("exit")
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
				log.WithFields(log.Fields{"appId": s.AppID, "uid": s.UID, "lastMsgId": s.LastMsgID}).Infof("save sync key")
				SaveSyncKey(s.AppID, s.UID, s.LastMsgID)
			}
		case s := <-syncGroupChan:
			origin := GetGroupSyncKey(s.AppId, s.UID, s.GroupId)
			if s.LastMsgId > origin {
				SaveGroupSyncKey(s.AppId, s.UID, s.GroupId, s.LastMsgId)
			}
		}
	}
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
