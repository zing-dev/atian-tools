package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk/model"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/websocket"
	"github.com/kataras/neffos"
	"github.com/zing-dev/atian-tools/cfg"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/protocol/mqtt"
	socket "github.com/zing-dev/atian-tools/protocol/websocket"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type (
	Config struct {
		HttpPort int `comment:"HTTP端口,默认 8088"`
		DTS      struct {
			Host         string `comment:"DTS主机地址 例如 127.0.0.1"`
			ChannelNum   int    `comment:"DTS主机通道数量 例如 4"`
			TempInterval int    `comment:"DTS温度信号更新时间间隔单位秒,默认10秒"`
		}
		MQTT struct {
			Url      string `comment:"MQTT主机地址 例如 192.168.0.215:1883"`
			Username string `comment:"MQTT用户名,可以为空"`
			Password string `comment:"MQTT密码,可以为空"`
		}
	}

	ZoneAlarm struct {
		Line      int                    `json:"line"`
		Name      string                 `json:"name"`
		Temp      float32                `json:"temp"`
		Type      model.DefenceAreaState `json:"type"`
		Position  float32                `json:"position"`
		StartTime string                 `json:"start_time"`
	}

	ZoneAlarms []ZoneAlarm

	ZoneSign struct {
		Name  string    `json:"name"`
		Temp  []float32 `json:"temp"`
		Area  string    `json:"area"`
		Scale float32   `json:"scale"`
	}

	LineSign struct {
		Line  int        `json:"line"`
		Zones []ZoneSign `json:"zones"`
	}

	Request struct {
		Type  int   `json:"type"`
		Lines []int `json:"lines"`
	}

	Response struct {
		Type int         `json:"type"`
		Info interface{} `json:"info"`
	}

	Core struct {
		ctx    context.Context
		cancel context.CancelFunc
		config *Config
		Web    *iris.Application
		Socket *neffos.Server
		App    *dts.App
		MQTT   *mqtt.MQTT

		Zones        map[int]dts.Zones
		SignalNotify map[int]dts.ChannelSignal
		LineSigns    map[int]LineSign
	}
)

var (
	SectionName = "DongYing"
)

const (
	CodeTemp  = 1
	CodeAlarm = 2

	TopicTemp  = "temperature"
	TopicAlarm = "alarms"
)

func newConfig() *Config {
	config := new(Config)
	cfg.New().Register(func(c *cfg.Config) {
		section := c.File.Section(SectionName)
		if section.Comment == "" {
			section.Comment = fmt.Sprintf("项目名: %s", SectionName)
		}
		if len(section.Keys()) == 0 {
			config.HttpPort = 8088
			config.DTS.Host = "127.0.0.1"
			config.DTS.TempInterval = 10
			err := section.ReflectFrom(config)
			if err != nil {
				log.L.Fatal(fmt.Sprintf("%s 反射失败: %s", SectionName, err))
			}
			c.Save()
		}
		err := section.MapTo(config)
		if err != nil {
			log.L.Fatal(fmt.Sprintf("映射错误: %s", err))
		}
		if config.DTS.ChannelNum == 0 {
			log.L.Fatal("通道数不为空")
		}

		if config.MQTT.Url == "" {
			log.L.Fatal("MQTT主机地址非法")
		}
	}).Load()
	return config
}

