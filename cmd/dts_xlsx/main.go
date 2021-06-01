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
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour*241)
	host := "192.168.0.215"
	app := dts.New(ctx, dts.DTS{Id: 1, Host: host}, dts.Config{ChannelNum: 4, ZonesTempInterval: 6})
	time.AfterFunc(time.Hour*240, cancel)
	app.Run()
	store := xlsx.New(ctx, xlsx.Config{
		Host:          host,
		Dir:           "./xlsx",
		MinTempMinute: 1,
		MinSaveHour:   6,
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
