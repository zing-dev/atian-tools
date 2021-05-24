package main

import (
	"context"
	"fmt"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"log"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	app := dts.New(ctx, dts.Config{
		EnableWarehouse: false,
		EnableRelay:     false,
		ChannelNum:      4,
		Host:            "192.168.0.215",
	})
	app.Run()
	go func() {
		for {
			select {
			case <-time.After(time.Second * 16):
				log.Println("----> ping")
				cancel()
			case <-time.After(time.Second * 30):
				log.Println("----> check")
			}
		}
	}()
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
			time.Sleep(time.Second * 5)
			log.Println("alarm over", alarm.DeviceId)
		}
	}
}
