package main

// 超级群离线消息数量限制，超过的部分会被丢弃
const GROUP_OFFLINE_LIMIT = 100

type StorageConfig struct {
	rpcListen         string
	storageRoot       string
	httpListenAddress string

	syncListen    string
	masterAddress string
	isPushSystem  bool
	groupLimit    int //普通群离线消息的数量限制
	Limit         int //单次离线消息的数量限制
	hardLimit     int //离线消息总的数量限制

	logFilename string
	logLevel    string
	logBackup   int //log files
	logAge      int //days
	logCaller   bool
}

func readStorageConf() *StorageConfig {
	config := new(StorageConfig)

	config.rpcListen = ":13333"
	config.storageRoot = "/data/ims"

	//config.logFilename = "/Users/zengqiang96/logs/ims.log"
	config.logAge = 30
	config.logBackup = 10
	config.logCaller = false

	return config
}
