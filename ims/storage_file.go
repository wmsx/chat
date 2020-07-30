package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"sync"
)

const HEADER_SIZE = 32
const MAGIC = 0x494d494d
const F_VERSION = 1 << 16            //1.0
const BLOCK_SIZE = 128 * 1024 * 1024 // 1个文件的大小 128M

type StorageFile struct {
	root  string
	mutex sync.Mutex

	dirty   bool     //是否有新的写入
	blockNo int      // 消息持久化文件的id
	file    *os.File // 持久化文件，名称和blockNo有关
}

func (storage *StorageFile) saveMessage(msg *Message) int64 {
	msgId, err := storage.file.Seek(0, io.SeekEnd)
	if err != nil {
		log.Fatalln(err)
	}

	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, int32(MAGIC))
	WriteMessage(buffer, msg)
	binary.Write(buffer, binary.BigEndian, int32(MAGIC))

	buf := buffer.Bytes()

	if msgId+int64(len(buf)) > BLOCK_SIZE { // 当前这个文件满了，需要开启下一个文件
		err := storage.file.Sync() // 同步到磁盘
		if err != nil {
			log.Fatalln("同步storage 文件失败 err: ", err)
		}
		storage.file.Close()
		storage.openWriteFile(storage.blockNo + 1)
		msgId, err = storage.file.Seek(0, io.SeekEnd)
		if err != nil {
			log.Fatalln(err)
		}
	}

	if msgId+int64(len(buf)) > BLOCK_SIZE { // 如果这个时候还满足条件，那就是消息太大了
		log.Fatalln("message size:", len(buf))
	}

	// 写入消息内容
	n, err := storage.file.Write(buf)
	if err != nil {
		log.Fatal("文件写入失败 err:", err)
	}
	if n != len(buf) {
		log.Fatal("文件写入大小不一致 write size:", len(buf), " nwrite:", n)
	}
	storage.dirty = true

	msgId = int64(storage.blockNo)*BLOCK_SIZE + msgId // msgId是当前文件的偏移量，这里计算全局的偏移
	log.Info("save message:", Command(msg.cmd), " ", msgId)
	return msgId
}

func (storage *StorageFile) openWriteFile(blockNo int) {
	path := fmt.Sprintf("%s/message_%d", storage.root, blockNo)
	log.Info("open/create message file path:", path)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	fileSize, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		log.Fatal("seek file")
	}
	if fileSize < HEADER_SIZE && fileSize > 0 { // 文件头部不正确，删除这个文件的内容
		log.Info("file header is't complete")
		err = file.Truncate(0)
		if err != nil {
			log.Fatal("truncate file")
		}
		fileSize = 0
	}
	if fileSize == 0 {
		storage.WriteHeader(file)
	}
	storage.file = file
	storage.blockNo = blockNo
	storage.dirty = false
}

func (storage *StorageFile) WriteHeader(file *os.File) {
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
