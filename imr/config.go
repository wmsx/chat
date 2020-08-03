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

	config.listen = ":4444"

	return config
}
