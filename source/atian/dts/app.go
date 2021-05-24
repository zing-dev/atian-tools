package dts

import (
	"context"
	"errors"
	"fmt"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk/model"
	"github.com/zing-dev/atian-tools/log"
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
	Context context.Context
	Cancel  context.CancelFunc

	Client *dtssdk.Client
	Config Config

	status     Status
	ChanStatus chan Status

	ChanZonesTemp     chan ZonesTemp
	ChanChannelSignal chan ChannelSignal
	ChanChannelEvent  chan ChannelEvent
	ChanZonesAlarm    chan ZonesAlarm
	Zones             map[uint]*Zone

	locker sync.Mutex
}

func New(ctx context.Context, config Config) *App {
	ctx, cancel := context.WithCancel(ctx)
	return &App{
		Context:           ctx,
		Cancel:            cancel,
		Config:            config,
		ChanStatus:        make(chan Status, 0),
		ChanZonesTemp:     make(chan ZonesTemp, 10),
		ChanChannelSignal: make(chan ChannelSignal, 10),
		ChanChannelEvent:  make(chan ChannelEvent, 10),
		ChanZonesAlarm:    make(chan ZonesAlarm, 10),
		Zones:             map[uint]*Zone{},
		locker:            sync.Mutex{},
	}
}

func (a *App) Run() {
	a.Client = dtssdk.NewDTSClient(a.Config.Host)
	a.Client.CallConnected(func(s string) {
		log.L.Info(fmt.Sprintf("主机为 %s 的dts连接成功", s))
		a.setStatus(StatusOnline)
		go a.GetSyncZones()
		a.call()
	})

	a.Client.OnTimeout(func(s string) {
		log.L.Warn(fmt.Sprintf("主机为 %s 的dts连接超时", s))
		a.setStatus(StatusOff)
	})

	a.Client.CallDisconnected(func(s string) {
		log.L.Warn(fmt.Sprintf("主机为 %s 的dts断开连接", s))
		a.setStatus(StatusOff)
	})
}

func (a *App) call() {
	var err = errors.New("")
	for err != nil {
		err = a.Client.CallZoneTempNotify(func(notify *model.ZoneTempNotify, err error) {
			zones := make([]ZoneTemp, len(notify.GetZones()))
			for i, zone := range notify.GetZones() {
				zones[i] = ZoneTemp{
					Zone: a.GetZone(uint(zone.ID)),
					Temperature: &Temperature{
						Max: zone.GetMaxTemperature(),
						Avg: zone.GetAverageTemperature(),
						Min: zone.GetMinTemperature(),
					},
				}
			}
			select {
			case a.ChanZonesTemp <- ZonesTemp{
				DeviceId:  notify.GetDeviceID(),
				Host:      a.Config.Host,
				CreatedAt: TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
				Zones:     zones,
			}:
			default:
			}
		})
		if err != nil {
			log.L.Error(fmt.Sprintf("主机为 %s 的dts接受温度回调失败: %s", a.Config.Host, err))
			time.Sleep(time.Second / 10)
			continue
		}

		err = a.Client.CallTempSignalNotify(func(notify *model.TempSignalNotify, err error) {
			signal := ChannelSignal{
				DeviceId:   notify.GetDeviceID(),
				ChannelId:  notify.GetChannelID(),
				RealLength: notify.GetRealLength(),
				Host:       a.Config.Host,
				Signal:     notify.GetSignal(),
				CreatedAt:  &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
			}
			select {
			case a.ChanChannelSignal <- signal:
			default:
			}
		})
		if err != nil {
			log.L.Error(fmt.Sprintf("主机为 %s 的dts接受信号回调失败: %s", a.Config.Host, err))
			time.Sleep(time.Second / 10)
			continue
		}

		err = a.Client.CallZoneAlarmNotify(func(notify *model.ZoneAlarmNotify, err error) {
			log.L.Warn(fmt.Sprintf("主机为 %s 的dts 产生了一个警报...", a.Config.Host))
			alarms := make([]ZoneAlarm, len(notify.GetZones()))
			for k, v := range notify.GetZones() {
				zone := a.GetZone(uint(v.GetID()))
				if zone == nil {
					log.L.Error("没有找防区: ", v.GetID())
					continue
				}
				alarms[k] = ZoneAlarm{
					Zone: zone,
					Temperature: &Temperature{
						Max: v.GetMaxTemperature(),
						Avg: v.GetAverageTemperature(),
						Min: v.GetMinTemperature(),
					},
					Location:  v.GetAlarmLoc(),
					AlarmType: v.GetAlarmType(),
				}
			}
			select {
			case a.ChanZonesAlarm <- ZonesAlarm{
				Zones:     alarms,
				DeviceId:  notify.GetDeviceID(),
				Host:      a.Config.Host,
				CreatedAt: &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
			}:
			default:
			}
		})
		if err != nil {
			log.L.Error(fmt.Sprintf("主机为 %s 的dts接受报警回调失败: %s", a.Config.Host, err))
			time.Sleep(time.Second / 10)
			continue
		}

		err = a.Client.CallDeviceEventNotify(func(notify *model.DeviceEventNotify, err error) {
			event := ChannelEvent{
				DeviceId:      notify.GetDeviceID(),
				ChannelId:     notify.GetChannelID(),
				Host:          a.Config.Host,
				EventType:     notify.GetEventType(),
				ChannelLength: notify.GetChannelLength(),
				CreatedAt:     &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
			}
			select {
			case a.ChanChannelEvent <- event:
			default:
			}
		})
		if err != nil {
			log.L.Error(fmt.Sprintf("主机为 %s 的dts接受信号回调失败: %s", a.Config.Host, err))
			time.Sleep(time.Second / 10)
			continue
		}
	}
}

