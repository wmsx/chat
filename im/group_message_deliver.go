package main

import (
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"sync"
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
	go storage.run2()
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


func (storage *GroupMessageDeliver) truncateFile() {
	err := storage.file.Truncate(HEADER_SIZE)
	if err != nil {
		log.WithField("err", err).Fatal("truncate err:", err)
	}

	storage.latestMsgId = 0
	storage.latestSentMsgId = 0

}


// 推送给群中的成员
//device_ID 发送者的设备ID
func (storage *GroupMessageDeliver) sendMessage(uid, sender, deviceID int64, msg *Message) bool {

	// publish只是发送给群成员不在当前im机器上连接
	PublishMessage(uid, msg)

	// 这里是处理连接在当前im机器上的群成员消息推送
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

func (storage *GroupMessageDeliver) DispatchMessage(msg *AppMessage) {
	group := groupManager.LoadGroup(msg.receiver)
	if group == nil {
		log.Warning("加载Group为空，不能分发Group消息")
		return
	}
	DispatchMessageToGroup(msg.msg, group, nil)
}
