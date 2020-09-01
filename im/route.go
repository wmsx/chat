package main

import (
	log "github.com/sirupsen/logrus"
	"sync"
)

type Route struct {
	mutex   sync.Mutex
	clients map[int64]ClientSet
}

func NewRoute() *Route {
	r := new(Route)
	r.clients = make(map[int64]ClientSet)
	return r
}

func (r *Route) AddClient(client *Client) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	isNew := false

	set, ok := r.clients[client.uid]
	if !ok {
		set = NewClientSet()
		r.clients[client.uid] = set
		isNew = true
	}
	set.Add(client)
	return isNew
}

func (r *Route) RemoveClient(client *Client) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if set, ok := r.clients[client.uid]; ok {
		set.Remove(client)
		if set.Count() == 0 {
			delete(r.clients, client.uid)
		}
	}
	log.Info("client non exists")
}

func (r *Route) FindClientSet(uid int64) ClientSet {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	set, ok := r.clients[uid]
	if ok {
		return set.Clone()
	}
	return nil
}
