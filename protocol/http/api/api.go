package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"github.com/zing-dev/atian-tools/source/device"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	Error          Type = iota
	Ping                //心跳 发
	Pong                //心跳 收
	DTSAlarm            //报警
	DTSFiber            //光线状态
	DTSTemperature      //温度更新
	DTSChannelSign      //通道信号

	ContentTypeJson = "application/json;charset=UTF-8"
)

type Type byte

type Request struct {
	Type  Type               `json:"type"`
	Zone  *dts.Zone          `json:"zone,omitempty"`  //单个报警的防区信息
	Sign  *dts.ChannelSignal `json:"sign,omitempty"`  //单个通道的信号数据
	Fiber *dts.ChannelEvent  `json:"fiber,omitempty"` //单个通道的光纤状态
}

type Response struct {
	Code byte        `json:"code,omitempty"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

func (r Request) JSON() []byte {
	data, _ := json.Marshal(r)
	return data
}

var (
	once = sync.Once{}
	api  *Api
)

type Api struct {
	URL string

	Client http.Client
	cron   *cron.Cron
	CronId cron.EntryID

	locker sync.Mutex
	status device.StatusType
}

func New(url string) *Api {
	once.Do(func() {
		api = &Api{
			URL:    url,
			Client: http.Client{Timeout: 3 * time.Second},
			locker: sync.Mutex{},
		}
	})
	return api
}

func (a *Api) GetId() string {
	t := device.TypeApi
	return fmt.Sprintf("%s-%s", t.String(), a.URL)
}

func (a *Api) GetType() device.Type {
	return device.TypeApi
}

func (a *Api) SetCron(cron *cron.Cron) {
	a.cron = cron
}

func (a *Api) setStatus(t device.StatusType) {
	a.locker.Lock()
	defer a.locker.Unlock()
	a.status = t
}

func (a *Api) GetStatus() device.StatusType {
	a.locker.Lock()
	defer a.locker.Unlock()
	return a.status
}

func (a *Api) ping() {
	a.setStatus(device.Connecting)
	resp, err := a.Client.Post(a.URL, ContentTypeJson, bytes.NewBuffer(Request{Type: Ping}.JSON()))
	if err != nil {
		a.setStatus(device.Disconnect)
		return
	}
	if resp.StatusCode != http.StatusOK {
		a.setStatus(device.Disconnect)
		return
	}
	a.setStatus(device.Connected)
}

func (a *Api) Post(request Request) ([]byte, error) {
	resp, err := a.Client.Post(a.URL, ContentTypeJson, bytes.NewBuffer(request.JSON()))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			return
		}
	}()
	return io.ReadAll(resp.Body)
}

func (a *Api) Run() {
	a.Client = http.Client{Timeout: time.Second * 3}
	id, err := a.cron.AddFunc("0 */1 * * * *", a.ping)
	if err != nil {
		return
	}
	a.CronId = id
}

func (a *Api) Close() {
	a.locker.Lock()
	defer a.locker.Unlock()
	a.cron.Remove(a.CronId)
}
