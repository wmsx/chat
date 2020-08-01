package main

import (
	"database/sql"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type GroupManager struct {
	mutex  sync.Mutex
	groups map[int64]*Group
	db     *sql.DB
}

func NewGroupManager() *GroupManager {
	m := new(GroupManager)
	m.group = make(map[int64]*Group)

	db, err := sql.Open("mysql", config.mysqlDatasource)
	if err != nil {
		log.Fatal("open db:", err)
	}
	m.db = db
	return m
}

func (groupManager *GroupManager) Start() {
	groupManager.load()

	go groupManager.Run()
	go groupManager.RecycleLoop()
}

func (groupManager *GroupManager) load() {
}

func (groupManager *GroupManager) Run() {

}

func (groupManager *GroupManager) RecycleLoop() {

}

func (groupManager *GroupManager) FindGroup(gid int64) *Group {
	groupManager.mutex.Lock()
	defer groupManager.mutex.Unlock()

	if group, ok := groupManager.groups[gid]; ok {
		now := int(time.Now().Unix())
		group.ts = now
		return group
	}

	return nil
}

func (groupManager *GroupManager) LoadGroup(gid int64) *Group {
	groupManager.mutex.Lock()

	if group, ok := groupManager.groups[gid]; ok {
		now := int(time.Now().Unix())
		group.ts = now
		groupManager.mutex.Unlock()
		return group
	}

	groupManager.mutex.Unlock()

	group, err := LoadGroup(groupManager.db, gid)
	if err != nil {
		log.Warningf("load group:%d err:%s", gid, err)
		return nil
	}

	groupManager.mutex.Lock()
	groupManager.groups[gid] = group
	groupManager.mutex.Unlock()
	return group
}
