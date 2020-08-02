package main

import "sync"

type Channel struct {
	addr string
	wt  chan *Message

	mutex sync.Mutex
}
