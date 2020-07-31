package test

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"net"
)

func main()  {
	conn, err := net.Dial("tcp", "127.0.0.1:23000")
	if err != nil {
		log.Error(err)
		return
	}
	defer conn.Close()

	im := &IMMessage{
		sender:1,
		receiver:2,
		content:"test im",
	}

	m := &Message{cmd:MSG_IM, seq:1, version:DEFAULT_VERSION, body:im}

	buffer := new(bytes.Buffer)
	WriteMessage(buffer, m)

	conn.Write(buffer.Bytes())
}