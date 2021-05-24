package main

import (
	"context"
	"fmt"
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
			{EnableRelay: false, EnableWarehouse: false, ChannelNum: 4, Host: "192.168.0.215"},
			{EnableRelay: false, EnableWarehouse: false, ChannelNum: 4, Host: "192.168.0.86"},
		},
	}
	for _, config := range core.configs {
		go func(config dts.Config) {
			app := dts.New(core.ctx, config)
			core.locker.Lock()
			core.apps[config.Host] = app
			core.locker.Unlock()
			app.Run()

			for {
				select {
				case <-app.Context.Done():
					app.Client.Close()
					fmt.Println("out")
					return
				case temp := <-app.ChanZonesTemp:
					log.Println("temp", temp.DeviceId)
				case sign := <-app.ChanChannelSignal:
					log.Println("sign", sign.DeviceId)
				case event := <-app.ChanChannelEvent:
					log.Println("event", event.DeviceId)
				case alarm := <-app.ChanZonesAlarm:
					log.Println("alarm start", alarm.DeviceId)
					time.Sleep(time.Second)
					log.Println("alarm over", alarm.DeviceId)
				}
			}
		}(config)
	}

	time.AfterFunc(time.Second*20, func() {
		core.locker.Lock()
		core.apps[core.configs[0].Host].Cancel()
		core.locker.Unlock()
	})

	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGABRT)
	select {
	case <-stop:
		log.Println("stop the word")
		return
	case <-core.ctx.Done():
		log.Println("done the word")
		return
	}
}
