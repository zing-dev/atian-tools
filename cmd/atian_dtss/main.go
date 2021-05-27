package main

import (
	"context"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/source/atian/dts"
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

	apps    map[string]*dts.App
	configs []dts.Config

	locker sync.Mutex
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	core := Core{
		ctx:    ctx,
		cancel: cancel,
		apps:   map[string]*dts.App{},
		configs: []dts.Config{
			{ChannelNum: 4, Host: "192.168.0.86"},
			{EnableRelay: false, EnableWarehouse: false, ChannelNum: 4, Host: "192.168.0.215"},
		},
	}
	for _, config := range core.configs {
		go func(config dts.Config) {
			app := dts.New(core.ctx, config)
			app.Cron = cron.New(cron.WithSeconds())
			core.locker.Lock()
			core.apps[config.Host] = app
			core.locker.Unlock()
			id, err := app.Cron.AddFunc("*/10 * * * * *", func() {
				log.Println("cron ", app.Config.Host)
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
				case temp := <-app.ChanZonesTemp:
					log.Println("temp", temp.Host)
				case sign := <-app.ChanChannelSignal:
					log.Println("sign", sign.Host, sign.ChannelId)
				case event := <-app.ChanChannelEvent:
					log.Println("event", event.Host)
				case alarm := <-app.ChanZonesAlarm:
					log.Println("alarm start", alarm.Host, dts.GetAlarmTypeString(alarm.Zones[0].AlarmType))
					time.Sleep(time.Second)
					log.Println("alarm over", alarm.Host, dts.GetAlarmTypeString(alarm.Zones[0].AlarmType))
				}
			}
		}(config)
	}

	time.AfterFunc(time.Minute, func() {
		core.locker.Lock()
		core.apps[core.configs[1].Host].Close()
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
