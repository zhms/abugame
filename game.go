package abugame

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/zhms/abugo"
)

type GameCallback func()

type GameUserData struct {
	SellerId   int
	ChannelId  int
	UserId     int
	Amount     float64
	BankAmount float64
}

type UserData struct {
	BaseData       GameUserData
	Connection     int64
	ReconnectToken string
	HeartBeatCount int
}

type GameMsgCallback func(int, *map[string]interface{})
type GameUserComeCallback func(int)
type GameUserLeaveCallback func(int)
type GameServer struct {
	project           string
	module            string
	game_thread       chan GameCallback
	db                *abugo.AbuDb
	redis             *abugo.AbuRedis
	http              *abugo.AbuHttp
	msgcallbacks      map[string]GameMsgCallback
	usercomecallback  GameUserComeCallback
	userleavecallback GameUserLeaveCallback
	lock              sync.Mutex
	user_conn         sync.Map
	conn_user         sync.Map
	gameid            int
	roomlevel         int
	serverid          int
}

func (c *GameServer) Init(project string, module string, http *abugo.AbuHttp, db *abugo.AbuDb, redis *abugo.AbuRedis) {
	c.lock = sync.Mutex{}
	c.project = project
	c.module = module
	c.db = db
	c.redis = redis
	c.http = http
	c.gameid = viper.GetInt("server.gameid")
	c.roomlevel = viper.GetInt("server.roomlevel")
	c.serverid = viper.GetInt("server.serverid")
	c.game_thread = make(chan GameCallback, 100000)
	http.WsDefaultMsgCallback(c.default_msg_callback)
	http.WsAddCloseCallback(c.onwsclose)
	go c.heart_beat()
	go c.game_runner()
}

func (c *GameServer) game_invoke(callback GameCallback) {
	c.game_thread <- callback
}

func (c *GameServer) onwsclose(conn int64) {
	userdata, cbok := c.conn_user.Load(conn)
	if cbok {
		c.conn_user.Delete(conn)
		c.user_conn.Delete(userdata.(*UserData).BaseData.UserId)
		c.game_invoke(func() {
			if c.userleavecallback != nil {
				c.userleavecallback(userdata.(*UserData).BaseData.UserId)
			}
		})
	}
}

func (c *GameServer) default_msg_callback(conn int64, msgid string, data interface{}) {
	mapdata := data.(map[string]interface{})
	if msgid == `login` {
		token := abugo.GetMapString(&mapdata, "Token")
		rediskey := fmt.Sprintf("%s:hall:token:%s", c.project, token)
		redisdata := c.redis.Get(rediskey)
		if redisdata == nil {
			c.http.WsSendMsg(conn, "login", abugo.H{"errmsg": "????????????,token?????????"})
		} else {
			jdata := map[string]interface{}{}
			json.Unmarshal(redisdata.([]byte), &jdata)
			gameid := abugo.GetMapInt(&jdata, "GameId")
			roomlevel := abugo.GetMapInt(&jdata, "RoomLevel")
			serverid := abugo.GetMapInt(&jdata, "ServerId")
			if c.gameid != int(gameid) || c.roomlevel != int(roomlevel) || c.serverid != int(serverid) {
				c.http.WsSendMsg(conn, "login", abugo.H{"errmsg": "????????????,?????????????????????"})
			} else {
				UserId := abugo.GetMapInt(&jdata, "UserId")
				useridrediskey := fmt.Sprintf("%s:hall:user:data:%d", c.project, UserId)
				redisuserdata := c.redis.HGetAll(useridrediskey)
				userdata := UserData{}
				userdata.Connection = conn
				userdata.BaseData.SellerId = int(abugo.GetMapInt(redisuserdata, "SellerId"))
				userdata.BaseData.ChannelId = int(abugo.GetMapInt(redisuserdata, "ChannelId"))
				userdata.BaseData.UserId = int(UserId)
				userdata.BaseData.Amount = abugo.GetMapFloat64(redisuserdata, "Amount")
				userdata.BaseData.BankAmount = abugo.GetMapFloat64(redisuserdata, "BankAmount")
				userdata.ReconnectToken = abugo.AbuGuid()
				c.conn_user.Store(conn, &userdata)
				c.user_conn.Store(userdata.BaseData.UserId, &userdata)
				c.game_invoke(func() {
					if c.usercomecallback != nil {
						c.usercomecallback(userdata.BaseData.UserId)
					}
				})
			}
		}
	} else if msgid == "heartbeat" {
		value, ok := c.conn_user.Load(conn)
		if ok {
			v := value.(*UserData)
			v.HeartBeatCount = 0
		}
	}
}

func (c *GameServer) game_runner() {
	for {
		v, ok := <-c.game_thread
		if ok {
			v()
		}
	}
}

func (c *GameServer) heart_beat() {
	for {
		c.conn_user.Range(func(key, value any) bool {
			v := value.(*UserData)
			if v.HeartBeatCount >= 5 {
				http.WsClose(key.(int64))
				c.onwsclose(key.(int64))
			} else {
				v.HeartBeatCount++
				c.http.WsSendMsg(key.(int64), "heartbeat", abugo.H{"Index": v.HeartBeatCount})
			}
			return true
		})
		time.Sleep(time.Second * 2)
	}
}

func (c *GameServer) AddUserComeCallback(callback GameUserComeCallback) {
	c.lock.Lock()
	c.usercomecallback = callback
	c.lock.Unlock()
}

func (c *GameServer) AddUserLeaveCallback(callback GameUserLeaveCallback) {
	c.lock.Lock()
	c.userleavecallback = callback
	c.lock.Unlock()
}

func (c *GameServer) AddMsgCallback(msgid string, callback GameMsgCallback) {
	c.lock.Lock()
	c.msgcallbacks[msgid] = callback
	c.lock.Unlock()
}

func (c *GameServer) RemoveMsgCallback(msgid string) {
	c.lock.Lock()
	delete(c.msgcallbacks, msgid)
	c.lock.Unlock()
}

func (c *GameServer) SendMsgToUser(UserId int, data interface{}) {
	c.lock.Lock()
	c.lock.Unlock()
}

func (c *GameServer) SendMsgToAll(data interface{}) {
	c.lock.Lock()
	c.lock.Unlock()
}

func (c *GameServer) KickOutUser(UserId int) {
	value, ok := c.user_conn.Load(UserId)
	if ok {
		conn := value.(*UserData).Connection
		http.WsClose(conn)
		c.onwsclose(conn)
	}
}

func (c *GameServer) GetUserData(UserId int) *GameUserData {
	value, ok := c.user_conn.Load(UserId)
	if ok {
		return &value.(*UserData).BaseData
	} else {
		return nil
	}
}
