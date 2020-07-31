package main

import "fmt"

func LoadUserAccessToken(token string) (int64, int64, error) {
	conn := redisPool.Get()
	defer conn.Close()

	key := fmt.Sprintf("access_token_%s", token)
	var uid int64
	var appId int64

	err := conn.Send("EXISTS", key)
	if err != nil  {
		return 0, 0, nil
	}

	return appId, uid, nil
}
