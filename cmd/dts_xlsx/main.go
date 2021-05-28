package main

import (
	"context"
	"fmt"
	"github.com/zing-dev/atian-tools/protocol/xlsx"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"log"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour*2)
	host := "192.168.0.215"
	app := dts.New(ctx, dts.Config{
		EnableWarehouse:   false,
		EnableRelay:       false,
		ChannelNum:        4,
		Host:              host,
		ZonesTempInterval: 6,
	})
	time.AfterFunc(time.Hour, cancel)
	app.Run()
	store := xlsx.New(ctx, xlsx.Config{
		Host:          host,
		Dir:           "./xlsx",
		MinTempMinute: 1,
		MinSaveHour:   60,
	})
	for {
		select {
		case <-app.Context.Done():
			app.Client.Close()
			fmt.Println("out")
			return
		case temp := <-app.ChanZonesTemp:
			log.Println("temp", temp.DeviceId)
			store.Store(temp)
		}
	}
}
