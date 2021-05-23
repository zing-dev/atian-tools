package main

import (
	"context"
	"fmt"
	"github.com/zing-dev/atian-tools/cfg"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/protocol/http/nandu"
	"github.com/zing-dev/atian-tools/source/beida_bluebird"
	"net/url"
	"os"
)

const SectionName = "BeiDaBlueBird-HTTP"

type Config struct {
	MapFile    string `comment:"防区和设备的映射文件,必须是xlsx文件(例 ./map_file.xlsx)"`
	SerialPort string `comment:"串口地址(例 COM1)"`
	HTTPUrl    string `comment:"webservice接收报警地址(例 http://127.0.0.1/alarm)"`
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
		if config.SerialPort == "" {
			log.L.Fatal("请输入设备串口号")
		}

		if config.HTTPUrl == "" {
			log.L.Fatal("请输入报警地址")
		}

		if _, err := url.Parse(config.HTTPUrl); err != nil {
			log.L.Fatal("报警地址非法")
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
	service *nandu.HTTP
}

func main() {
	log.L.Info("start running...")
	ctx, cancel := context.WithCancel(context.Background())
	app := App{
		ctx:    ctx,
		cancel: cancel,
		config: newConfig(),
	}

	app.service = nandu.New(ctx, app.config.HTTPUrl)
	sensation := beida_bluebird.New(app.ctx, &beida_bluebird.Config{Port: app.config.SerialPort, MapFile: app.config.MapFile})
	go sensation.Run()
	go app.service.Ping()
	for {
		select {
		case <-app.ctx.Done():
			return
		default:
			protocol := sensation.Protocol()
			fmt.Println(protocol.String())
			// beida_bluebird.PartTypeManual 适配老版本的报警
			if protocol.IsTypeSmokeSensation() || protocol.PartType == beida_bluebird.PartTypeManual {
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
				if protocol.IsCmdAlarm() {
					for _, item := range list {
						log.L.Warn("发生报警: ", item.String())
						response, err := app.service.Send(nandu.Request{
							LocationCode: item.Name,
							Status:       nandu.CodeAlarm,
						})
						if err != nil {
							log.L.Error(fmt.Sprintf("发送报警失败: %s", err))
							return
						}
						log.L.Info("报警结果: ", response.String())
					}
				}
			}
		}
	}
}
