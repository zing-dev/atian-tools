package dts

import (
	"atian.tools/log"
	"context"
	"fmt"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk/model"
	"regexp"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	EnableWarehouse bool
	EnableRelay     bool
	ChannelNum      byte
	Host            string
}

type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	Client *dtssdk.Client
	config Config

	alarms chan []ZoneAlarm
	Zones  map[uint]*Zone

	locker *sync.RWMutex
}

func New(ctx context.Context, config Config) *App {
	ctx, cancel := context.WithCancel(ctx)
	return &App{
		ctx:    ctx,
		cancel: cancel,
		config: config,
		locker: new(sync.RWMutex),
	}
}

func (a *App) Run() {
	a.Client = dtssdk.NewDTSClient(a.config.Host)
	a.Client.CallConnected(func(s string) {
		log.L.Info(fmt.Sprintf("主机为 %s 的dts连接成功", s))
		go a.GetZones()
		err := a.Client.CallZoneAlarmNotify(func(notify *model.ZoneAlarmNotify, err error) {
			alarms := make([]ZoneAlarm, len(notify.GetZones()))
			for k, v := range notify.GetZones() {
				zone := a.GetZone(uint(v.ID))
				if zone != nil {
					log.L.Error()
					continue
				}
				alarms[k] = ZoneAlarm{
					Zone: zone,
					Temperature: Temperature{
						Max: v.MaxTemperature,
						Avg: v.AverageTemperature,
						Min: v.MinTemperature,
					},
					Location:  v.AlarmLoc,
					AlarmAt:   TimeLocal{time.Unix(notify.Timestamp, 0)},
					AlarmType: v.AlarmType,
				}
			}
			select {
			case a.alarms <- alarms:
			default:
			}
		})
		if err != nil {
			log.L.Error(fmt.Sprintf("主机为 %s 的dts接受报警回调失败: %s", s, err))
		}

	})
}

func (a *App) GetZone(id uint) *Zone {
	a.locker.RLocker()
	defer a.locker.RUnlock()
	return a.Zones[id]
}

func (a *App) GetZones() {
	for i := byte(1); i < a.config.ChannelNum; i++ {
		response, err := a.Client.GetDefenceZone(int(i), "")
		if err != nil {
			log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区失败: %s", a.config.Host, i, err))
			continue
		}
		if !response.Success {
			log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区响应失败: %s", a.config.Host, i, response.ErrMsg))
			continue
		}
		a.locker.Lock()
		for _, v := range response.Rows {
			v := v
			id := uint(v.ID)
			a.Zones[uint(v.ID)] = &Zone{
				Id:        id,
				Name:      v.ZoneName,
				ChannelId: byte(v.ChannelID),
				Tag:       DecodeTags(v.Tag),
				Start:     v.Start,
				Finish:    v.Finish,
				Host:      a.config.Host,
			}

			if a.config.EnableRelay {
				//relay:A1,2,3,4,5
				r := a.Zones[id].Tag[TagRelay]
				if len(r) < 2 {
					log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 继电器标签字符值至少两位,例如A1", a.config.Host, v.ZoneName, i))
				} else if ok, err := regexp.MatchString("^([1-9]*[1-9][0-9]*,)+[1-9]*[1-9][0-9]*$", r[1:]); !ok {
					log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 继电器标签模式不匹配: %s, 必须如A1,2,3,4", a.config.Host, i, v.ZoneName, err))
				} else {
					a.Zones[id].Relay = Relay{r[0]: r[1:]}
				}
			}
			if a.config.EnableWarehouse {
				var (
					row, column, layer = 0, 0, 0
					err                error
				)
				row, err = strconv.Atoi(a.Zones[id].Tag[TagRow])
				if err != nil {
					log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 行失败: %s", a.config.Host, i, v.ZoneName, err))
					continue
				}
				column, err = strconv.Atoi(a.Zones[id].Tag[TagColumn])
				if err != nil {
					log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 列失败: %s", a.config.Host, i, v.ZoneName, err))
					continue
				}
				layer, err = strconv.Atoi(a.Zones[id].Tag[TagLayer])
				if err != nil {
					log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 层失败: %s", a.config.Host, i, v.ZoneName, err))
					continue
				}
				a.Zones[id].ZoneExtend = ZoneExtend{
					Warehouse: a.Zones[id].Tag[TagWarehouse],
					Group:     a.Zones[id].Tag[TagGroup],
					Row:       row,
					Column:    column,
					Layer:     layer,
				}
			}
		}
		a.locker.Unlock()
	}
}

func (a *App) RecAlarm() []ZoneAlarm {
	return <-a.alarms
}
