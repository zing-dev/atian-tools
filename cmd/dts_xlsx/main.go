package main

import (
	"context"
	"fmt"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/protocol/xlsx"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"github.com/zing-dev/atian-tools/source/device"
	"time"
)

func main() {
	log.Init()
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour*241)
	host := "192.168.0.86"
	store := xlsx.New(ctx, xlsx.Config{
		Host:          host,
		Dir:           "./xlsx/temp",
		MinTempMinute: 1,
		MinSaveHour:   6,
	})
	signalStore := xlsx.NewSignalStore(&xlsx.Config{
		Host:          host,
		Dir:           "./xlsx/signal",
		MinTempMinute: 1,
		MinSaveHour:   6,
	})

	app := dts.New(ctx, dts.DTS{Id: 1, Host: host}, &dts.Config{ChannelNum: 4, ZonesTempSec: 6})
	time.AfterFunc(time.Hour*240, cancel)
	app.CallTypes = []dts.CallType{dts.CallTemp, dts.CallSignal}
	err := app.Run()
	if err != nil {
		log.L.Fatal(err)
	}
	for {
		select {
		case <-app.Context.Done():
			app.Client.Close()
			fmt.Println("out")
			return
		case status := <-app.ChanStatus:
			log.L.Info(status)
			if status == device.Connected {
				app.SyncZones()
				err := app.Register()
				if err != nil {
					log.L.Fatal(err)
				} else {
					log.L.Info("注册成功")
				}
			}
		case temp := <-app.ChanZonesTemp:
			log.L.Info("temp", temp.DeviceId)
			store.Store(temp)
		case data := <-app.ChanChannelSignal:
			log.L.Info("signal:", data.DeviceId)
			signalStore.Store(data)
		}
	}
}
