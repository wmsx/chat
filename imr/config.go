package main

type RouteConfig struct {
	listen        string
	redisAddr     string
	redisPassword string
	redisDB       int
	isPushSystem  bool
	httpListenAddress string


	logFilename        string
	logLevel           string
	logBackup          int  //log files
	logAge             int  //days
	logCaller          bool
}

func readConfig() *RouteConfig  {
	config := new(RouteConfig)

	config.listen = ":4445"


	//config.logFilename = "/Users/zengqiang96/logs/imr.log"
	config.logAge = 30
	config.logBackup = 10
	config.logCaller = false


	return config
}
