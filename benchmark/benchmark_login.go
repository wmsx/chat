package main

import "fmt"

const APP_ID = 7
const CHARSET = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func login(uid int64) (string, error) {
	conn := redisPool.Get()
	defer conn.Close()

	token := RandomStringWithCharset(24, CHARSET)

	key := fmt.Sprintf("access_token_%s", token)
	_, err := conn.Do("HMSET", key, "access_token", token, "user_id", uid, "app_id", APP_ID)
	if err != nil {
		return "", err
	}
	_, err = conn.Do("PEXPIRE", key, 1000*3600*4)
	if err != nil {
		return "", err
	}

	return token, nil
}

func RandomStringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
