package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
)

func GetDeviceID(deviceId string, platformId int) (int64, error) {
	conn := redisPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("devices_%s_%d", deviceId, platformId)
	deviceID, err := redis.Int64(conn.Do("GET", key))
	if err != nil {
		k := "devices_id"
		deviceID, err = redis.Int64(conn.Do("INCR", k))
		if err != nil {
			return 0, err
		}
		_, err = conn.Do("SET", key, deviceID)
		if err != nil {
			return 0, err
		}
	}
	return deviceID, nil
}
