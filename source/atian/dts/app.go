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
	CallAlarm  CallType = iota //注册报警回调
	CallTemp                   //注册温度更新回调
	CallSignal                 //注册光纤通道信号回调
	CallEvent                  //注册光纤事件回调
)

type CallType byte //回调类型

// Config DTS设备的配置
type Config struct {
	Coordinate    bool   //是否启用防区坐标标签
	Relay         bool   //是否启用防区继电器标签
	ChannelNum    byte   //通道数
	ZonesAlarmSec byte   //防区温度间隔 秒
	ZonesTempSec  uint16 //报警温度间隔 秒
	ChanSignSec   uint16 //通道信号间隔 秒
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
	if config.ZonesAlarmSec <= 0 {
		config.ZonesAlarmSec = 20
	}
	if config.ZonesTempSec <= 0 {
		config.ZonesTempSec = 60
	}
	if config.ChanSignSec <= 0 {
		config.ChanSignSec = 60
	}
	if config.ChannelNum == 0 {
		config.ChannelNum = 4
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

// Register 在连接成功后回调该函数!!!
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
			case CallAlarm: //防区报警
				a.ChanZonesAlarm = make(chan ZonesAlarm, 10)
				err = a.Client.CallZoneAlarmNotify(func(notify *model.ZoneAlarmNotify, err error) {
					var (
						index = 0                                                  //报警防区索引,最后一个索引即为当前报警防区的长度
						zones = make([]*model.DefenceZone, len(notify.GetZones())) //实际可用的报警防区
					)
					for _, zone := range notify.GetZones() {
						//若当前报警防区已存在缓存中,且当前报警时间与上一个报警时间的间隔小于报警阈值,则不处理
						value, ok := a.ZonesTemp.LoadOrStore(fmt.Sprintf("%s-%d", notify.GetDeviceID(), Id(a.DTS.Id, uint(zone.ID))), notify.GetTimestamp())
						if ok && time.Now().Sub(time.Unix(value.(int64)/1000, 0)) < time.Second*time.Duration(a.Config.ZonesAlarmSec) {
							return
						}
						//缓存当前报警防区
						a.ZonesTemp.Store(fmt.Sprintf("%s-%d", notify.GetDeviceID(), Id(a.DTS.Id, uint(zone.ID))), notify.GetTimestamp())
						zones[index] = zone
						index++ // 可用,索引递增
					}

					//当索引为0时则表明无可用的报警防区
					if index == 0 {
						a.setMessage(fmt.Sprintf("主机为 %s 的dts更新防区温度数量为空!", a.DTS.Host), logrus.ErrorLevel)
						return
					}
					//当防区数量大于10时,进行防区的温度校验
					if index > 10 {
						divide := index / 5
						if zones[0].AverageTemperature == 0 && zones[divide*1].AverageTemperature == 0 &&
							zones[divide*2].AverageTemperature == 0 && zones[divide*3].AverageTemperature == 0 &&
							zones[divide*4].AverageTemperature == 0 && zones[divide*5-1].AverageTemperature == 0 {
							a.setMessage(fmt.Sprintf("主机为 %s 的dts更新防区温度异常!", a.DTS.Host), logrus.ErrorLevel)
							return
						}
					}

					//获取可用的防区
					zones = zones[:index]

					//对于可用的报警防区进行重新组装
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
							At:  &device.TimeLocal{Time: time.Unix(notify.GetTimestamp()/1000, 0)},
						}
						alarms[k].Alarm = &Alarm{
							At:       &device.TimeLocal{Time: time.Unix(notify.GetTimestamp()/1000, 0)},
							Location: v.GetAlarmLoc(),
							State:    v.GetAlarmType(), //报警类型
						}
					}
					select {
					case a.ChanZonesAlarm <- ZonesAlarm{
						DTS:       a.DTS,
						Zones:     alarms,
						Host:      a.DTS.Host,
						DeviceId:  notify.GetDeviceID(),
						CreatedAt: &device.TimeLocal{Time: time.Unix(notify.GetTimestamp()/1000, 0)},
					}:
					default:
						log.L.Warn(fmt.Sprintf("主机 %s 报警防区缓冲已满,忽略当前报警...", a.DTS.Host))
					}
				})
				if err != nil {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts注册报警回调失败: %s", a.DTS.Host, err), logrus.ErrorLevel)
					time.Sleep(time.Second * 3)
					break START
				} else {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts注册报警回调", a.DTS.Host), logrus.InfoLevel)
				}
			case CallTemp:
				a.setMessage(fmt.Sprintf("主机为 %s 的dts 注册温度更新回调", a.DTS.Host), logrus.InfoLevel)
				a.ChanZonesTemp = make(chan ZonesTemp, 30)
				err = a.Client.CallZoneTempNotify(func(notify *model.ZoneTempNotify, err error) {

					//若当前温度更新防区已存在缓存中,且当前温度更新时间与上一个温度更新时间的间隔小于温度更新阈值,则不处理
					value, ok := a.ZonesTemp.LoadOrStore(notify.GetDeviceID(), notify)
					if ok && time.Now().Sub(time.Unix(value.(*model.ZoneTempNotify).GetTimestamp()/1000, 0)) < time.Second*time.Duration(a.Config.ZonesTempSec) {
						return
					}
					a.ZonesTemp.Store(notify.GetDeviceID(), notify)

					var (
						index = 0                                   //温度更新防区索引,最后一个索引即为当前温度更新防区的长度
						zones = make(Zones, len(notify.GetZones())) //实际可用的温度更新防区
					)
					for _, zone := range notify.GetZones() {
						//对温度校验,当最大温度,平均温度,最小温度均为0时,则当前温度更新防区无效
						if zone.GetMaxTemperature() == 0 && zone.GetAverageTemperature() == 0 && zone.GetMinTemperature() == 0 {
							continue
						}
						id := Id(a.DTS.Id, uint(zone.GetID()))
						z := a.GetZone(id)
						if z == nil {
							a.setMessage(fmt.Sprintf("主机为 %s 的dts %d 防区未找到", a.DTS.Host, id), logrus.ErrorLevel)
							continue
						}

						zones[index] = &Zone{
							BaseZone: BaseZone{Id: id, ChannelId: z.ChannelId},
							Temperature: &Temperature{
								Max: zone.GetMaxTemperature(),
								Avg: zone.GetAverageTemperature(),
								Min: zone.GetMinTemperature(),
							},
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
						CreatedAt: &device.TimeLocal{Time: time.Unix(notify.GetTimestamp()/1000, 0)},
						Zones:     zones,
					}:
					default:
						log.L.Warn(fmt.Sprintf("主机 %s 温度更新防区缓冲已满,忽略当前更新...", a.DTS.Host))
					}
				})
				if err != nil {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts注册实时温度更新回调失败: %s", a.DTS.Host, err), logrus.ErrorLevel)
					break START
				} else {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts注册实时温度更新回调", a.DTS.Host), logrus.InfoLevel)
				}
			case CallSignal:
				a.setMessage(fmt.Sprintf("主机为 %s 的dts 注册通道信号回调", a.DTS.Host), logrus.InfoLevel)
				a.ChanChannelSignal = make(chan ChannelSignal, 30)
				err = a.Client.CallTempSignalNotify(func(notify *model.TempSignalNotify, err error) {
					value, ok := a.ZonesChannelSignal.LoadOrStore(fmt.Sprintf("%s-%d", notify.GetDeviceID(), notify.ChannelID), notify)
					if ok && time.Now().Sub(time.Unix(value.(*model.TempSignalNotify).GetTimestamp()/1000, 0)) < time.Second*time.Duration(a.Config.ChanSignSec) {
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
						CreatedAt:  &device.TimeLocal{Time: time.Unix(notify.GetTimestamp()/1000, 0)},
					}
					select {
					case a.ChanChannelSignal <- signal:
					default:
						log.L.Warn(fmt.Sprintf("主机 %s 通道 %d 信号缓冲已满,忽略当前更新...", a.DTS.Host, signal.ChannelId))
					}
				})
				if err != nil {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts注册信号回调失败: %s", a.DTS.Host, err), logrus.ErrorLevel)
					break START
				}
			case CallEvent:
				a.setMessage(fmt.Sprintf("主机为 %s 的dts 注册通道光纤事件回调", a.DTS.Host), logrus.InfoLevel)
				a.ChanChannelEvent = make(chan ChannelEvent, 10)
				err = a.Client.CallDeviceEventNotify(func(notify *model.DeviceEventNotify, err error) {
					event := ChannelEvent{
						DTS:           a.DTS,
						DeviceId:      notify.GetDeviceID(),
						ChannelId:     notify.GetChannelID(),
						Host:          a.DTS.Host,
						EventType:     notify.GetEventType(),
						ChannelLength: notify.GetChannelLength(),
						CreatedAt:     &device.TimeLocal{Time: time.Unix(notify.GetTimestamp()/1000, 0)},
					}
					select {
					case a.ChanChannelEvent <- event:
					default:
					}
				})
				if err != nil {
					a.setMessage(fmt.Sprintf("主机为 %s 的dts注册信号回调失败: %s", a.DTS.Host, err), logrus.ErrorLevel)
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

// Destroy 销毁通道
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

// Close 关闭DTS设备
func (a *App) Close() error {
	a.cancel()
	status := a.GetStatus()
	if status == device.Connecting || status == device.Connected {
		a.Client.Close()
	}
	for _, id := range a.CronIds {
		a.Cron.Remove(id)
	}
	a.setStatus(device.Disconnect)
	time.AfterFunc(time.Second*3, a.Destroy)
	return nil
}

// Status 获取当前设备的运行状态
func (a *App) Status() device.StatusType {
	a.locker.Lock()
	defer a.locker.Unlock()
	return a.status
}

func (a *App) setStatus(s device.StatusType) {
	a.locker.Lock()
	a.status = s
	a.locker.Unlock()
	if a.ChanStatus == nil {
		return
	}
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

// GetZone 根据获取防区防区映射Id获取防区信息
func (a *App) GetZone(id uint) *Zone {
	return a.GetZones()[id]
}

// GetZones 以map方式获取当前设备的所有防区,key为设备id和防区id的映射
func (a *App) GetZones() map[uint]*Zone {
	a.locker.Lock()
	defer a.locker.Unlock()
	return a.Zones
}

// GetSyncChannelZones 根据通道 Id 获取防区集合
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

// GetDeviceCode 获取设备编码
func (a *App) GetDeviceCode() (string, error) {
	response, err := a.Client.GetDeviceID()
	if err != nil {
		return "", err
	}
	if !response.Success {
		return "", errors.New(response.ErrMsg)
	}
	return response.GetDeviceID(), nil
}
