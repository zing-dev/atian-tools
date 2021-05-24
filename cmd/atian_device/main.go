package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/source/device"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type DTS struct {
	ctx    context.Context
	cancel context.CancelFunc
	id     string
}

func (d *DTS) GetId() string {
	return d.id
}

func (d *DTS) GetType() device.Type {
	return device.TypeDTS
}

func (d *DTS) GetStatus() device.StatusType {
	return device.Connecting
}

func (d *DTS) Cron(cron *cron.Cron) {
}

func (d *DTS) Run() {
	time.Sleep(time.Millisecond)
	log.Println(d.GetId(), "run")
	time.Sleep(time.Millisecond)
}

func (d *DTS) Close() {
	log.Println(d.GetId(), "close")
}

func main() {
	manger := device.NewManger(context.Background())
	manger.RegisterEvent(device.EventError, func(listener device.EventListener) {})
	manger.RegisterEvent(device.EventAdd, func(listener device.EventListener) {
		listener.Device.Run()
	})
	manger.RegisterEvent(device.EventRun, func(listener device.EventListener) {})
	manger.RegisterEvent(device.EventUpdate, func(listener device.EventListener) {})
	manger.RegisterEvent(device.EventClose, func(listener device.EventListener) {})
	manger.RegisterEvent(device.EventDelete, func(listener device.EventListener) {
		listener.Device.Close()
	})

	ctx, cancel := context.WithCancel(manger.Context)
	dd := make([]*DTS, 20)
	for i := 0; i < 20; i++ {
		dd[i] = &DTS{id: fmt.Sprintf("%d", i), ctx: ctx, cancel: cancel}
		manger.Adds(dd[i])
	}
	w := gin.Default()
	connections := make([]*websocket.Conn, 0)
	w.GET("/", func(c *gin.Context) {
		manger.WriteToWebsocket(connections...)
	})
	w.GET("/api", func(c *gin.Context) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Fatal(err)
		}
		connections = append(connections, conn)
	})
	go w.Run(":1234")
	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGABRT, syscall.SIGHUP)
	<-stop
}
