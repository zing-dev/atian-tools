package main

import (
	"atian.tools/cfg"
	"atian.tools/log"
	"atian.tools/protocol/soap/q5"
	"atian.tools/source/beida_bluebird"
	"context"
	"fmt"
	"github.com/hooklift/gowsdl/soap"
	"net/url"
	"os"
	"time"
)

const SectionName = "BeiDaBlueBird-WebService"

type Config struct {
	MapFile       string `comment:"防区和设备的映射文件,必须是xlsx文件(例 ./map_file.xlsx)"`
	SerialPort    string `comment:"串口地址(例 COM1)"`
	WebServiceUrl string `comment:"webservice接收报警地址(例 http://127.0.0.1/webservice)"`
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

		if config.WebServiceUrl == "" {
			log.L.Fatal("请输入报警地址")
		}

		if _, err := url.Parse(config.WebServiceUrl); err != nil {
			log.L.Fatal("报警地址非法")
		}

		if config.SerialPort == "" {
			log.L.Fatal("请输入设备串口号")
		}

		if config.MapFile == "" {
			log.L.Fatal("请输入防区设备的映射文件路径")
		}

		_, err = os.Stat(config.MapFile)
		if os.IsNotExist(err) {
			log.L.Fatal("当前防区设备的映射文件不存在")
		}

		if os.IsPermission(err) {
			log.L.Fatal("当前防区设备的映射文件不可读")
		}
	}).Load()
	return config
}

type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	config  *Config
	service q5.IDtsWcfService
}

func main() {
	log.L.Info("start running...")
	ctx, cancel := context.WithCancel(context.Background())
	app := App{
		ctx:    ctx,
		cancel: cancel,
		config: newConfig(),
	}

	app.service = q5.NewIDtsWcfService(soap.NewClient(app.config.WebServiceUrl, soap.WithTimeout(time.Second*3)))
	sensation := beida_bluebird.New(app.ctx, &beida_bluebird.Config{Port: app.config.SerialPort, MapFile: app.config.MapFile})
	go sensation.Run()

	for {
		select {
		case <-app.ctx.Done():
			return
		default:
			protocol := sensation.Protocol()
			if protocol.IsTypeSmokeSensation() {
				m := &beida_bluebird.Map{
					Controller: protocol.Controller,
					Loop:       protocol.Loop,
					Part:       protocol.Part,
					PartType:   protocol.PartType,
				}
				list := sensation.Maps.Get(m.Key())
				if list == nil {
					log.L.Error(fmt.Sprintf("未找到当前的报警防区[控制器号 %d,回路号 %d,部位号 %d,部件类型 %d]", m.Controller, m.Loop, m.Part, m.PartType))
					continue
				}

				for _, item := range list {
					if protocol.IsCmdFailure() {
						response, err := app.service.DeviceWarn(&q5.DeviceWarn{
							LocCode:     item.Code,
							WarnContext: item.String(),
						})
						if err != nil {
							log.L.Error("FireWarn err: ", err)
							continue
						}

						if response.DeviceWarnResult {
							continue
						}
					}

					if protocol.IsCmdAlarm() {
						response, err := app.service.FireWarn(&q5.FireWarn{
							LocCode:     m.Code,
							WarnContext: item.String(),
						})
						if err != nil {
							log.L.Error("FireWarn err: ", err)
							continue
						}

						if response.FireWarnResult {
							continue
						}
					}
				}
			}
		}
	}
}
