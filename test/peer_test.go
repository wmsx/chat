package test

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"testing"
	"time"
)

func TestPeerMessage(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:23000")
	if err != nil {
		log.Error(err)
		return
	}
	defer conn.Close()

	im := &IMMessage{
		sender:   1,
		receiver: 2,
		content:  "test im",
	}

	m := &Message{cmd: MSG_IM, seq: 1, version: DEFAULT_VERSION, body: im}

	buffer := new(bytes.Buffer)
	WriteMessage(buffer, m)

	conn.Write(buffer.Bytes())


	r := ReceiveClientMessage(conn)

	messageChan := make(chan *Message, 1)
	messageChan <- r

	select {
	case msg := <-messageChan:
		if msg == nil {
			fmt.Println("msg is null")
		} else {
			fmt.Println(msg)
			if status, ok := msg.body.(*MessageACK); ok {
				fmt.Println(status)
			}
		}
	case <-time.After(10 * time.Second):
		fmt.Println("timeout")
	}
}