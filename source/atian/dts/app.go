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
	"sync"
	"time"
)

type Config struct {
	DeviceId        uint
	EnableWarehouse bool
	EnableRelay     bool
	ChannelNum      byte
	Host            string

	//ZonesAlarmInterval 防区温度间隔秒
	ZonesAlarmInterval byte
	//ZonesTempInterval 报警温度间隔秒
	ZonesTempInterval uint16
	ZonesSignInterval uint16
}

type App struct {
	Context context.Context
	cancel  context.CancelFunc

	Client *dtssdk.Client
	Config Config

	Cron       *cron.Cron
	CronIds    map[byte]cron.EntryID
	status     device.StatusType
	ChanStatus chan device.StatusType

	ChanZonesTemp     chan ZonesTemp
	ChanChannelSignal chan ChannelSignal
	ChanChannelEvent  chan ChannelEvent
	ChanZonesAlarm    chan ZonesAlarm
	Zones             map[uint]*Zone

	ZonesChannelSignal sync.Map
	ZonesTemp          sync.Map
	ZonesAlarm         sync.Map
	locker             sync.Mutex
}

func New(ctx context.Context, config Config) *App {
	if config.ZonesAlarmInterval <= 0 {
		config.ZonesAlarmInterval = 20
	}

	if config.ZonesTempInterval <= 0 {
		config.ZonesTempInterval = 60
	}
	if config.ZonesSignInterval <= 0 {
		config.ZonesSignInterval = 60
	}

	ctx, cancel := context.WithCancel(ctx)
	return &App{
		Context:           ctx,
		cancel:            cancel,
		Config:            config,
		ChanStatus:        make(chan device.StatusType, 0),
		ChanZonesTemp:     make(chan ZonesTemp, 30),
		ChanChannelSignal: make(chan ChannelSignal, 10),
		ChanChannelEvent:  make(chan ChannelEvent, 10),
		ChanZonesAlarm:    make(chan ZonesAlarm, 30),
		Zones:             map[uint]*Zone{},
		CronIds:           map[byte]cron.EntryID{},
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
	return a.Status()
}

func (a *App) Run() {
	a.Client = dtssdk.NewDTSClient(a.Config.Host)
	a.setStatus(device.Disconnect)
	a.Client.CallConnected(func(s string) {
		log.L.Info(fmt.Sprintf("主机为 %s 的dts连接成功", s))
		a.SyncZones()
		a.setStatus(device.Connected)
		a.call()
	})

	a.Client.OnTimeout(func(s string) {
		log.L.Warn(fmt.Sprintf("主机为 %s 的dts连接超时", s))
		a.setStatus(device.Connecting)
	})

	a.Client.CallDisconnected(func(s string) {
		log.L.Warn(fmt.Sprintf("主机为 %s 的dts断开连接", s))
		a.setStatus(device.Disconnect)
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
				id := Id(a.Config.DeviceId, uint(zone.GetID()))
				zones[i] = a.GetZone(id)
				if zones[i] == nil {
					log.L.Error(fmt.Sprintf("主机为 %s 的dts %d 防区未找到", a.Config.Host, id))
					continue
				}
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
				CreatedAt: &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
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
			value, ok := a.ZonesChannelSignal.LoadOrStore(fmt.Sprintf("%s-%d", notify.GetDeviceID(), notify.ChannelID), notify)
			if ok && time.Now().Sub(time.Unix(value.(*model.TempSignalNotify).GetTimestamp()/1000, 0)) < time.Second*time.Duration(a.Config.ZonesSignInterval) {
				return
			}
			length := len(notify.GetSignal())
			if length == 0 {
				log.L.Error(fmt.Sprintf("主机为 %s 通道 %d 的dts信号数据为空!", a.Config.Host, notify.GetChannelID()))
				return
			}
			if length > 10 {
				signal := notify.GetSignal()
				divide := length / 5
				if signal[0] == 0 && signal[divide*1] == 0 && signal[divide*2] == 0 && signal[divide*3] == 0 &&
					signal[divide*4] == 0 && signal[divide*5-1] == 0 {
					log.L.Error(fmt.Sprintf("主机为 %s 通道 %d 的dts信号异常!", a.Config.Host, notify.GetChannelID()))
					return
				}
			}

			a.ZonesChannelSignal.Store(fmt.Sprintf("%s-%d", notify.GetDeviceID(), notify.ChannelID), notify)
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
				value, ok := a.ZonesTemp.LoadOrStore(fmt.Sprintf("%s-%d", notify.GetDeviceID(), Id(a.Config.DeviceId, uint(zone.ID))), notify.GetTimestamp())
				if ok && time.Now().Sub(time.Unix(value.(int64)/1000, 0)) < time.Second*time.Duration(a.Config.ZonesAlarmInterval) {
					return
				}
				a.ZonesTemp.Store(fmt.Sprintf("%s-%d", notify.GetDeviceID(), Id(a.Config.DeviceId, uint(zone.ID))), notify.GetTimestamp())
				zones = append(zones, zone)
			}
			length := len(zones)
			if length == 0 {
				log.L.Error(fmt.Sprintf("主机为 %s 的dts更新防区温度数量为空!", a.Config.Host))
				return
			}
			if length > 10 {
				divide := length / 5
				if zones[0].AverageTemperature == 0 && zones[divide*1].AverageTemperature == 0 &&
					zones[divide*2].AverageTemperature == 0 && zones[divide*3].AverageTemperature == 0 &&
					zones[divide*4].AverageTemperature == 0 && zones[divide*5-1].AverageTemperature == 0 {
					log.L.Error(fmt.Sprintf("主机为 %s 的dts更新防区温度异常!", a.Config.Host))
					return
				}
			}
			alarms := make(Zones, len(zones))
			for k, v := range zones {
				id := Id(a.Config.DeviceId, uint(v.GetID()))
				zone := a.GetZone(id)
				if zone == nil {
					log.L.Error(fmt.Sprintf("主机为 %s 的dts %d 防区未找到", a.Config.Host, id))
					continue
				}
				alarms[k] = zone
				alarms[k].Temperature = &Temperature{
					Max: v.GetMaxTemperature(),
					Avg: v.GetAverageTemperature(),
					Min: v.GetMinTemperature(),
				}
				alarms[k].Alarm = &Alarm{
					At:       &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
					Location: v.GetAlarmLoc(),
					State:    v.GetAlarmType(),
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

func (a *App) SetCron(cron *cron.Cron) {
	a.Cron = cron
}

func (a *App) Close() {
	for _, id := range a.CronIds {
		a.Cron.Remove(id)
	}
	a.cancel()
}

func (a *App) Status() device.StatusType {
	a.locker.Lock()
	defer a.locker.Unlock()
	return a.status
}

func (a *App) setStatus(s device.StatusType) {
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
		id := Id(a.Config.DeviceId, uint(v.ID))
		zones[k] = new(Zone)
		zones[k].BaseZone = BaseZone{
			Id:        id,
			Name:      v.GetZoneName(),
			ChannelId: byte(v.GetChannelID()),
			Start:     v.GetStart(),
			Finish:    v.GetFinish(),
			Host:      a.Config.Host,
		}
		if v.Tag == "" && (a.Config.EnableRelay || a.Config.EnableWarehouse) {
			log.L.Warn(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 标签为空", a.Config.Host, channelId, v.ZoneName))
			continue
		}
		zones[k].Tag = DecodeTags(v.GetTag())
		if a.Config.EnableRelay {
			relay, err := NewRelay(zones[k].Tag)
			if err != nil {
				continue
			}
			zones[k].Relay = relay
		}
		if a.Config.EnableWarehouse {
			coordinate, err := NewCoordinate(zones[k].Tag)
			if err != nil {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 坐标失败: %s", a.Config.Host, channelId, v.ZoneName, err))
				continue
			}
			zones[k].Coordinate = coordinate
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
