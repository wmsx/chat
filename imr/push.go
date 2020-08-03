package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
)

func (client *Client) PublishPeerMessage(appId int64, im *IMMessage) {
	v := make(map[string]interface{})
	v["appId"] = appId
	v["sender"] = im.sender
	v["receiver"] = im.receiver
	v["content"] = im.content

	b, _ := json.Marshal(v)

	queueName := "push_queue"
	client.PushChan(queueName, b)
}

func (client *Client) PushChan(queueName string, content []byte) {
	select {
	case client.pwt <- &Push{queueName: queueName, content: content}:
	default:
		log.Warning("rpush message timeout")
	}
}

func (client *Client) Push() {
	//单次入redis队列消息限制
	const PUSH_LIMIT = 1000
	closed := false
	ps := make([]*Push, 0, PUSH_LIMIT)

	for {
		for !closed {
			select {
			case p := <-client.pwt:
				if p == nil {
					closed = true
				} else {
					ps = append(ps, p)
					if len(ps) >= PUSH_LIMIT {

					}
				}
			}
		}

		if len(ps) > 0 {
			client.PushQueue(ps)
		}
	}
}

func (client *Client) PushQueue(ps []*Push) {
	conn := redisPool.Get()
	defer conn.Close()

	conn.Send("MULTI")
	for _, p := range ps {
		conn.Send("RPUSH", p.queueName, p.content)
	}

	_, err := conn.Do("EXEC")
	if err != nil {
		log.Info("multi rpush error:", err)
	}
}
