package dts

import (
	"context"
	"errors"
	"fmt"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk/model"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/source/device"
	"sync"
	"time"
)

const (
	CallAlarm  CallType = iota //接收报警回调
	CallTemp                   //接收温度更新回调
	CallSignal                 //接受光纤通道信号回调
	CallEvent                  //接受光纤事件回调
)

type CallType byte //回调类型

type Config struct {
	//DeviceId uint //设备 DeviceId
	Coordinate bool //是否启用防区坐标标签
	Relay      bool //是否启用防区继电器标签
	ChannelNum byte
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
	DTS    DTS

	CallTypes []CallType

	Cron    *cron.Cron
	CronIds map[byte]cron.EntryID

	status     device.StatusType
	ChanStatus chan device.StatusType

	ChanMessage       chan device.Message
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

func New(ctx context.Context, dts DTS, config Config) *App {
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
		Context:     ctx,
		cancel:      cancel,
		Config:      config,
		DTS:         dts,
		status:      device.UnConnect,
		ChanMessage: make(chan device.Message, 0),
		ChanStatus:  make(chan device.StatusType, 0),
		Zones:       map[uint]*Zone{},
		CronIds:     map[byte]cron.EntryID{},
		locker:      sync.Mutex{},
	}
}

func (a *App) GetId() string {
	return a.DTS.Host
}

func (a *App) GetType() device.Type {
	return device.TypeDTS
}

func (a *App) GetStatus() device.StatusType {
	return a.Status()
}

func (a *App) Run() error {
	status := a.GetStatus()
	if status == device.Connecting || status == device.Connected {
		return errors.New(fmt.Sprintf("设备 %s 已经正在运行中", a.DTS.Host))
	}
	if len(a.CallTypes) == 0 {
		a.CallTypes = []CallType{CallAlarm, CallTemp}
	}
	a.Client = dtssdk.NewDTSClient(a.DTS.Host)
	a.setStatus(device.Connecting)
	a.Client.CallConnected(func(s string) {
		a.setStatus(device.Connected)
		a.setMessage(fmt.Sprintf("主机为 %s 的dts连接成功", s), logrus.InfoLevel)
	})

	a.Client.OnTimeout(func(s string) {
		a.setMessage(fmt.Sprintf("主机为 %s 的dts连接超时", s), logrus.WarnLevel)
		a.setStatus(device.Connecting)
	})

	a.Client.CallDisconnected(func(s string) {
		a.setMessage(fmt.Sprintf("主机为 %s 的dts断开连接", s), logrus.WarnLevel)
		a.setStatus(device.Disconnect)
	})
	return nil
}

