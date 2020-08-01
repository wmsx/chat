package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
)

const DEFAULT_VERSION = 2

const MSG_HEADER_SIZE = 12

func ReceiveClientMessage(conn io.Reader) *Message {
	m, _ := ReceiveLimitMessage(conn, 32*1024, true)
	return m
}

func ReceiveLimitMessage(conn io.Reader, limitSize int, external bool) (*Message, error) {
	buff := make([]byte, 12)
	_, err := io.ReadFull(conn, buff)
	if err != nil {
		log.Info("sock read error:", err)
		return nil, err
	}
	length, seq, cmd, version, flag := ReadHeader(buff)

	if length < 0 || length > limitSize {
		log.Info("invalid len:", length)
		return nil, errors.New("invalid length")
	}

	buff = make([]byte, length)
	_, err = io.ReadFull(conn, buff)
	if err != nil {
		log.Info("sock read error:", err)
		return nil, err
	}

	message := new(Message)
	message.cmd = cmd
	message.seq = seq
	message.version = version
	message.flag = flag
	if !message.FromData(buff) {
		log.Warningf("parse error:%d, %d %d %d %s", cmd, seq, version,
			flag, hex.EncodeToString(buff))
		return nil, errors.New("parse error")
	}
	return message, nil
}

func ReadHeader(buff []byte) (int, int, int, int, int) {
	var length int32
	var seq int32
	buffer := bytes.NewBuffer(buff)
	binary.Read(buffer, binary.BigEndian, &length)
	binary.Read(buffer, binary.BigEndian, &seq)

	cmd, _ := buffer.ReadByte()
	version, _ := buffer.ReadByte()
	flag, _ := buffer.ReadByte()

	return int(length), int(seq), int(cmd), int(version), int(flag)
}

func WriteMessage(w *bytes.Buffer, msg *Message) {
	body := msg.ToData()
	WriteHeader(int32(len(body)), int32(msg.seq), byte(msg.cmd), byte(msg.version), byte(msg.flag), w)
	w.Write(body)
}

func WriteHeader(len int32, seq int32, cmd byte, version byte, flag byte, buffer io.Writer) {
	binary.Write(buffer, binary.BigEndian, len)
	binary.Write(buffer, binary.BigEndian, seq)
	t := []byte{cmd, version, flag, 0}
	buffer.Write(t)
}

func SendMessage(conn io.Writer, msg *Message) error {
	buffer := new(bytes.Buffer)
	WriteMessage(buffer, msg)
	buf := buffer.Bytes()
	n, err := conn.Write(buf)
	if err != nil {
		log.Info("sock write error:", err)
		return err
	}
	if n != len(buf) {
		log.Infof("write less:%d %d", n, len(buf))
		return errors.New("write less")
	}
	return nil
}


//消息大小限制在1M
func ReceiveStorageMessage(conn io.Reader) *Message {
	m, _ := ReceiveLimitMessage(conn, 1024*1024, false)
	return m
}