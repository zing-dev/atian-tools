package main

import (
	"fmt"
	"github.com/zing-dev/atian-tools/cfg"
	"github.com/zing-dev/atian-tools/log"
	"net"
	"strconv"
	"strings"
)

const SectionName = "ATian-DaLian-HaoSen"

type Config struct {
	Debug      bool     `comment:"是否为调试模式"`
	DTSIp      []string `comment:"DTS主机地址支持多地址逗号隔开(例 192.168.0.1,192.168.0.2)"`
	ServerHost string   `comment:"接收报警服务器IP(例 127.0.0.1:1233)"`
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

		host := strings.Split(config.ServerHost, ":")
		if len(host) != 2 {
			log.L.Fatal("")
		}
		if ip := net.ParseIP(host[0]); ip == nil {
			log.L.Fatal("")
		}

		port, err := strconv.Atoi(host[1])
		if err != nil {
			log.L.Fatal("")
		}
		if 100 > port || port > 65535 {
			log.L.Fatal("")
		}

	}).Load()
	return config
}