func (a *App) Register() (err error) {
	err = errors.New("")
	start := time.Now()
START:
	for err != nil {
		if time.Now().Sub(start) > time.Minute {
			a.setMessage(fmt.Sprintf("主机为 %s 的dts 回调数据失败", a.DTS.Host), logrus.ErrorLevel)
			break
		}
		for _, t := range a.CallTypes {
			switch t {
			case CallAlarm:
				a.ChanZonesAlarm = make(chan ZonesAlarm, 10)
				err = a.Client.CallZoneAlarmNotify(func(notify *model.ZoneAlarmNotify, err error) {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts 产生了一个警报...", a.DTS.Host), logrus.WarnLevel)
					zones := make([]*model.DefenceZone, 0)
					for _, zone := range notify.GetZones() {
						value, ok := a.ZonesTemp.LoadOrStore(fmt.Sprintf("%s-%d", notify.GetDeviceID(), Id(a.DTS.Id, uint(zone.ID))), notify.GetTimestamp())
						if ok && time.Now().Sub(time.Unix(value.(int64)/1000, 0)) < time.Second*time.Duration(a.Config.ZonesAlarmInterval) {
							return
						}
						a.ZonesTemp.Store(fmt.Sprintf("%s-%d", notify.GetDeviceID(), Id(a.DTS.Id, uint(zone.ID))), notify.GetTimestamp())
						zones = append(zones, zone)
					}
					length := len(zones)
					if length == 0 {
						a.setMessage(fmt.Sprintf("主机为 %s 的dts更新防区温度数量为空!", a.DTS.Host), logrus.ErrorLevel)
						return
					}
					if length > 10 {
						divide := length / 5
						if zones[0].AverageTemperature == 0 && zones[divide*1].AverageTemperature == 0 &&
							zones[divide*2].AverageTemperature == 0 && zones[divide*3].AverageTemperature == 0 &&
							zones[divide*4].AverageTemperature == 0 && zones[divide*5-1].AverageTemperature == 0 {
							a.setMessage(fmt.Sprintf("主机为 %s 的dts更新防区温度异常!", a.DTS.Host), logrus.ErrorLevel)
							return
						}
					}
					alarms := make(Zones, len(zones))
					for k, v := range zones {
						id := Id(a.DTS.Id, uint(v.GetID()))
						zone := a.GetZone(id)
						if zone == nil {
							a.setMessage(fmt.Sprintf("主机为 %s 的dts %d 防区未找到", a.DTS.Host, id), logrus.ErrorLevel)
							continue
						}
						alarms[k] = zone
						alarms[k].Temperature = &Temperature{
							Max: v.GetMaxTemperature(),
							Avg: v.GetAverageTemperature(),
							Min: v.GetMinTemperature(),
							At:  &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
						}
						alarms[k].Alarm = &Alarm{
							At:       &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
							Location: v.GetAlarmLoc(),
							State:    v.GetAlarmType(),
						}
					}
					select {
					case a.ChanZonesAlarm <- ZonesAlarm{
						DTS:       a.DTS,
						Zones:     alarms,
						Host:      a.DTS.Host,
						DeviceId:  notify.GetDeviceID(),
						CreatedAt: &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
					}:
					default:
					}
				})
				if err != nil {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts接受报警回调失败: %s", a.DTS.Host, err), logrus.ErrorLevel)
					time.Sleep(time.Second * 3)
					break START
				} else {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts接受报警回调", a.DTS.Host), logrus.InfoLevel)
				}
			case CallTemp:
				a.setMessage(fmt.Sprintf("主机为 %s 的dts 注册 接受温度回调", a.DTS.Host), logrus.InfoLevel)
				a.ChanZonesTemp = make(chan ZonesTemp, 30)
				err = a.Client.CallZoneTempNotify(func(notify *model.ZoneTempNotify, err error) {
					value, ok := a.ZonesTemp.LoadOrStore(notify.GetDeviceID(), notify)
					if ok && time.Now().Sub(time.Unix(value.(*model.ZoneTempNotify).GetTimestamp()/1000, 0)) < time.Second*time.Duration(a.Config.ZonesTempInterval) {
						return
					}
					a.ZonesTemp.Store(notify.GetDeviceID(), notify)
					zones := make(Zones, len(notify.GetZones()))
					index := 0
					for _, zone := range notify.GetZones() {
						if zone.GetMaxTemperature() == zone.GetMinTemperature() &&
							zone.GetAverageTemperature() == zone.GetMaxTemperature() &&
							zone.GetAverageTemperature() == 0 {
							continue
						}
						id := Id(a.DTS.Id, uint(zone.GetID()))
						z := a.GetZone(id)
						if z == nil {
							a.setMessage(fmt.Sprintf("主机为 %s 的dts %d 防区未找到", a.DTS.Host, id), logrus.ErrorLevel)
							continue
						}

						zones[index] = z
						zones[index].Temperature = &Temperature{
							Max: zone.GetMaxTemperature(),
							Avg: zone.GetAverageTemperature(),
							Min: zone.GetMinTemperature(),
							At:  &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
						}
						index++
					}
					zones = zones[:index]
					if len(zones) == 0 {
						a.setMessage(fmt.Sprintf("更新温度, 主机为 %s 的dts防区为空", a.DTS.Host), logrus.ErrorLevel)
						return
					}
					select {
					case a.ChanZonesTemp <- ZonesTemp{
						DTS:       a.DTS,
						Host:      a.DTS.Host,
						DeviceId:  notify.GetDeviceID(),
						CreatedAt: &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
						Zones:     zones,
					}:
					default:
					}
				})
				if err != nil {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts接受温度回调失败: %s", a.DTS.Host, err), logrus.ErrorLevel)
					break START
				} else {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts接受温度回调", a.DTS.Host), logrus.InfoLevel)
				}
			case CallSignal:
				a.ChanChannelSignal = make(chan ChannelSignal, 30)
				err = a.Client.CallTempSignalNotify(func(notify *model.TempSignalNotify, err error) {
					value, ok := a.ZonesChannelSignal.LoadOrStore(fmt.Sprintf("%s-%d", notify.GetDeviceID(), notify.ChannelID), notify)
					if ok && time.Now().Sub(time.Unix(value.(*model.TempSignalNotify).GetTimestamp()/1000, 0)) < time.Second*time.Duration(a.Config.ZonesSignInterval) {
						return
					}
					length := len(notify.GetSignal())
					if length == 0 {
						a.setMessage(fmt.Sprintf("主机为 %s 通道 %d 的dts信号数据为空!", a.DTS.Host, notify.GetChannelID()), logrus.ErrorLevel)
						return
					}
					if length > 10 {
						signal := notify.GetSignal()
						divide := length / 5
						if signal[0] == 0 && signal[divide*1] == 0 && signal[divide*2] == 0 && signal[divide*3] == 0 &&
							signal[divide*4] == 0 && signal[divide*5-1] == 0 {
							a.setMessage(fmt.Sprintf("主机为 %s 通道 %d 的dts信号异常!", a.DTS.Host, notify.GetChannelID()), logrus.ErrorLevel)
							return
						}
					}

					a.ZonesChannelSignal.Store(fmt.Sprintf("%s-%d", notify.GetDeviceID(), notify.ChannelID), notify)
					signal := ChannelSignal{
						DeviceId:   notify.GetDeviceID(),
						ChannelId:  notify.GetChannelID(),
						RealLength: notify.GetRealLength(),
						Host:       a.DTS.Host,
						Signal:     notify.GetSignal(),
						CreatedAt:  &TimeLocal{time.Unix(notify.GetTimestamp()/1000, 0)},
					}
					select {
					case a.ChanChannelSignal <- signal:
					default:
					}
				})
				if err != nil {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts接受信号回调失败: %s", a.DTS.Host, err), logrus.ErrorLevel)
					break START
				}
			case CallEvent:
				a.ChanChannelEvent = make(chan ChannelEvent, 10)
				err = a.Client.CallDeviceEventNotify(func(notify *model.DeviceEventNotify, err error) {
					event := ChannelEvent{
						DTS:           a.DTS,
						DeviceId:      notify.GetDeviceID(),
						ChannelId:     notify.GetChannelID(),
						Host:          a.DTS.Host,
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
					a.setMessage(fmt.Sprintf("主机为 %s 的dts接受信号回调失败: %s", a.DTS.Host, err), logrus.ErrorLevel)
					break START
				}
			}
		}
	}
	return
}

func (a *App) SetCron(cron *cron.Cron) {
	a.Cron = cron
}

func (a *App) Destroy() {
	select {
	case <-a.ChanStatus:
	default:
		if a.ChanStatus != nil {
			close(a.ChanStatus)
		}
	}
	select {
	case <-a.ChanMessage:
	default:
		if a.ChanMessage != nil {
			close(a.ChanMessage)
		}
	}
	select {
	case <-a.ChanZonesTemp:
	default:
		if a.ChanZonesTemp != nil {
			close(a.ChanZonesTemp)
		}
	}
	select {
	case <-a.ChanChannelEvent:
	default:
		if a.ChanChannelEvent != nil {
			close(a.ChanChannelEvent)
		}
	}
	select {
	case <-a.ChanChannelSignal:
	default:
		if a.ChanChannelSignal != nil {
			close(a.ChanChannelSignal)
		}
	}
	select {
	case <-a.ChanZonesAlarm:
	default:
		if a.ChanZonesAlarm != nil {
			close(a.ChanZonesAlarm)
		}
	}
}

// Close 关闭DTS
func (a *App) Close() error {
	a.cancel()
	status := a.GetStatus()
	if status == device.Connecting || status == device.Connected {
		a.Client.Close()
	}
	for _, id := range a.CronIds {
		a.Cron.Remove(id)
	}
	a.setMessage(fmt.Sprintf("主机为 %s 的dts断开连接", a.DTS.Host), logrus.WarnLevel)
	a.setStatus(device.Disconnect)
	time.AfterFunc(time.Second*3, a.Destroy)
	return nil
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

func (a *App) setMessage(msg string, level logrus.Level) {
	log.L.Log(level, msg)
	if a.ChanMessage == nil {
		return
	}
	select {
	case a.ChanMessage <- device.Message{
		Msg:   msg,
		Level: level,
		At:    device.TimeLocal{Time: time.Now()},
	}:
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
		id := Id(a.DTS.Id, uint(v.ID))
		zones[k] = new(Zone)
		zones[k].BaseZone = BaseZone{
			Id:        id,
			Name:      v.GetZoneName(),
			ChannelId: byte(v.GetChannelID()),
			Start:     v.GetStart(),
			Finish:    v.GetFinish(),
			Host:      a.DTS.Host,
		}
		if v.Tag == "" && (a.Config.Coordinate || a.Config.Relay) {
			log.L.Warn(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 标签为空", a.DTS.Host, channelId, v.ZoneName))
			continue
		}
		zones[k].Tag = DecodeTags(v.GetTag())
		if a.Config.Relay {
			relay, err := NewRelay(zones[k].Tag)
			if err != nil {
				continue
			}
			zones[k].Relay = relay
		}
		if a.Config.Coordinate {
			coordinate, err := NewCoordinate(zones[k].Tag)
			if err != nil {
				log.L.Error(fmt.Sprintf("获取主机 %s 通道 %d 防区 %s 坐标失败: %s", a.DTS.Host, channelId, v.ZoneName, err))
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
	log.L.Info(fmt.Sprintf("获取主机 %s 通道 %d 防区", a.DTS.Host, channelId))
	return nil
}

// SyncZones 同步获取所有防区
func (a *App) SyncZones() {
	for i := byte(1); i <= a.Config.ChannelNum; i++ {
		err := a.SyncChannelZones(i)
		if err != nil {
			a.setMessage(fmt.Sprintf("获取主机 %s 通道 %d 防区失败: %s", a.DTS.Host, i, err), logrus.ErrorLevel)
		}
	}
}
