package main

import (
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const HEADER_SIZE = 32
const MAGIC = 0x494d494d
const F_VERSION = 1 << 16 //1.0

type GroupLoader struct {
	gid int64
	c   chan *Group
}

//后台发送普通群消息
//普通群消息首先保存到临时文件中，之后按照保存到文件中的顺序依次派发
type GroupMessageDeliver struct {
	root  string
	mutex sync.Mutex
	file  *os.File

	wt chan int64 //通知有新消息等待发送

	latestMsgId     int64 //最近保存的消息id
	latestSentMsgId int64 //最近发送出去的消息id

	//保证单个群组结构只会在一个线程中被加载
	lt chan *GroupLoader //加载group结构到内存
}

func NewGroupMessageDeliver(root string) *GroupMessageDeliver {
	storage := new(GroupMessageDeliver)

	storage.root = root
	if _, err := os.Stat(root); os.IsNotExist(err) {
		err = os.Mkdir(root, 0755)
		if err != nil {
			log.Fatal("mkdir err:", err)
		}
	} else if err != nil {
		log.Fatal("stat err:", err)
	}

	storage.openWriteFile()

	return storage
}

func (storage *GroupMessageDeliver) openWriteFile() {
	path := fmt.Sprintf("%s/pending_group_messages", storage.root)
	log.Info("open/create message file path:", path)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal("open file:", err)
	}
	fileSize, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		log.Fatal("seek file")
	}
	if fileSize < HEADER_SIZE && fileSize > 0 {
		log.Info("file header is't complete")
		err := file.Truncate(0)
		if err != nil {
			log.Fatal("truncate file")
		}
		fileSize = 0
	}
	if fileSize == 0 {
		storage.WriteHeader(file)
	}
	storage.file = file
}

func (storage *GroupMessageDeliver) WriteHeader(file *os.File) {
	var m int32 = MAGIC
	err := binary.Write(file, binary.BigEndian, m)
	if err != nil {
		log.Fatalln(err)
	}
	var v int32 = F_VERSION
	err = binary.Write(file, binary.BigEndian, v)
	if err != nil {
		log.Fatalln(err)
	}

	pad := make([]byte, HEADER_SIZE-8)
	n, err := file.Write(pad)
	if err != nil || n != (HEADER_SIZE-8) {
		log.Fatalln(err)
	}
}

func (storage *GroupMessageDeliver) Start() {
	go storage.run()
	go storage.run2()
}

func (storage *GroupMessageDeliver) run() {
	log.Info("group message deliver running")

	for {
		select {
		case <-storage.wt:
			storage.flushPendingMessage()
		case <-time.After(30 * time.Second):
			storage.flushPendingMessage()
		}
	}
}

func (storage *GroupMessageDeliver) run2() {
	log.Info("group message deliver running loop2")
	for {
		select {
		case groupLoader := <-storage.lt:
			storage.loadGroup(groupLoader)
		}
	}
}

func (storage *GroupMessageDeliver) loadGroup(groupLoader *GroupLoader) {
	group := groupManager.LoadGroup(groupLoader.gid)
	groupLoader.c <- group
}

func (storage *GroupMessageDeliver) flushPendingMessage() {
	latestMsgId := atomic.LoadInt64(&storage.latestMsgId)
	log.Infof("flush pending message latest msgid:%d latest sended msgid:%d",
		latestMsgId, storage.latestSentMsgId)

	if latestMsgId > storage.latestSentMsgId {
		storage.sendPendingMessage()
		if storage.latestSentMsgId > 128*1024*1024 {
			storage.mutex.Lock()
			defer storage.mutex.Unlock()
			latestMsgId = atomic.LoadInt64(&storage.latestMsgId)
			if latestMsgId > storage.latestSentMsgId {
				storage.sendPendingMessage()
			}
			if latestMsgId == storage.latestSentMsgId {
				storage.truncateFile()
			}
		}
	}
}

