package main

import (
	"context"
	"fmt"
	"github.com/judwhite/go-svc"
	"github.com/zing-dev/atian-tools/cfg"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/protocol/http/xiandao"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"github.com/zing-dev/atian-tools/source/device"
	"net/http"
	"net/url"
	"time"
)

const SectionName = "ATian-XianDao"

type Config struct {
	Debug bool     `comment:"是否为调试模式"`
	DTSIp []string `comment:"DTS主机地址支持多地址逗号隔开(例 192.168.0.1,192.168.0.2)"`
	URL   string   `comment:"报警地址(例 http://127.0.0.1/alarm)"`
}

func newConfig() *Config {
	config := new(Config)
	cfg.New().Register(func(c *cfg.Config) {
		section := c.File.Section(SectionName)
		if section.Comment == "" {
			section.Comment = fmt.Sprintf("项目名: %s", SectionName)
		}
		if len(section.Keys()) == 0 {
			err := section.ReflectFrom(config)
			if err != nil {
				log.L.Fatal(fmt.Sprintf("%s 反射失败: %s", SectionName, err))
			}
			c.Save()
		}
		err := section.MapTo(config)
		if err != nil {
			log.L.Fatal(fmt.Sprintf("映射错误: %s", err))
		}

		if config.URL == "" {
			log.L.Fatal("请输入报警地址")
		}

		if _, err := url.Parse(config.URL); err != nil {
			log.L.Fatal("报警地址非法")
		}
	}).Load()
	return config
}

// implements svc.Service
type program struct {
	app *App
}

type App struct {
	ctx    context.Context
	cancel context.CancelFunc
	config *Config

	service *xiandao.HTTP
}

func (app *App) run() {
	for i, ip := range app.config.DTSIp {
		go func(i int, ip string) {
			a := dts.New(app.ctx, dts.DTS{Id: uint(i + 1), Name: ip, Host: ip}, dts.Config{ChannelNum: 4})
			a.CallTypes = []dts.CallType{dts.CallAlarm}
			go func() {
				for {
					select {
					case <-time.After(time.Second * 30):
						if a.GetStatus() != device.Connected {
							log.L.Error(fmt.Sprintf("主机 %s 不在线", a.DTS.Host))
						} else {
							log.L.Info(fmt.Sprintf("主机 %s 在线", a.DTS.Host))
						}
					case status := <-a.ChanStatus:
						if status != device.Connected {
							log.L.Error(fmt.Sprintf("主机 %s 不在线", a.DTS.Host))
						} else {
							log.L.Info(fmt.Sprintf("主机 %s 在线", a.DTS.Host))
						}
					case alarms := <-a.ChanZonesAlarm:
						for _, zone := range alarms.Zones {
							response, err := app.service.Post(xiandao.Request{LocationCode: zone.Name})
							if err != nil {
								log.L.Error(fmt.Sprintf("主机 %s 通道 %d 防区 %s 报警失败: %s", zone.DTS.Host, zone.ChannelId, zone.Name, err))
								continue
							}
							if response.Code == 0 {
								log.L.Warn(fmt.Sprintf("主机 %s 通道 %d 防区 %s 报警成功: %s", zone.DTS.Host, zone.ChannelId, zone.Name, response.Msg))
								continue
							}
							log.L.Error(fmt.Sprintf("主机 %s 通道 %d 防区 %s 报警失败: %s", zone.DTS.Host, zone.ChannelId, zone.Name, response.Msg))
						}
					}
				}
			}()
			err := a.Run()
			if err != nil {
				log.L.Error(fmt.Sprintf("运行主机 %s 失败", err))
				return
			}
		}(i, ip)
	}
}

func (app *App) stop() error {
	app.cancel()
	return nil
}

func main() {
	log.Init()
	config := newConfig()
	ctx, cancel := context.WithCancel(context.Background())
	prg := program{
		app: &App{
			ctx:    ctx,
			cancel: cancel,
			config: config,
			service: &xiandao.HTTP{
				URL:    config.URL,
				Client: http.Client{Timeout: time.Second * 3},
			},
		},
	}
	if err := svc.Run(&prg); err != nil {
		log.L.Fatal(err)
	}
}

func (p *program) Init(env svc.Environment) error {
	log.L.Info("is win service? ", env.IsWindowsService())
	return nil
}

func (p *program) Start() error {
	log.L.Info("Starting...")
	go p.app.run()
	return nil
}

func (p *program) Stop() error {
	log.L.Info("Stopping...")
	if err := p.app.stop(); err != nil {
		return err
	}
	log.L.Info("Stopped.")
	return nil
}
