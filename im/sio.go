package main

import (
	"bytes"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func ReadWebsocketMessage(conn *websocket.Conn) *Message {
	messageType, p, err := conn.ReadMessage()
	if err != nil {
		log.WithField("err", err).Info("读取wesocket失败")
		return nil
	}
	if messageType == websocket.BinaryMessage {
		return ReadBinaryMessage(p)
	} else {
		log.WithField("消息类型", messageType).Error("不支持的websocket消息类型")
		return nil
	}
}

func ReadBinaryMessage(p []byte) *Message  {
	reader := bytes.NewBuffer(p)
	return ReceiveClientMessage(reader)
}