package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
)

func LoadUserAccessToken(token string) (int64, error) {
	conn := redisPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("access_token_%s", token)
	var uid int64

	err := conn.Send("EXISTS", key)
	if err != nil {
		return 0, err
	}
	err = conn.Send("HMGET", key, "user_id", "app_id")
	if err != nil {
		return 0, err
	}
	err = conn.Flush()
	if err != nil {
		return 0, err
	}
	exists, err := redis.Bool(conn.Receive())
	if err != nil {
		return 0, err
	}
	reply, err := redis.Values(conn.Receive())
	if err != nil {
		return 0, err
	}

	if !exists {
		return 0, err
	}
	_, err = redis.Scan(reply, &uid)
	if err != nil {
		return 0, err
	}

	return uid, nil
}

func GetSyncKey(uid int64) int64 {
	conn := redisPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("users_%d", uid)

	origin, err := redis.Int64(conn.Do("HGET", key, "sync_key"))
	if err != nil && err != redis.ErrNil {
		log.WithField("err", err).Info("hget失败")
		return 0
	}
	return origin
}

func SaveSyncKey(uid int64, syncKey int64) {
	conn := redisPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("users_%d", uid)

	_, err := conn.Do("HSET", key, "sync_key", syncKey)
	if err != nil {
		log.WithField("err", err).Warning("hset失败")
	}
}

func GetGroupSyncKey(uid int64, groupId int64) int64 {
	conn := redisPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("users_%d", uid)
	field := fmt.Sprintf("group_sync_key_%d", groupId)

	origin, err := redis.Int64(conn.Do("HGET", key, field))
	if err != nil {
		return 0
	}
	return origin
}

func SaveGroupSyncKey(uid, gid, syncKey int64) {
	conn := redisPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("users_%d", uid)
	field := fmt.Sprintf("group_sync_key_%d", gid)

	_, err := conn.Do("HSET", key, field, syncKey)
	if err != nil {
		log.WithField("err", err).Warning("hset group sync key失败")
	}
}
