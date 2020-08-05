package main

import (
	"bytes"
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

	callbackMutex    sync.Mutex               // callback变量的锁
	id               int64                    //自增的callback id
	callbacks        map[int64]chan *Metadata //返回保存到ims的消息id
	callbackId2msgId map[int64]int64          //callback -> msgId
}

func NewGroupMessageDeliver(root string) *GroupMessageDeliver {
	storage := new(GroupMessageDeliver)

	storage.root = root
	if _, err := os.Stat(root); os.IsNotExist(err) {
		err = os.Mkdir(root, 0755)
		if err != nil {
			log.WithField("err", err).Fatal("GroupMessageDeliver mkdir 失败")
		}
	} else if err != nil {
		log.WithField("err", err).Fatal("GroupMessageDeliver stat 失败")
	}

	storage.openWriteFile()

	storage.wt = make(chan int64, 10)
	storage.lt = make(chan *GroupLoader)
	storage.callbacks = make(map[int64]chan *Metadata)
	storage.callbackId2msgId = make(map[int64]int64)

	return storage
}

func (storage *GroupMessageDeliver) openWriteFile() {
	path := fmt.Sprintf("%s/pending_group_messages", storage.root)
	log.Info("open/create message file path:", path)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.WithField("err", err).Fatal("open file:", err)
	}
	fileSize, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		log.WithField("err", err).Fatal("seek file")
	}
	if fileSize < HEADER_SIZE && fileSize > 0 {
		log.Info("GroupMessageDeliver 文件头部不完整")
		err := file.Truncate(0)
		if err != nil {
			log.WithField("err", err).Fatal("truncate file")
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
	log.WithFields(log.Fields{"latestMsgId": latestMsgId, "latestSentMsgId": storage.latestSentMsgId}).Info("刷入pending message到磁盘")

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
		log.WithField("err", err).Fatal("truncate err:", err)
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
		log.WithField("err", err).Error("seek file失败")
		return
	}

	for {
		msgId, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			log.WithField("err", err).Error("seek file失败")
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
		storage.DoCallback(msgId, meta)
		storage.latestSentMsgId = msgId
	}
}

func (storage *GroupMessageDeliver) sendGroupMessage(gm *PendingGroupMessage) (*Metadata, bool) {
	msg := &IMMessage{sender: gm.sender, receiver: gm.gid, timestamp: gm.timestamp, content: gm.content}
	m := &Message{cmd: MSG_GROUP_IM, version: DEFAULT_VERSION, body: msg}

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

	metadata := &Metadata{}
	for _, mb := range batchMembers {
		r, err := SavePeerGroupMessage(gm.appId, mb, gm.deviceID, m)
		if err != nil {
			log.WithFields(log.Fields{"sender": gm.sender, "gid": gm.gid, "err": err}).Error("保存peer group message失败")
			return nil, false
		}
		if len(r) != len(mb)*2 {
			log.WithFields(log.Fields{"len(r)": len(r), "len(mb)": len(mb)}).Error("保存peer group message失败")
			return nil, false
		}
		for i := 0; i < len(r); i += 2 {
			msgId, prevMsgId := r[i], r[i+1]
			member := mb[i/2]
			meta := &Metadata{syncKey: msgId, prevSyncKey: prevMsgId}
			mm := &Message{cmd: MSG_GROUP_IM, version: DEFAULT_VERSION, flag: MESSAGE_FLAG_PUSH, body: msg, meta: meta}
			storage.sendMessage(gm.appId, member, gm.sender, gm.deviceID, mm)

			notify := &Message{cmd: MSG_SYNC_NOTIFY, body: &SyncKey{syncKey: msgId}}
			storage.sendMessage(gm.appId, member, gm.sender, gm.deviceID, notify)

			if member == gm.sender {
				metadata.syncKey = msgId
				metadata.prevSyncKey = prevMsgId
			}
		}
	}

	groupMembers := make(map[int64]int64)
	for _, member := range gm.members {
		groupMembers[member] = 0
	}
	// 离线推送，不需要
	//group := NewGroup(gm.gid, gm.appId, groupMembers)
	//PushGroupMessage(gm.appId, group, m)
	return metadata, true
}

