package main

import (
	"atian.tools/source/atian/dts"
	"context"
	"fmt"
)

func main() {
	app := dts.New(context.Background(), dts.Config{
		EnableWarehouse: false,
		EnableRelay:     false,
		ChannelNum:      4,
		Host:            "192.168.0.86",
	})
	alarm := app.RecAlarm()
	fmt.Println(alarm)
}
