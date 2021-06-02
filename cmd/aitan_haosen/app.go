package main

import (
	"context"
	"fmt"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/protocol/tcp/haosen"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"github.com/zing-dev/atian-tools/source/device"
)

type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	config *Config
	client *haosen.Client
	manger *device.Manger
}

func NewApp() *App {
	config := newConfig()
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		ctx:    ctx,
		cancel: cancel,
		config: config,
	}
}
func (a *App) Run() {
	a.client = haosen.NewClient(a.ctx, "192.168.0.251:9090")
	err := a.client.Connect()
	if err != nil {
		log.L.Fatal(err)
	}
	a.manger = device.NewManger(a.ctx)
	a.manger.Register(device.EventAdd, func(d device.Device) {
		switch d.GetType() {
		case device.TypeDTS:
			go func(app *dts.App) {
				app.CallTypes = []dts.CallType{dts.CallTemp, dts.CallAlarm, dts.CallEvent}
				for {
					select {
					case <-app.Context.Done():
						log.L.Warn("over")
					case status := <-app.ChanStatus:
						if status == device.Connected {
							log.L.Info("获取防区开始")
							app.SyncZones()
							log.L.Info("获取防区结束")
							err := app.Register()
							if err != nil {
								log.L.Error("register ", err)
							}
						}
						log.L.Info("status", status)
					case alarm := <-app.ChanZonesAlarm:
						log.L.Info("alarm ", alarm.DeviceId)
						zones := make([]haosen.Alarm, len(alarm.Zones))
						var i = 0
						for _, zone := range alarm.Zones {
							if zone.Coordinate == nil {
								log.L.Error(fmt.Sprintf("报警防区 %s 无有效的坐标", zone.Name))
								continue
							}
							zones[i] = haosen.Alarm{
								Zone: haosen.Zone{
									ZoneId:      zone.Name,
									Line:        "1111",
									X:           int(zone.Coordinate.Row),
									Y:           int(zone.Coordinate.Column),
									Z:           int(zone.Coordinate.Layer),
									Temperature: fmt.Sprintf("%.3f", zone.Temperature.Avg),
								},
							}
							i++
						}
						zones = zones[:i]
						if i == 0 {
							log.L.Error(fmt.Sprintf("报警主机 %s 无有效的报警防区信息", alarm.DTS.Host))
							continue
						}
						request := haosen.AlarmRequest{
							CMD:       haosen.CMDAlarm,
							GUID:      alarm.CreatedAt.Format(haosen.GUIDFormat),
							DeviceId:  alarm.DeviceId,
							TimeStamp: alarm.CreatedAt.Format(haosen.LocalTimeFormat),
							Alarms: haosen.Alarms{
								Count:  len(zones),
								Alarms: zones,
							},
						}
						response, err := a.client.Send(haosen.MsgAlarm, request)
						if err != nil {
							log.L.Error(err)
						} else {
							log.L.Info(response.ErrorCode, response.ErrorMsg)
						}

					case event := <-app.ChanChannelEvent:
						log.L.Info("event ", event.DeviceId)
						request := haosen.EventRequest{
							CMD:       haosen.CMDAlarm,
							GUID:      event.CreatedAt.Format(haosen.GUIDFormat),
							DeviceId:  event.DeviceId,
							TimeStamp: event.CreatedAt.Format(haosen.LocalTimeFormat),
							EventType: event.EventType.String(),
						}
						response, err := a.client.Send(haosen.MsgEvent, request)
						if err != nil {
							log.L.Error(err)
						} else {
							log.L.Info(response.ErrorCode, response.ErrorMsg)
						}

					case temp := <-app.ChanZonesTemp:
						log.L.Info("temp ", temp.DeviceId)
						datas := make([]haosen.Data, len(temp.Zones))
						var i = 0
						for _, zone := range temp.Zones {
							if zone.Coordinate == nil {
								log.L.Error(fmt.Sprintf("更新防区温度 %s 无有效的坐标", zone.Name))
								continue
							}
							datas[i] = haosen.Data{
								Zone: haosen.Zone{
									ZoneId:      zone.Name,
									Line:        "1111",
									X:           int(zone.Coordinate.Row),
									Y:           int(zone.Coordinate.Column),
									Z:           int(zone.Coordinate.Layer),
									Temperature: fmt.Sprintf("%.3f", zone.Temperature.Avg),
								},
							}
							i++
						}
						datas = datas[:i]
						if i == 0 {
							log.L.Error(fmt.Sprintf("%s 无有效的防区,无法更新温度信息", temp.DTS.Host))
							continue
						}
						request := haosen.RealTimeTempRequest{
							CMD:       haosen.CMDAlarm,
							GUID:      temp.CreatedAt.Format(haosen.GUIDFormat),
							DeviceId:  temp.DeviceId,
							TimeStamp: temp.CreatedAt.Format(haosen.LocalTimeFormat),
							Datas: haosen.Datas{
								Count: 0,
							},
						}
						response, err := a.client.Send(haosen.MsgRealTimeTemp, request)
						if err != nil {
							log.L.Error(err)
						} else {
							log.L.Info(response.ErrorCode, response.ErrorMsg)
						}
					}
				}
			}(d.(*dts.App))
		}
	})
	for k, v := range a.config.DTSIp {
		a.manger.Add(dts.New(a.manger.Context, dts.DTS{Id: uint(k + 1), Host: v}, dts.Config{ChannelNum: 4, Coordinate: true}))
	}

	a.manger.Range(func(s string, d device.Device) {
		err := d.Run()
		if err != nil {
			log.L.Error("run ", d.GetType(), err)
		}
	})

}
func (a *App) Close() {
	a.cancel()
}
