package main

import (
	"github.com/valyala/gorpc"
	"gopkg.in/natefinch/lumberjack.v2"
	"time"
)
import log "github.com/sirupsen/logrus"

var storage *Storage
var config *StorageConfig

func main() {
	config = readStorageConf()
	initLog()

	storage = NewStorage(config.storageRoot)


	go FlushIndexLoop()

	ListenRPCClient()
}

// 将MessageIndex持久化到磁盘
func FlushIndexLoop()  {
	ticker := time.NewTicker(time.Minute * 5)
	for range ticker.C {
		storage.FlushIndex()
	}
}

func ListenRPCClient() {
	dispatcher := gorpc.NewDispatcher()
	dispatcher.AddFunc("SyncMessage", SyncMessage)
	dispatcher.AddFunc("SavePeerMessage", SavePeerMessage)
	dispatcher.AddFunc("SavePeerGroupMessage", SavePeerGroupMessage)
	dispatcher.AddFunc("SaveGroupMessage", SaveGroupMessage)

	s := gorpc.Server{
		Addr:    config.rpcListen,
		Handler: dispatcher.NewHandlerFunc(),
	}

	// 阻塞在Serve()
	if err := s.Serve(); err != nil {
		log.Fatalf("Cannot start rpc server: %s", err)
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
		log.SetFormatter(&log.TextFormatter{DisableColors:true})
		log.StandardLogger().SetNoLock()
	}
	log.SetReportCaller(config.logCaller)
}
