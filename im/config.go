package main

type Config struct {
	port            int
	sslPort         int
	mysqlDatasource string
	pendingRoot     string

	kefuAppId int64

	redisAddress  string
	redisPassword string
	redisDB       int

	httpListenAddress string

	//websocket listen address
	wsAddress string

	wssAddress string
	certFile   string
	keyFile    string

	storageRpcAddrs      []string
	groupStorageRpcAddrs []string
	routeAddrs           []string
	groupRouteAddrs      []string //可选配置项， 超群群的route server

	groupDeliverCount int    //群组消息投递并发数量,默认4
	wordFile          string //关键词字典文件
	friendPermission  bool   //验证好友关系
	enableBlacklist   bool   //验证是否在对方的黑名单中

	memoryLimit int64 //rss超过limit，不接受新的链接

	logFilename string
	logLevel    string
	logBackup   int //log files
	logAge      int //days
	logCaller   bool
}

func readConfig() *Config{
	config := new(Config)
	config.port = 23000

	config.redisAddress = "127.0.0.1:6379"
	config.redisPassword = "mingchaonaxieshi"

	config.storageRpcAddrs = []string{"127.0.0.1:13333"}

	config.groupDeliverCount = 4
	config.pendingRoot = "/Users/zengqiang96/im/pending"

	return config
}
