package main

import "sync"

type ClientSet map[*Client]struct{}

func NewClientSet() ClientSet {
	return map[*Client]struct{}{}
}

func (set ClientSet) Add(c *Client) {
	set[c] = struct{}{}
}

func (set ClientSet) Remove(c *Client) {
	if _, ok := set[c]; !ok {
		return
	}
	delete(set, c)
}

type AppRoute struct {
	mutex sync.Mutex
	apps  map[int64]*Route
}

func (r *AppRoute) FindRoute(appId int64) *Route {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.apps[appId]
}

func (r *AppRoute) AddRoute(route *Route) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.apps[route.appId] = route
}
