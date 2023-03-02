package abugame

import (
	"github.com/beego/beego/logs"
	"github.com/zhms/abugo"
)

var http *abugo.AbuHttp
var redis *abugo.AbuRedis
var db *abugo.AbuDb

func Init() {
	abugo.Init()
	db = new(abugo.AbuDb)
	db.Init("server.db")
	redis = new(abugo.AbuRedis)
	redis.Init("server.redis")
	http = new(abugo.AbuHttp)
	http.Init("server.http")
	http.InitWs("/capi/ws")
	logs.Debug("****************start****************")
}

func Project() string {
	return abugo.Project()
}

func Module() string {
	return abugo.Module()
}

func Env() string {
	return abugo.Env()
}

func Http() *abugo.AbuHttp {
	return http
}

func Redis() *abugo.AbuRedis {
	return redis
}

func Db() *abugo.AbuDb {
	return db
}

func Run() {
	abugo.Run()
}
