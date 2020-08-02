package main

import (
	"github.com/valyala/gorpc"
	"time"
)
import log "github.com/sirupsen/logrus"

var storage *Storage
var config *StorageConfig

func main() {
	config = readStorageConf()
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

	s := gorpc.Server{
		Addr:    config.rpcListen,
		Handler: dispatcher.NewHandlerFunc(),
	}

	// 阻塞在Serve()
	if err := s.Serve(); err != nil {
		log.Fatalf("Cannot start rpc server: %s", err)
	}
}
