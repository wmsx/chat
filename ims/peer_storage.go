package main

import log "github.com/sirupsen/logrus"

const BATCH_SIZE = 1000

type UserId struct {
	appId int64
	uid   int64
}

// 用于找到上一条消息
type UserIndex struct {
	lastMsgId   int64
	lastId      int64
	lastPeerId  int64
	lastBatchId int64
	lastSeqId   int64 // 纯粹用于计数，用于计算得到lastBatchId
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

func (storage *PeerStorage) SavePeerMessage(appId, receiver, deviceID int64, msg *Message) (int64, int64) {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	msgId := storage.saveMessage(msg)

	userIndex := storage.getPeerIndex(appId, receiver)

	lastId := userIndex.lastId
	lastPeerId := userIndex.lastPeerId
	lastBatchId := userIndex.lastBatchId
	lastSeqId := userIndex.lastSeqId

	off := &OfflineMessage{
		appId:          appId,
		receiver:       receiver,
		msgId:          msgId,
		deviceID:       deviceID,
		seqId:          lastSeqId + 1,
		prevMsgId:      lastId,
		prevPeerMsgId:  lastPeerId,
		prevBatchMsgId: lastBatchId,
	}

	var flag int
	if storage.isGroupMessage(msg) {
		flag = MESSAGE_FLAG_GROUP
	}

	m := &Message{cmd: MSG_OFFLINE, flag: flag, body: off}
	lastId = storage.saveMessage(m)

	if !storage.isGroupMessage(m) {
		lastPeerId = lastId
	}

	lastSeqId += 1
	if lastSeqId%BATCH_SIZE == 0 {
		lastBatchId = lastId
	}

	ui := &UserIndex{lastMsgId: msgId, lastId: lastId, lastPeerId: lastPeerId, lastBatchId: lastBatchId, lastSeqId: lastSeqId}
	storage.setPeerIndex(appId, receiver, ui)
	return msgId, userIndex.lastMsgId
}

func (storage *PeerStorage) SavePeerGroupMessage(appId int64, members []int64, deviceID int64, msg *Message) []int64 {
	r := make([]int64, 0, len(members)*2)
	for _, receiver := range members {
		msgId, prevMsgId := storage.SavePeerMessage(appId, receiver, deviceID, msg)
		r = append(r, msgId)
		r = append(r, prevMsgId)
	}
	return r
}

func (storage *PeerStorage) isGroupMessage(msg *Message) bool {
	return msg.cmd == MSG_GROUP_IM || msg.flag&MESSAGE_FLAG_GROUP != 0
}

func (storage *PeerStorage) getPeerIndex(appId, receiver int64) *UserIndex {
	id := UserId{appId, receiver}
	if ui, ok := storage.messageIndex[id]; ok {
		return ui
	}
	return &UserIndex{}
}
func (storage *PeerStorage) setPeerIndex(appId, receiver int64, ui *UserIndex) {
	id := UserId{appId, receiver}
	storage.messageIndex[id] = ui
}

func (storage *PeerStorage) LoadHistoryMessages(appId int64, receiver int64, syncMsgId int64, limit int, hardLimit int) ([]*EMessage, int64, bool) {
	var lastMsgId int64
	var lastOfflineMsgId int64

	msgIndex := storage.getPeerIndex(appId, receiver)

	lastBatchId := msgIndex.lastBatchId

	batchCount := limit / BATCH_SIZE
	if batchCount == 0 {
		batchCount = 1 //默认是BATCH_SIZE
	}

	batchIds := make([]int64, 0, 10)

	for {
		if lastBatchId <= syncMsgId { //
			break
		}
		msg := storage.LoadMessage(lastBatchId)
		if msg == nil {
			break
		}

		off, ok := msg.body.(*OfflineMessage)
		if !ok {
			log.Warning("invalid message cmd:", msg.cmd)
			break
		}

		if off.msgId <= syncMsgId {
			break
		}

		batchIds = append(batchIds, lastBatchId)
		lastBatchId = off.prevBatchMsgId
	}

	var lastId int64
	if len(batchIds) >= batchCount {
		index := len(batchIds) - batchCount
		lastId = batchIds[index]
	} else if msgIndex != nil {
		lastId = msgIndex.lastId
	}

	messages := make([]*EMessage, 0, 10)
	for {
		msg := storage.LoadMessage(lastId)
		off, ok := msg.body.(*OfflineMessage)
		if !ok {
			log.Warning("invalid message cmd:", msg.cmd)
			break
		}

		if lastMsgId == 0 {
			lastMsgId = off.msgId
			lastOfflineMsgId = lastId
		}

		if off.msgId < syncMsgId {
			break
		}

		msg = storage.LoadMessage(off.msgId)
		if msg == nil {
			break
		}

		emsg := &EMessage{msgId: off.msgId, deviceId: off.deviceID, msg: msg}
		messages = append(messages, emsg)
		if limit > 0 && len(messages) >= limit {
			break
		}
		lastId = off.prevMsgId
	}

	if len(messages) < 1000 {
		log.Warningf("appid:%d uid:%d sync msgid:%d history message overflow:%d",
			appId, receiver, syncMsgId, len(messages))
	}

	var hasMore bool
	if msgIndex != nil && lastOfflineMsgId > 0 && lastOfflineMsgId < msgIndex.lastId {
		hasMore = true
	}
	return messages, lastMsgId, hasMore
}
