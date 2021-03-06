package main

import log "github.com/sirupsen/logrus"

type Storage struct {
	*StorageFile
	*PeerStorage
	*GroupStorage
}

func NewStorage(root string) *Storage {
	file := NewStorageFile(root)
	ps := NewPeerStorage(file)
	gs := NewGroupStorage(file)

	storage := &Storage{
		file,
		ps,
		gs,
	}

	r1 := storage.readPeerIndex()
	storage.lastSavedId = storage.lastId

	if r1 {
		storage.repairPeerIndex()
	}

	log.Infof("last id:%d last saved id:%d", storage.lastId, storage.lastSavedId)
	storage.FlushIndex()
	return storage
}

func (storage *Storage) FlushIndex() {
	storage.flushIndex()
}

func (storage *Storage) flushIndex() {
	storage.mutex.Lock()
	lastId := storage.lastId
	peerIndex := storage.clonePeerIndex()
	groupIndex := storage.cloneGroupIndex()
	storage.mutex.Unlock()

	storage.savePeerIndex(peerIndex)
	storage.saveGroupIndex(groupIndex)
	storage.lastSavedId = lastId
}
