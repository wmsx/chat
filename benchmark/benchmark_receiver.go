package main

import (
	"math/rand"
	"runtime"
	"time"
)

func main() {
	runtime.GOMAXPROCS(4)

	seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	redisPool = NewRedisPool(redisAddress, redisPassword, redisDB)

	receive(100)
}
