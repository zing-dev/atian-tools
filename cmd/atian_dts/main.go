package main

import (
	"atian.tools/source/atian/dts"
	"context"
	"fmt"
	"time"
)

func main() {
	app := dts.New(context.Background(), dts.Config{
		EnableWarehouse: false,
		EnableRelay:     false,
		ChannelNum:      4,
		Host:            "192.168.0.215",
	})
	app.Run()
	for {
		select {
		case temp := <-app.ChanZonesTemp:
			fmt.Println(temp.JSON())
		case sign := <-app.ChanChannelSignal:
			fmt.Println(sign)
		case event := <-app.ChanChannelEvent:
			fmt.Println(event)
		case alarm := <-app.ChanZonesAlarm:
			fmt.Println(alarm)
		default:
			time.Sleep(time.Second)
		}
	}
}