// 推送给群中的成员
//device_ID 发送者的设备ID
func (storage *GroupMessageDeliver) sendMessage(appId, uid, sender, deviceID int64, msg *Message) bool {

	// publish只是发送给群成员不在当前im机器上连接
	PublishMessage(appId, uid, msg)

	// 这里是处理连接在当前im机器上的群成员消息推送
	route := appRoute.FindRoute(appId)
	if route == nil {
		log.WithFields(log.Fields{"appId": appId, "uid": uid, "cmd": msg.cmd}).Warn("不能发送消息")
		return false
	}

	clients := route.FindClientSet(uid)
	if len(clients) == 0 {
		return false
	}

	for c, _ := range clients {
		// 不发送给自己
		if c.deviceID == deviceID && sender == uid {
			continue
		}
		c.EnqueueMessage(msg)
	}
	return true
}

//save without lock
func (storage *GroupMessageDeliver) saveMessage(msg *Message) int64 {
	msgId, err := storage.file.Seek(0, io.SeekEnd)
	if err != nil {
		log.Fatalln(err)
	}

	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, int32(MAGIC))

	body := msg.ToData()
	var msgLen = MSG_HEADER_SIZE + int32(len(body))
	binary.Write(buffer, binary.BigEndian, msgLen)

	WriteHeader(int32(len(body)), int32(msg.seq), byte(msg.cmd),
		byte(msg.version), byte(msg.flag), buffer)
	buffer.Write(body)

	binary.Write(buffer, binary.BigEndian, msgLen)
	binary.Write(buffer, binary.BigEndian, int32(MAGIC))
	buf := buffer.Bytes()

	n, err := storage.file.Write(buf)
	if err != nil {
		log.WithField("err", err).Fatal("file write失败")
	}
	if n != len(buf) {
		log.Fatal("file write size:", len(buf), " nwrite:", n)
	}

	log.Info("save message:", Command(msg.cmd), " ", msgId)
	return msgId
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

func (storage *GroupMessageDeliver) SaveMessage(msg *Message, ch chan *Metadata) int64 {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	msgId := storage.saveMessage(msg)
	atomic.StoreInt64(&storage.latestMsgId, msgId)

	var callbackId int64
	if ch != nil {
		callbackId = storage.AddCallback(msgId, ch)
	}

	select {
	case storage.wt <- msgId:
	default:
	}
	return callbackId
}

func (storage *GroupMessageDeliver) AddCallback(msgId int64, ch chan *Metadata) int64 {
	storage.callbackMutex.Lock()
	defer storage.callbackMutex.Unlock()
	storage.id += 1
	storage.callbacks[msgId] = ch
	storage.callbackId2msgId[storage.id] = msgId
	return storage.id
}

func (storage *GroupMessageDeliver) RemoveCallback(callbackId int64) {
	storage.callbackMutex.Lock()
	defer storage.callbackMutex.Unlock()

	if msgId, ok := storage.callbackId2msgId[callbackId]; ok {
		delete(storage.callbackId2msgId, callbackId)
		delete(storage.callbacks, msgId)
	}
}

func (storage *GroupMessageDeliver) DoCallback(msgId int64, meta *Metadata) {
	storage.callbackMutex.Lock()
	defer storage.callbackMutex.Unlock()

	if ch, ok := storage.callbacks[msgId]; ok {
		ch <- meta
	}
}

func (storage *GroupMessageDeliver) DispatchMessage(msg *AppMessage) {
	group := groupManager.LoadGroup(msg.receiver)
	if group == nil {
		log.Warning("加载Group为空，不能分发Group消息")
		return
	}
	DispatchMessageToGroup(msg.msg, group, msg.appId, nil)
}
