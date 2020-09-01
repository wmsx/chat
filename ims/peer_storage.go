package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"time"
)

const BATCH_SIZE = 1000

const PEER_INDEX_FILE_NAME = "peer_index.v3"

type UserId struct {
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

func (storage *PeerStorage) SavePeerMessage(receiver, deviceID int64, msg *Message) (int64, int64) {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	msgId := storage.saveMessage(msg)

	userIndex := storage.getPeerIndex(receiver)

	lastId := userIndex.lastId
	lastPeerId := userIndex.lastPeerId
	lastBatchId := userIndex.lastBatchId
	lastSeqId := userIndex.lastSeqId

	off := &OfflineMessage{
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
	log.Info("receiver: ", receiver, " userIndex: ", ui)
	storage.setPeerIndex(receiver, ui)
	return msgId, userIndex.lastMsgId
}

func (storage *PeerStorage) SavePeerGroupMessage(members []int64, deviceID int64, msg *Message) []int64 {
	r := make([]int64, 0, len(members)*2)
	for _, receiver := range members {
		msgId, prevMsgId := storage.SavePeerMessage(receiver, deviceID, msg)
		r = append(r, msgId)
		r = append(r, prevMsgId)
	}
	return r
}

func (storage *PeerStorage) isGroupMessage(msg *Message) bool {
	return msg.cmd == MSG_GROUP_IM || msg.flag&MESSAGE_FLAG_GROUP != 0
}

func (storage *PeerStorage) getPeerIndex(receiver int64) *UserIndex {
	id := UserId{receiver}
	if ui, ok := storage.messageIndex[id]; ok {
		return ui
	}
	return &UserIndex{}
}
func (storage *PeerStorage) setPeerIndex(receiver int64, ui *UserIndex) {
	id := UserId{receiver}
	storage.messageIndex[id] = ui

	if ui.lastId > storage.lastId {
		storage.lastId = ui.lastId
	}
}

func (storage *PeerStorage) LoadHistoryMessages(receiver int64, syncMsgId int64, limit int, hardLimit int) ([]*EMessage, int64, bool) {
	var lastMsgId int64
	var lastOfflineMsgId int64

	msgIndex := storage.getPeerIndex(receiver)

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
		if msg == nil {
			break
		}

		off, ok := msg.body.(*OfflineMessage)
		if !ok {
			log.Warning("invalid message cmd:", msg.cmd)
			break
		}

		if lastMsgId == 0 {
			lastMsgId = off.msgId
			lastOfflineMsgId = lastId
		}

		if off.msgId <= syncMsgId {
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

	if len(messages) > 1000 {
		log.Warningf("uid:%d sync msgid:%d history message overflow:%d",
			receiver, syncMsgId, len(messages))
	}

	var hasMore bool
	if msgIndex != nil && lastOfflineMsgId > 0 && lastOfflineMsgId < msgIndex.lastId {
		hasMore = true
	}
	return messages, lastMsgId, hasMore
}

func (storage *PeerStorage) clonePeerIndex() map[UserId]*UserIndex {
	messageIndex := make(map[UserId]*UserIndex)
	for k, v := range storage.messageIndex {
		messageIndex[k] = v
	}
	return messageIndex
}

func (storage *PeerStorage) savePeerIndex(messageIndex map[UserId]*UserIndex) {
	path := fmt.Sprintf("%s/peer_index_t", storage.root)
	log.Info("write peer message index path:", path)
	begin := time.Now().UnixNano()
	log.Info("flush peer index begin:", begin)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal("open file:", err)
	}
	defer file.Close()

	buffer := new(bytes.Buffer)
	index := 0
	for id, value := range messageIndex {
		binary.Write(buffer, binary.BigEndian, id.uid)
		binary.Write(buffer, binary.BigEndian, value.lastMsgId)
		binary.Write(buffer, binary.BigEndian, value.lastId)
		binary.Write(buffer, binary.BigEndian, value.lastPeerId)
		binary.Write(buffer, binary.BigEndian, value.lastBatchId)
		binary.Write(buffer, binary.BigEndian, value.lastSeqId)

		index += 1

		if index%1000 == 0 {
			buf := buffer.Bytes()
			n, err := file.Write(buf)
			if err != nil {
				log.Fatal("write file:", err)
			}
			if n != len(buf) {
				log.Fatal("can't write file:", len(buf), n)
			}
			buffer.Reset()
		}
	}

	buf := buffer.Bytes()
	n, err := file.Write(buf)
	if err != nil {
		log.Fatal("write file:", err)
	}
	if n != len(buf) {
		log.Fatal("can't write file:", len(buf), n)
	}

	err = file.Sync()
	if err != nil {
		log.Info("sync file err:", err)
	}

	path2 := fmt.Sprintf("%s/%s", storage.root, PEER_INDEX_FILE_NAME)
	err = os.Rename(path, path2)
	if err != nil {
		log.Fatal("rename peer index file err:", err)
	}

	end := time.Now().UnixNano()
	log.Info("flush peer index end:", end, " used:", end-begin)
}

func (storage *PeerStorage) repairPeerIndex() {
	log.Info("修复message index开始:", storage.lastId, time.Now().UnixNano())
	first := storage.getBlockNo(storage.lastId)
	off := storage.getBlockOffset(storage.lastId)

	for i := first; i <= storage.blockNo; i++ {
		file := storage.openReadFile(i)
		if file == nil {
			//历史消息被删除
			continue
		}

		offset := HEADER_SIZE
		if i == first {
			offset = off
		}
		_, err := file.Seek(int64(offset), io.SeekStart)
		if err != nil {
			log.Warning("seek file err:", err)
			file.Close()
			break
		}
		for {
			msgId, err := file.Seek(0, io.SeekCurrent)
			if err != nil {
				log.Info("seek file err:", err)
				break
			}
			msg := storage.ReadMessage(file)
			if msg == nil {
				break
			}
			blockNo := i
			msgId = int64(blockNo)*BLOCK_SIZE + msgId
			if msgId == storage.lastId {
				continue
			}
			storage.execMessage(msg, msgId)
		}
		file.Close()
	}
	log.Info("修复message index结束:", storage.lastId, time.Now().UnixNano())

}

func (storage *PeerStorage) execMessage(msg *Message, msgId int64) {
	if msg.cmd == MSG_OFFLINE {
		off := msg.body.(*OfflineMessage)
		lastPeerId := msgId

		index := storage.getPeerIndex(off.receiver)
		if (msg.flag & MESSAGE_FLAG_GROUP) != 0 {
			lastPeerId = index.lastPeerId
		}
		lastBatchId := index.lastBatchId
		lastSeqId := index.lastSeqId + 1
		if lastSeqId%BATCH_SIZE == 0 {
			lastBatchId = msgId
		}

		ui := &UserIndex{off.msgId, msgId, lastPeerId, lastBatchId, lastSeqId}
		storage.setPeerIndex(off.receiver, ui)
	}

}

func (storage *PeerStorage) readPeerIndex() bool {
	path := fmt.Sprintf("%s/%s", storage.root, PEER_INDEX_FILE_NAME)
	log.Info("read message index path:", path)
	file, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatal("open file:", err)
		}
		return false
	}
	defer file.Close()
	const INDEX_SIZE = 56
	data := make([]byte, INDEX_SIZE*1000)

	for {
		n, err := file.Read(data)
		if err != nil {
			if err != io.EOF {
				log.Fatal("read err:", err)
			}
			break
		}
		n = n - n%INDEX_SIZE
		buffer := bytes.NewBuffer(data[:n])
		for i := 0; i < n/INDEX_SIZE; i++ {
			id := UserId{}
			var lastMsgId int64
			var lastId int64
			var peerMsgId int64
			var batchId int64
			var seqId int64
			binary.Read(buffer, binary.BigEndian, &id.uid)
			binary.Read(buffer, binary.BigEndian, &lastMsgId)
			binary.Read(buffer, binary.BigEndian, &lastId)
			binary.Read(buffer, binary.BigEndian, &peerMsgId)
			binary.Read(buffer, binary.BigEndian, &batchId)
			binary.Read(buffer, binary.BigEndian, &seqId)
			ui := &UserIndex{lastMsgId, lastId, peerMsgId, batchId, seqId}
			storage.setPeerIndex(id.uid, ui)
		}
	}
	return true
}
