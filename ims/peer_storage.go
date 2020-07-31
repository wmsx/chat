package main

type UserId struct {
	appId int64
	uid   int64
}

type UserIndex struct {
	lastMsgId   int64
	lastId      int64
	lastPeerId  int64
	lastBatchId int64
	lastSeqId   int64
}

//在取离线消息时，可以对群组消息和点对点消息分别获取，
//这样可以做到分别控制点对点消息和群组消息读取量，避免单次读取超量的离线消息
type PeerStorage struct {
	*StorageFile

	//消息索引全部放在内存中,在程序退出时,再全部保存到文件中，
	//如果索引文件不存在或上次保存失败，则在程序启动的时候，从消息DB中重建索引，这需要遍历每一条消息
	messageIndex map[UserId]*UserIndex
}

func NewPeerStorage(f *StorageFile) *PeerStorage {
	storage := &PeerStorage{StorageFile: f}
	storage.messageIndex = make(map[UserId]*UserIndex)
	return storage
}

func (storage *PeerStorage) SavePeerMessage(appId, receiver, deviceId int64, msg *Message) (int64, int64) {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	//userIndex := storage.getPeerIndex(appId, receiver)

	//lastId := userIndex.lastId
	//lastPeerId := userIndex.lastPeerId
	//lastBatchId := userIndex.lastBatchId
	//lastSeqId := userIndex.lastSeqId

	msgId := storage.saveMessage(msg)
	return msgId, 0
}

func (storage *PeerStorage) getPeerIndex(appId, receiver int64) *UserIndex {
	id := UserId{appId, receiver}
	if ui, ok := storage.messageIndex[id]; ok {
		return ui
	}
	return &UserIndex{}
}
