package main

import (
	"github.com/zing-dev/atian-tools/log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Init()
	NewApp().Run()
	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGHUP, syscall.SIGKILL)
	<-stop
}