func (storage *GroupMessageDeliver) truncateFile() {
	err := storage.file.Truncate(HEADER_SIZE)
	if err != nil {
		log.Fatal("truncate err:", err)
	}

	storage.latestMsgId = 0
	storage.latestSentMsgId = 0

}

func (storage *GroupMessageDeliver) sendPendingMessage() {
	file := storage.openReadFile()
	if file == nil {
		return
	}
	defer file.Close()

	offset := storage.latestSentMsgId
	if offset == 0 {
		offset = HEADER_SIZE
	}
	_, err := file.Seek(offset, io.SeekStart)
	if err != nil {
		log.Error("seek file err:", err)
		return
	}

	for {
		msgId, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			log.Error("seek file err:", err)
			break
		}

		msg := storage.ReadMessage(file)
		if msg == nil {
			break
		}

		if msgId <= storage.latestSentMsgId {
			continue
		}

		if msg.cmd != MSG_PENDING_GROUP_MESSAGE {
			continue
		}
		gm := msg.body.(*PendingGroupMessage)
		meta, r := storage.sendGroupMessage(gm)
		if !r {
			log.Warning("send group message failure")
			break
		}
		storage.latestSentMsgId = msgId
	}
}

func (storage *GroupMessageDeliver) sendGroupMessage(gm *PendingGroupMessage) (*Metadata, bool) {
	msg := &IMMessage{sender: gm.sender, receiver: gm.gid, timestamp: gm.timestamp, content: gm.content}
	m := &Message{cmd: MSG_GROUP_IM, version: DEFAULT_VERSION, body: msg}

	metadata := &Metadata{}
	batchMembers := make(map[int64][]int64)

	for _, member := range gm.members {
		index := GetStorageRPCIndex(member)
		if _, ok := batchMembers[index]; !ok {
			batchMembers[index] = []int64{member}
		} else {
			mb := batchMembers[index]
			mb = append(mb, member)
			batchMembers[index] = mb
		}
	}

	for _, mb := range batchMembers {
		r, err := SavePeerGroupMessage(gm.appId, mb, gm.deviceID, m)
		if err != nil {
			log.Errorf("save peer group message:%d %d err:%s", gm.sender, gm.gid, err)
			return nil, false
		}
	}
}

func (storage *GroupMessageDeliver) ReadMessage(file *os.File) *Message {
	//校验消息起始位置的magic
	var magic int32
	err := binary.Read(file, binary.BigEndian, &magic)
	if err != nil {
		log.Info("read file err:", err)
		return nil
	}

	if magic != MAGIC {
		log.Warning("magic err:", magic)
		return nil
	}

	var msgLen int32
	err = binary.Read(file, binary.BigEndian, &msgLen)
	if err != nil {
		log.Info("read file err:", err)
		return nil
	}

	msg := ReceiveStorageMessage(file)
	if msg == nil {
		return msg
	}

	err = binary.Read(file, binary.BigEndian, &msgLen)
	if err != nil {
		log.Info("read file err:", err)
		return nil
	}
	err = binary.Read(file, binary.BigEndian, &magic)
	if err != nil {
		log.Info("read file err:", err)
		return nil
	}
	if magic != MAGIC {
		log.Warning("magic err:", magic)
		return nil
	}
	return msg
}

func (storage *GroupMessageDeliver) openReadFile() *os.File {
	//open file readonly mode
	path := fmt.Sprintf("%s/pending_group_messages", storage.root)
	log.Info("open message block file path:", path)
	file, err := os.Open(path)
	if err != nil {
		log.Error("open pending_group_messages error:", err)
		return nil
	}
	return file
}

func (storage *GroupMessageDeliver) LoadGroup(groupId int64) *Group {
	group := groupManager.FindGroup(groupId)
	if group != nil {
		return group
	}

	groupLoader := &GroupLoader{gid: groupId, c: make(chan *Group)}
	storage.lt <- groupLoader

	group = <-groupLoader.c
	return group
}
