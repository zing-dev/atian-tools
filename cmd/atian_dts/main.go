package main

import (
	"context"
	"fmt"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	app := dts.New(ctx, dts.Config{
		EnableWarehouse: false,
		EnableRelay:     false,
		ChannelNum:      4,
		Host:            "192.168.0.215",
	})
	app.Run()
	for {
		select {
		case temp := <-app.ChanZonesTemp:
			fmt.Println(temp.Zones[0])
		case sign := <-app.ChanChannelSignal:
			fmt.Println("sign", sign.RealLength)
		case event := <-app.ChanChannelEvent:
			fmt.Println(event)
		case alarm := <-app.ChanZonesAlarm:
			fmt.Println(alarm)
		case <-app.Context.Done():
			app.Client.Close()
			fmt.Println("out")
			return
		case <-time.After(time.Second * 5):
			fmt.Println("will out")
			cancel()
		default:
			time.Sleep(time.Second)
		}
	}
}
