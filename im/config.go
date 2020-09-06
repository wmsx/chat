package main

type Config struct {
	port            int
	sslPort         int
	mysqlDatasource string
	pendingRoot     string

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

func readConfig() *Config {
	config := new(Config)
	config.port = 23000

	config.redisAddress = "sx-redis:6379"
	config.redisPassword = "mingchaonaxieshi"

	config.mysqlDatasource = "root:mingchaonaxieshi@tcp(sx-mysql:3306)/group_svc"

	config.storageRpcAddrs = []string{"sx-ims:13333"}
	config.groupStorageRpcAddrs = []string{"sx-imgs:13333"}

	config.routeAddrs = []string{"sx-imr:4444"}
	config.groupRouteAddrs = []string{"sx-imgr:4444"}

	config.groupDeliverCount = 1
	config.pendingRoot = "/data/im/pending"

	//config.logFilename = "/Users/zengqiang96/logs/im.log"
	config.logAge = 30
	config.logBackup = 10
	config.logCaller = false

	return config
}
