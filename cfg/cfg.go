package cfg

import (
	"fmt"
	"github.com/zing-dev/atian-tools/log"
	"gopkg.in/ini.v1"
	"os"
	"sync"
)

const defaultFilename = "config.ini"

var (
	o sync.Once
	c *Config
)

type Config struct {
	filename string
	File     *ini.File
	fn       []func(*Config)
}

func New() *Config {
	o.Do(func() {
		c = new(Config)
	})
	return c
}

func (c *Config) Register(f func(*Config)) *Config {
	c.fn = append(c.fn, f)
	return c
}

func (c *Config) Load() {
	if c.filename == "" {
		c.filename = defaultFilename
	}
	c.Check()
	for _, f := range c.fn {
		f(c)
	}
}

func (c *Config) Save() {
	err := c.File.SaveTo(c.filename)
	if err != nil {
		log.L.Fatal(fmt.Sprintf("保存配置文件[ %s ]失败: %s", c.filename, err))
	}
}

func (c *Config) Filename(filename string) {
	c.filename = filename
}

func (c *Config) Check() {
	file, err := ini.ShadowLoad(c.filename)
	if err != nil {
		f, err := os.Create(c.filename)
		if err != nil {
			log.L.Fatal(fmt.Sprintf("创建配置文件[ %s ]失败: %s", c.filename, err))
		}
		_ = f.Close()
		file, err = ini.ShadowLoad(c.filename)
		if err != nil {
			log.L.Fatal(fmt.Sprintf("加载配置文件[ %s ]失败: %s", c.Filename, err))
		}
	}
	c.File = file
}
