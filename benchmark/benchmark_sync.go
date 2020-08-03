package main

import (
	"math/rand"
	"runtime"
	"time"
)

func main3() {
	runtime.GOMAXPROCS(4)

	seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	redisPool = NewRedisPool(redisAddress, redisPassword, redisDB)

	sync(100)
}
