package dts

import (
	"context"
	"errors"
	"fmt"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk/model"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/source/device"
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

	//ZonesAlarmInterval 防区温度间隔秒
	ZonesAlarmInterval byte
	//ZonesTempInterval 报警温度间隔秒
	ZonesTempInterval byte
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

	ZonesTemp  sync.Map
	ZonesAlarm sync.Map
	locker     sync.Mutex
}

func New(ctx context.Context, config Config) *App {
	if config.ZonesAlarmInterval <= 0 {
		config.ZonesAlarmInterval = 10
	}

	if config.ZonesTempInterval <= 0 {
		config.ZonesTempInterval = 30
	}

	ctx, cancel := context.WithCancel(ctx)
	return &App{
		Context:           ctx,
		Cancel:            cancel,
		Config:            config,
		ChanStatus:        make(chan Status, 0),
		ChanZonesTemp:     make(chan ZonesTemp, 30),
		ChanChannelSignal: make(chan ChannelSignal, 10),
		ChanChannelEvent:  make(chan ChannelEvent, 10),
		ChanZonesAlarm:    make(chan ZonesAlarm, 30),
		Zones:             map[uint]*Zone{},
		locker:            sync.Mutex{},
	}
}

func (a *App) GetId() string {
	return a.Config.Host
}

func (a *App) GetType() device.Type {
	return device.TypeDTS
}

func (a *App) GetStatus() device.StatusType {
	return device.StatusType(a.Status())
}

func (a *App) Cron(*cron.Cron) {}

func (a *App) Run() {
	a.Client = dtssdk.NewDTSClient(a.Config.Host)
	a.Client.CallConnected(func(s string) {
		log.L.Info(fmt.Sprintf("主机为 %s 的dts连接成功", s))
		a.setStatus(StatusOnline)
		go a.SyncZones()
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
			value, ok := a.ZonesTemp.LoadOrStore(notify.GetDeviceID(), notify)
			if ok && time.Now().Sub(time.Unix(value.(*model.ZoneTempNotify).GetTimestamp()/1000, 0)) < time.Second*time.Duration(a.Config.ZonesTempInterval) {
				return
			}
			a.ZonesTemp.Store(notify.GetDeviceID(), notify)
			zones := make(Zones, len(notify.GetZones()))
			for i, zone := range notify.GetZones() {
				zones[i] = a.GetZone(uint(zone.ID))
				zones[i].Temperature = &Temperature{
					Max: zone.GetMaxTemperature(),
					Avg: zone.GetAverageTemperature(),
					Min: zone.GetMinTemperature(),
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
			zones := make([]*model.DefenceZone, 0)
			for _, zone := range notify.GetZones() {
				value, ok := a.ZonesTemp.LoadOrStore(fmt.Sprintf("%s-%d", notify.GetDeviceID(), zone.ID), notify.GetTimestamp())
				if ok && time.Now().Sub(time.Unix(value.(int64)/1000, 0)) < time.Second*time.Duration(a.Config.ZonesAlarmInterval) {
					return
				}
				a.ZonesTemp.Store(fmt.Sprintf("%s-%d", notify.GetDeviceID(), zone.ID), notify.GetTimestamp())
				zones = append(zones, zone)
			}
			if len(zones) == 0 {
				return
			}
			alarms := make(Zones, len(zones))
			for k, v := range zones {
				zone := a.GetZone(uint(v.GetID()))
				if zone == nil {
					log.L.Error("没有找防区: ", v.GetID())
					continue
				}
				alarms[k] = zone
				alarms[k].Temperature = &Temperature{
					Max: v.GetMaxTemperature(),
					Avg: v.GetAverageTemperature(),
					Min: v.GetMinTemperature(),
				}
				alarms[k].Alarm = &Alarm{
					AlarmAt:   &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
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

func (a *App) Close() {
	a.Cancel()
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

func (a *App) GetSyncChannelZones(channelId byte) (Zones, error) {
	response, err := a.Client.GetDefenceZone(int(channelId), "")
	if err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, errors.New(response.ErrMsg)
	}
	zones := make(Zones, len(response.Rows))
	for k := range response.Rows {
		v := response.Rows[k]
		id := uint(v.ID)
		zones[k] = new(Zone)
		zones[k].BaseZone = BaseZone{
			Id:        id,
			Name:      v.ZoneName,
			ChannelId: byte(v.ChannelID),
			Start:     v.Start,
			Finish:    v.Finish,
			Host:      a.Config.Host,
		}
		if v.Tag != "" {
			zones[k].Tag = DecodeTags(v.Tag)
		}
		if a.Config.EnableRelay {
			r, ok := zones[k].Tag[TagRelay]
			if !ok {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 继电器标签不存在", a.Config.Host, channelId, v.ZoneName))
			} else if len(r) < 2 {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 继电器标签字符值至少两位,例如A1", a.Config.Host, channelId, v.ZoneName))
			} else if ok, err := regexp.MatchString("^([1-9]*[1-9][0-9]*,)+[1-9]*[1-9][0-9]*$", r[1:]); !ok {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 继电器标签模式不匹配: %s, 必须如A1,2,3,4", a.Config.Host, channelId, v.ZoneName, err))
			} else {
				zones[k].Relay = Relay{r[0]: r[1:]}
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
			zones[k].ZoneExtend = &ZoneExtend{
				Warehouse: zones[k].Tag[TagWarehouse],
				Group:     zones[k].Tag[TagGroup],
				Row:       row,
				Column:    column,
				Layer:     layer,
			}
		}
	}
	return zones, nil
}

// SyncChannelZones 同步获取通道防区
func (a *App) SyncChannelZones(channelId byte) error {
	zones, err := a.GetSyncChannelZones(channelId)
	if err != nil {
		return err
	}
	a.locker.Lock()
	for _, zone := range zones {
		a.Zones[zone.Id] = zone
	}
	a.locker.Unlock()
	log.L.Info(fmt.Sprintf("获取主机 %s 通道 %d 防区", a.Config.Host, channelId))
	return nil
}

// SyncZones 同步获取所有防区
func (a *App) SyncZones() {
	for i := byte(1); i <= a.Config.ChannelNum; i++ {
		err := a.SyncChannelZones(i)
		if err != nil {
			log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区失败: %s", a.Config.Host, i, err))
		}
	}
}
