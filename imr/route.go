package main

import "sync"

type Route struct {
	appId int64
	mutex sync.Mutex
	uids  map[int64]bool
}

func NewRoute(appId int64) *Route {
	r := new(Route)
	r.appId = appId
	r.uids = make(map[int64]bool)
	return r
}

func (r *Route) IsUserOnline(uid int64) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.uids[uid]
}

func (r *Route) ContainUserID(uid int64) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, ok := r.uids[uid]
	return ok
}

func (r *Route) AddUserID(uid int64, online bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.uids[uid] = online
}

func (r *Route) RemoveUserID(uid int64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.uids, uid)
}
