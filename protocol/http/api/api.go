package api

import (
	"bytes"
	"encoding/json"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/source/atian/dts"
	"github.com/zing-dev/atian-tools/source/device"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	_           Type = iota
	Ping             //心跳
	Alarm            //报警
	Fault            //故障
	Temperature      //温度更新
	ChannelSign      //通道信号

	ContentTypeJson = "application/json;charset=UTF-8"
)

type Type byte

type Request struct {
	Type  Type               `json:"type"`
	Host  string             `json:"host"`
	Zone  *dts.Zone          `json:"zone,omitempty"`
	Sign  *dts.ChannelSignal `json:"sign,omitempty"`
	Fault *dts.ChannelEvent  `json:"fault,omitempty"`
}

func (r Request) JSON() []byte {
	data, _ := json.Marshal(r)
	return data
}

type Api struct {
	Host string
	URL  string

	Client http.Client
	cron   *cron.Cron
	CronId cron.EntryID

	locker sync.Mutex
	status device.StatusType
}

func (a *Api) GetId() string {
	return "api-" + a.URL
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
	resp, err := a.Client.Post(a.URL, ContentTypeJson, bytes.NewBuffer(Request{
		Type: Ping,
		Host: a.Host,
	}.JSON()))
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
	request.Host = a.Host
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
	id, err := a.cron.AddFunc("* */1 * * * *", a.ping)
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
