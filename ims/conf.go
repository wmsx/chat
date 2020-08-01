package main


type StorageConfig struct {
	rpcListen          string
	storageRoot        string
	kefuAppId          int64
	httpListenAddress string

	syncListen         string
	masterAddress      string
	isPushSystem      bool
	groupLimit         int  //普通群离线消息的数量限制
	Limit               int  //单次离线消息的数量限制
	hardLimit          int  //离线消息总的数量限制

	logFilename        string
	logLevel           string
	logBackup          int  //log files
	logAge             int  //days
	logCaller          bool
}

func readStorageConf() *StorageConfig {
	return &StorageConfig{
		rpcListen: ":13333",
		storageRoot: "/Users/zengqiang96/ims",
	}
}
