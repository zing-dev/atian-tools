package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	socket "github.com/zing-dev/atian-tools/protocol/websocket"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"github.com/zing-dev/atian-tools/source/device"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	connections := make([]*websocket.Conn, 0)
	manger := device.NewManger(context.Background())
	manger.Register(device.EventAdd, func(d device.Device) {
		app := d.(*dts.App)
		go func() {
			for {
				select {
				case <-app.Context.Done():
					socket.WriteToWebsocket(socket.TypeDevice, map[string]string{
						"status": "over",
					}, connections...)
					socket.WriteToWebsocket(socket.TypeDevice, manger.GetStatus(), connections...)
					return
				case <-app.ChanStatus:
					socket.WriteToWebsocket(socket.TypeDevice, manger.GetStatus(), connections...)
				case temp := <-app.ChanZonesTemp:
					log.Println("temp", temp.DTS.Host)
				}
			}
		}()
	})
	manger.Register(device.EventRun, func(d device.Device) {
		log.Println("run", d.GetId())
		err := d.Run()
		if err != nil {
			log.Println(err)
		}
		socket.WriteToWebsocket(socket.TypeDevice, manger.GetStatus(), connections...)
	})
	manger.Register(device.EventClose, func(d device.Device) {
		log.Println("close", d.GetId())
		err := d.Close()
		if err != nil {
			log.Println(err)
		}
		socket.WriteToWebsocket(socket.TypeDevice, manger.GetStatus(), connections...)
	})

	devices := []device.Device{
		dts.New(manger.Context, dts.DTS{Id: 1, Name: "1", Host: "192.168.0.215"}, dts.Config{ChannelNum: 4, ZonesTempInterval: 10}),
		dts.New(manger.Context, dts.DTS{Id: 2, Name: "2", Host: "192.168.0.86"}, dts.Config{ChannelNum: 4, ZonesTempInterval: 10}),
	}
	manger.Adds(devices...)
	manger.Range(func(s string, d device.Device) {
		err := d.Run()
		if err != nil {
			log.Println(err)
		}
	})
	go func() {
		for {
			select {
			case <-time.After(time.Second * 10):
				socket.WriteToWebsocket(socket.TypeDevice, manger.GetStatus(), connections...)
			}
		}
	}()
	w := gin.Default()
	w.GET("/run/:host", func(c *gin.Context) {
		for _, d := range devices {
			if d.GetId() == c.Param("host") {
				manger.Adds(d)
				manger.Run(d.GetId())
			}
		}
		c.JSON(200, manger.GetStatus())
	})
	w.GET("/close/:host", func(c *gin.Context) {
		manger.Close(c.Param("host"))
		c.JSON(200, manger.GetStatus())
	})
	w.GET("/api", func(c *gin.Context) {
		up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := up.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Fatal(err)
		}
		conn.WriteJSON(manger.GetStatus())
		connections = append(connections, conn)
	})
	go w.Run(":1234")
	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGABRT, syscall.SIGHUP)
	<-stop
}