func (a *App) Status() Status {
	a.locker.Lock()
	defer a.locker.Unlock()
	return a.status
}

func (a *App) setStatus(s Status) {
	a.locker.Lock()
	a.status = s
	a.locker.Unlock()
	select {
	case a.ChanStatus <- s:
	default:
	}
}

func (a *App) GetZone(id uint) *Zone {
	return a.GetZones()[id]
}

func (a *App) GetZones() map[uint]*Zone {
	a.locker.Lock()
	defer a.locker.Unlock()
	return a.Zones
}

// GetSyncChannelZones 同步获取通道防区
func (a *App) GetSyncChannelZones(channelId byte) error {
	response, err := a.Client.GetDefenceZone(int(channelId), "")
	if err != nil {
		return err
	}
	if !response.Success {
		return errors.New(response.ErrMsg)
	}
	a.locker.Lock()
	for k := range response.Rows {
		v := response.Rows[k]
		id := uint(v.ID)
		a.Zones[id] = &Zone{
			Id:        id,
			Name:      v.ZoneName,
			ChannelId: byte(v.ChannelID),
			Start:     v.Start,
			Finish:    v.Finish,
			Host:      a.Config.Host,
		}
		if v.Tag != "" {
			a.Zones[id].Tag = DecodeTags(v.Tag)
		}
		if a.Config.EnableRelay {
			//relay:A1,2,3,4,5
			r, ok := a.Zones[id].Tag[TagRelay]
			if !ok {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 继电器标签不存在", a.Config.Host, channelId, v.ZoneName))
			} else if len(r) < 2 {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 继电器标签字符值至少两位,例如A1", a.Config.Host, channelId, v.ZoneName))
			} else if ok, err := regexp.MatchString("^([1-9]*[1-9][0-9]*,)+[1-9]*[1-9][0-9]*$", r[1:]); !ok {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 继电器标签模式不匹配: %s, 必须如A1,2,3,4", a.Config.Host, channelId, v.ZoneName, err))
			} else {
				a.Zones[id].Relay = Relay{r[0]: r[1:]}
			}
		}
		if a.Config.EnableWarehouse {
			var (
				row, column, layer = 0, 0, 0
				err                error
			)
			row, err = strconv.Atoi(a.Zones[id].Tag[TagRow])
			if err != nil {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 行失败: %s", a.Config.Host, channelId, v.ZoneName, err))
				continue
			}
			column, err = strconv.Atoi(a.Zones[id].Tag[TagColumn])
			if err != nil {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 列失败: %s", a.Config.Host, channelId, v.ZoneName, err))
				continue
			}
			layer, err = strconv.Atoi(a.Zones[id].Tag[TagLayer])
			if err != nil {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 层失败: %s", a.Config.Host, channelId, v.ZoneName, err))
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
	log.L.Info(fmt.Sprintf("获取主机 %s 通道 %d 防区", a.Config.Host, channelId))
	return nil
}

// GetSyncZones 同步获取所有防区
func (a *App) GetSyncZones() {
	for i := byte(1); i <= a.Config.ChannelNum; i++ {
		err := a.GetSyncChannelZones(i)
		if err != nil {
			log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区失败: %s", a.Config.Host, i, err))
		}
	}
}
