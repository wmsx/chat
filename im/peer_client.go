package main

type PeerClient struct {
	*Connection
}

func (client *PeerClient) HandleMessage(msg *Message) {
	switch msg.cmd {
	case MSG_IM:
		client.HandleIMMessage(msg)
	}
}

func (client *PeerClient) HandleIMMessage(message *Message) {
	msg := message.body.(*IMMessage)

	m := &Message{cmd:MSG_IM, version:DEFAULT_VERSION, body:msg}
	_, _, _ = SaveMessage(client.appId, msg.receiver, client.deviceID, m)

}
