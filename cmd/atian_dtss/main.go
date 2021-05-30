package main

import (
	"context"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"github.com/zing-dev/atian-tools/source/device"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Core struct {
	ctx    context.Context
	cancel context.CancelFunc

	apps   map[string]*dts.App
	DTS    []dts.DTS
	config dts.Config

	locker sync.Mutex

	CoordinateZones map[string]map[string]dts.Zones
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	core := Core{
		ctx:    ctx,
		cancel: cancel,
		apps:   map[string]*dts.App{},
		DTS: []dts.DTS{
			{Id: 0, Name: "", Host: "192.168.0.86"},
			{Id: 0, Name: "", Host: "192.168.0.215"},
		},
		config:          dts.Config{ChannelNum: 4},
		CoordinateZones: map[string]map[string]dts.Zones{},
	}
	for _, d := range core.DTS {
		d := d
		go func(d dts.DTS) {
			app := dts.New(core.ctx, d, core.config)
			app.Cron = cron.New(cron.WithSeconds())
			core.locker.Lock()
			core.apps[d.Host] = app
			core.locker.Unlock()
			id, err := app.Cron.AddFunc("*/10 * * * * *", func() {
				log.Println("cron ", d.Host)
			})
			if err != nil {
				return
			}
			app.CronIds[byte(id)] = id
			app.Cron.Start()
			app.Run()
			for {
				select {
				case <-app.Context.Done():
					app.Client.Close()
					fmt.Println("out")
					return
				case status := <-app.ChanStatus:
					log.Println("status", status.String())
					if status == device.Connected {
						log.Println("开始配置新能源防区结构层级...")
						core.locker.Lock()
						for _, zone := range app.Zones {
							if zone.Coordinate.Warehouse == "" || zone.Coordinate.Group == "" {
								continue
							}
							if len(core.CoordinateZones[zone.Coordinate.Warehouse]) == 0 {
								core.CoordinateZones[zone.Coordinate.Warehouse] = map[string]dts.Zones{}
							}
							if len(core.CoordinateZones[zone.Coordinate.Warehouse][zone.Coordinate.Group]) == 0 {
								core.CoordinateZones[zone.Coordinate.Warehouse][zone.Coordinate.Group] = dts.Zones{}
							}
							core.CoordinateZones[zone.Coordinate.Warehouse][zone.Coordinate.Group] =
								append(core.CoordinateZones[zone.Coordinate.Warehouse][zone.Coordinate.Group], zone)
						}
						core.locker.Unlock()
						log.Println("配置新能源防区结构层级结束...")
					}
				case temp := <-app.ChanZonesTemp:
					log.Println("temp", temp.Host)
				case sign := <-app.ChanChannelSignal:
					log.Println("sign", sign.Host, sign.ChannelId)
				case event := <-app.ChanChannelEvent:
					log.Println("event", event.Host)
				case alarm := <-app.ChanZonesAlarm:
					log.Println("alarm start", alarm.Host, dts.GetAlarmTypeString(alarm.Zones[0].Alarm.State))
					time.Sleep(time.Second)
					log.Println("alarm over", alarm.Host, dts.GetAlarmTypeString(alarm.Zones[0].Alarm.State))
				}
			}
		}(d)
	}

	time.AfterFunc(time.Minute, func() {
		core.locker.Lock()
		core.apps[core.DTS[1].Host].Close()
		core.locker.Unlock()
	})

	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGABRT)
	select {
	case <-stop:
		log.Println("stop the word")
		core.cancel()
		time.Sleep(time.Second)
		return
	case <-core.ctx.Done():
		log.Println("done the word")
		time.Sleep(time.Second)
		return
	}
}
