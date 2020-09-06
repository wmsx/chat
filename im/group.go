package main

import (
	"database/sql"
	mysql "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type Group struct {
	gid   int64
	mutex sync.Mutex

	members map[int64]int64 //key:成员id value:入群时间|(mute<<31)
	ts      int             //访问时间
}

func (group *Group) GetMemberMute(uid int64) bool {
	t, _ := group.members[uid]
	return int((t>>31)&0x01) != 0
}

func (group *Group) Members() map[int64]int64 {
	return group.members
}

func (group *Group) GetMemberTimestamp(uid int64) int {
	ts, _ := group.members[uid]
	return int(ts & 0x7FFFFFFF)
}

func NewSuperGroup(gid int64, members map[int64]int64) *Group {
	return &Group{
		gid:     gid,
		members: members,
		ts:      int(time.Now().Unix()),
	}
}

func LoadGroup(db *sql.DB, groupId int64) (*Group, error) {
	members, err := LoadGroupMember(db, groupId)
	if err != nil {
		log.Info("error:", err)
		return nil, err
	}

	group := NewSuperGroup(groupId, members)
	log.Info("load group success:", groupId)
	return group, nil
}

func LoadGroupMember(db *sql.DB, groupId int64) (map[int64]int64, error) {
	stmtIns, err := db.Prepare("SELECT member_id, timestamp, mute FROM `t_discuss_group_member` WHERE group_id = ? AND deleted is null ")
	if err == mysql.ErrInvalidConn {
		log.Info("db prepare error:", err)
		stmtIns, err = db.Prepare("SELECT member_id, timestamp, mute FROM `t_discuss_group_member` WHERE group_id = ? AND deleted is null ")
	}
	if err != nil {
		log.Info("db prepare error:", err)
		return nil, err
	}

	defer stmtIns.Close()
	members := make(map[int64]int64)
	rows, err := stmtIns.Query(groupId)
	for rows.Next() {
		var uid int64
		var timestamp int64
		var mute int64
		rows.Scan(&uid, &timestamp, &mute)
		members[uid] = timestamp | (mute << 31)
	}
	return members, nil
}
