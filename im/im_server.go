package main

import (
	log "github.com/sirupsen/logrus"
	"time"
)
import "github.com/gomodule/redigo/redis"

var config *Config
var redisPool *redis.Pool

func main() {
	config = readConfig()

	redisPool = NewRedisPool(config.redisAddress, config.redisPassword, config.redisDB)

	ListenClient(config.port)
	log.Infof("exit")
}

func NewRedisPool(server, password string, db int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     100,
		MaxActive:   500,
		IdleTimeout: 400 * time.Second,
		Dial: func() (redis.Conn, error) {
			timeout := time.Duration(2) * time.Second
			c, err := redis.Dial("tcp", server, redis.DialConnectTimeout(timeout))
			if err != nil {
				return nil, err
			}
			if len(password) > 0 {
				if _,  err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			if db > 0 && db < 16 {
				if _, err := c.Do("SELECT", db); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, nil
		},
	}
}
