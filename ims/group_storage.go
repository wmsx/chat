package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
)

const GROUP_INDEX_FILE_NAME = "group_index.v2"

type GroupId struct {
	gid   int64
}

type GroupIndex struct {
	lastMsgId   int64
	lastId      int64
	lastBatchId int64
	lastSeqId   int64
}

type GroupStorage struct {
	*StorageFile
	messageIndex map[GroupId]*GroupIndex
}

func NewGroupStorage(f *StorageFile) *GroupStorage {
	storage := &GroupStorage{StorageFile: f}
	storage.messageIndex = make(map[GroupId]*GroupIndex)
	return storage
}

func (storage *GroupStorage) SaveGroupMessage(gid, deviceID int64, msg *Message) (int64, int64) {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	msgId := storage.saveMessage(msg)

	index := storage.getGroupIndex(gid)

	lastId := index.lastId
	lastBatchId := index.lastBatchId
	lastSeqId := index.lastSeqId

	off := OfflineMessage{}
	off.receiver = gid
	off.msgId = msgId
	off.deviceID = deviceID
	off.seqId = lastSeqId + 1
	off.prevMsgId = lastId
	off.prevPeerMsgId = 0
	off.prevBatchMsgId = lastBatchId

	m := &Message{cmd: MSG_GROUP_OFFLINE, body: off}
	lastId = storage.saveMessage(m)

	lastSeqId += 1
	if lastSeqId%BATCH_SIZE == 0 {
		lastBatchId = lastId
	}
	groupIndex := &GroupIndex{lastMsgId: msgId, lastId: lastId, lastBatchId: lastBatchId, lastSeqId: lastSeqId}
	storage.setGroupIndex(gid, groupIndex)
	return msgId, index.lastMsgId
}

func (storage *GroupStorage) getGroupIndex(gid int64) *GroupIndex {
	id := GroupId{gid: gid}
	if groupIndex, ok := storage.messageIndex[id]; ok {
		return groupIndex
	}
	return &GroupIndex{}

}

func (storage *GroupStorage) setGroupIndex(gid int64, index *GroupIndex) {
	id := GroupId{gid: gid}
	storage.messageIndex[id] = index
	if index.lastId > storage.lastId {
		storage.lastId = index.lastId
	}
}

func (storage *GroupStorage) cloneGroupIndex() map[GroupId]*GroupIndex {
	messageIndex := make(map[GroupId]*GroupIndex)
	for k, v := range storage.messageIndex {
		messageIndex[k] = v
	}
	return messageIndex
}

func (storage *Storage) saveGroupIndex(messageIndex map[GroupId]*GroupIndex) {
	path := fmt.Sprintf("%s/group_index_t", storage.root)
	log.WithField("path", path).Info("持久化群组消息索引")
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.WithField("err", err).Fatal("打开文件失败")
	}
	defer file.Close()

	buffer := new(bytes.Buffer)
	index := 0
	for id, value := range messageIndex {
		binary.Write(buffer, binary.BigEndian, id.gid)
		binary.Write(buffer, binary.BigEndian, value.lastMsgId)
		binary.Write(buffer, binary.BigEndian, value.lastId)
		binary.Write(buffer, binary.BigEndian, value.lastBatchId)
		binary.Write(buffer, binary.BigEndian, value.lastSeqId)

		index += 1
		if index%1000 == 0 {
			buf := buffer.Bytes()
			n, err := file.Write(buf)
			if err != nil {
				log.WithField("err", err).Fatal("写入群组消息索引文件失败")
			}
			if n != len(buf) {
				log.WithFields(log.Fields{"len": len(buf), "all": n}).Fatal("写入群组消息索引文件丢失数据")
			}
		}
		buffer.Reset()
	}

	buf := buffer.Bytes()
	n, err := file.Write(buf)
	if err != nil {
		log.WithField("err", err).Fatal("写入群组消息索引文件失败")
	}
	if n != len(buf) {
		log.WithFields(log.Fields{"len": len(buf), "all": n}).Fatal("写入群组消息索引文件丢失数据")
	}
	err = file.Sync()
	if err != nil {
		log.WithField("err", err).Fatal("sync群组消息索引文件失败")
	}

	rename := fmt.Sprintf("%s/%s", storage.root, GROUP_INDEX_FILE_NAME)
	err = os.Rename(path, rename)
	if err != nil {
		log.WithField("err", err).Fatal("重命名群组消息索引文件失败")
	}
	log.Info("群组消息索引文件刷入到磁盘成功")
}

//获取所有消息id大于msgid的消息
//ts:入群时间
func (storage *GroupStorage) LoadGroupHistoryMessage(uid, gid, msgId int64, ts int32, limit int) ([]*EMessage, int64) {
	log.WithFields(log.Fields{"msgId": msgId, "ts": ts}).Info("加载群组历史消息")
	messageIndex := storage.getGroupIndex(gid)
	lastId := messageIndex.lastId

	var lastMsgId int64
	c := make([]*EMessage, 0, 10)

	for ; lastId > 0; {
		msg := storage.LoadMessage(lastId)
		if msg == nil {
			log.WithField("msgId", msgId).Warning("加载群组消息失败")
			break
		}

		off := msg.body.(*OfflineMessage)

		if lastMsgId == 0 {
			lastMsgId = off.msgId
		}

		if off.msgId == 0 || off.msgId <= msgId {
			break
		}

		m  := storage.LoadMessage(off.msgId)
		if msgId == 0 && m.cmd == MSG_GROUP_IM {
			im := m.body.(*IMMessage)
			if im.timestamp < ts {
				break
			}
		}
		c = append(c, &EMessage{msgId:off.msgId, deviceId:off.deviceID, msg:m})

		lastId = off.prevMsgId

		if len(c) >= limit {
			break
		}
	}

	return c, lastMsgId
}