func New() *Core {
	ctx, cancel := context.WithCancel(context.Background())
	config := newConfig()
	app := dts.New(ctx, dts.Config{Host: config.DTS.Host, ChannelNum: byte(config.DTS.ChannelNum), ZonesTempInterval: byte(config.DTS.TempInterval)})
	m := mqtt.New(ctx, mqtt.Config{Url: config.MQTT.Url, Username: config.MQTT.Username, Password: config.MQTT.Password})
	web := iris.New()
	core := &Core{
		ctx:          ctx,
		cancel:       cancel,
		config:       config,
		App:          app,
		MQTT:         m,
		Web:          web,
		Zones:        map[int]dts.Zones{},
		SignalNotify: map[int]dts.ChannelSignal{},
		LineSigns:    map[int]LineSign{},
	}
	core.Socket = neffos.New(websocket.DefaultGobwasUpgrader, websocket.Events{
		websocket.OnNativeMessage: func(conn *neffos.NSConn, msg neffos.Message) error {
			request := &Request{}
			err := json.Unmarshal(msg.Body, request)
			if err != nil {
				return nil
			}
			if request.Type == CodeTemp {
				core.HandleSignal(request)
				conn.Conn.Server().Broadcast(conn, msg)
			}
			return nil
		},
	})
	core.Socket.OnConnect = func(c *neffos.Conn) error {
		lines := make([]int, core.App.Config.ChannelNum)
		for i := byte(1); i <= core.App.Config.ChannelNum; i++ {
			lines[i-1] = int(i)
		}
		data, _ := json.Marshal(core.HandleSignal(&Request{Type: CodeTemp, Lines: lines}))
		c.Write(neffos.Message{Body: data, IsNative: true})
		return nil
	}
	core.Socket.OnDisconnect = func(c *neffos.Conn) {
	}
	return core
}
func main() {
	core := New()
	core.App.Run()
	core.MQTT.Run()
	core.Web.Get("/ws", websocket.Handler(core.Socket, websocket.DefaultIDGenerator))
	go func() {
		if err := core.Web.Run(iris.Addr(fmt.Sprintf("0.0.0.0:%d", core.config.HttpPort))); err != nil {
			log.L.Fatal(err)
		}
	}()
	go func() {
		for {
			select {
			case <-core.App.Context.Done():
			case status := <-core.App.ChanStatus:
				if status == dts.StatusOnline {
					for i := byte(1); i <= core.App.Config.ChannelNum; i++ {
						zones, err := core.App.GetSyncChannelZones(i)
						if err != nil {
							continue
						}
						core.Zones[int(i)] = zones
					}
				}
			case sign := <-core.App.ChanChannelSignal:
				log.L.Info(fmt.Sprintf("通道 %d 信号更新...", sign.ChannelId))
				lines := make([]int, core.App.Config.ChannelNum)
				for i := byte(1); i <= core.App.Config.ChannelNum; i++ {
					lines[i-1] = int(i)
				}
				core.SignalNotify[int(sign.ChannelId)] = sign
				data, _ := json.Marshal(core.HandleSignal(&Request{Type: CodeTemp, Lines: lines}))
				if core.MQTT.Client != nil && core.MQTT.Client.IsConnected() {
					if token := core.MQTT.Client.Publish(TopicTemp, 1, false, data); token.Wait() && token.Error() != nil {
						log.L.Error("mqtt发布温度失败: ", token.Error())
					}
				}
			case alarms := <-core.App.ChanZonesAlarm:
				log.L.Info(fmt.Sprintf("新的报警产生..."))
				var info = make(ZoneAlarms, len(alarms.Zones))
				for k, v := range alarms.Zones {
					info[k] = ZoneAlarm{
						Line:      int(v.ChannelId),
						Name:      v.Name,
						Temp:      Decimal(v.Avg),
						Position:  v.Location,
						Type:      v.AlarmType,
						StartTime: v.AlarmAt.String(),
					}
				}
				var response = &Response{Type: CodeAlarm, Info: info}
				data, _ := json.Marshal(response)
				if core.MQTT.Client != nil && core.MQTT.Client.IsConnected() {
					if token := core.MQTT.Client.Publish(TopicAlarm, 1, false, data); token.Wait() && token.Error() != nil {
						log.L.Error("mqtt发布报警失败: ", token.Error())
					} else {
						log.L.Info("mqtt发布报警成功")
					}
				}
				socket.Send(data, core.Socket)
			}
		}
	}()

	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGHUP, syscall.SIGABRT, syscall.SIGINT)
	<-stop
	core.cancel()
	time.Sleep(time.Second)
}

func (c *Core) HandleSignal(r *Request) *Response {
	var LineSigns = make([]*LineSign, len(r.Lines))
	for k, line := range r.Lines {
		notify := c.SignalNotify[line]
		if len(notify.Signal) == 0 {
			//return &Response{
			//	Type: CodeTemp,
			//	Info: "防区信号为空",
			//}
			continue
		}
		zones := c.Zones[line]
		if len(zones) == 0 {
			return &Response{
				Type: CodeTemp,
				Info: fmt.Sprintf("通道 %d 防区为空", line),
			}
		}

		var zs = make([]ZoneSign, len(zones))

		for k := range notify.Signal {
			notify.Signal[k] = Decimal(notify.Signal[k])
		}
		for k, zone := range zones {
			sign, err := dts.ZoneMapSign(notify.Signal, zone.Start, zone.Finish, notify.RealLength)
			if err != nil {
				continue
			}
			zs[k] = ZoneSign{
				Name:  zone.Name,
				Temp:  sign,
				Area:  fmt.Sprintf("%.3f~%.3f", zone.Start, zone.Finish),
				Scale: notify.RealLength,
			}
		}
		LineSigns[k] = &LineSign{
			Line:  line,
			Zones: zs,
		}
	}
	return &Response{
		Type: CodeTemp,
		Info: LineSigns,
	}
}

func Decimal(value float32) float32 {
	return float32(math.Trunc(float64(value*1e1+0.5)) / 1e1)
}
