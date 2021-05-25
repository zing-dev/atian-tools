package mqtt

import (
	"context"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	uuid "github.com/iris-contrib/go.uuid"
	"github.com/zing-dev/atian-tools/log"
	"time"
)

type Config struct {
	Url      string
	Username string
	Password string
}

type MQTT struct {
	ctx    context.Context
	cancel context.CancelFunc
	Client mqtt.Client
}

var (
	OnConnect = func(client mqtt.Client) {
		log.L.Info("MQTT连接成功")
	}
	OnConnectionLost = func(client mqtt.Client, e error) {
		log.L.Info("MQTT连接失败: ", e)
	}
)

func New(ctx context.Context, config Config) *MQTT {
	u2, _ := uuid.NewV4()
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("%s", config.Url))
	opts.SetClientID(u2.String())
	opts.ConnectTimeout = time.Second * 3
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetCleanSession(false)
	opts.AutoReconnect = true
	opts.MaxReconnectInterval = time.Second * 3
	opts.OnConnect = OnConnect
	opts.OnConnectionLost = OnConnectionLost
	ctx, cancel := context.WithCancel(ctx)
	return &MQTT{
		ctx:    ctx,
		cancel: cancel,
		Client: mqtt.NewClient(opts),
	}
}

func (m *MQTT) Run() {
	if token := m.Client.Connect(); token.Error() != nil && token.Wait() {
		log.L.Error("MQTT 连接失败: ", token.Error())
		return
	}
}
