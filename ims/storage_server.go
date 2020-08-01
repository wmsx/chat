package main

import "github.com/valyala/gorpc"
import log "github.com/sirupsen/logrus"

var storage *Storage
var config *StorageConfig

func main() {

	config = readStorageConf()
	storage = NewStorage(config.storageRoot)

	ListenRPCClient()
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
