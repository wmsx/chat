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
	dispatcher.AddFunc("SavePeerMessage", SavePeerMessage)

	s := gorpc.Server{
		Addr:    ":13333",
		Handler: dispatcher.NewHandlerFunc(),
	}

	// 阻塞在Serve()
	if err := s.Serve(); err != nil {
		log.Fatalf("Cannot start rpc server: %s", err)
	}
}
