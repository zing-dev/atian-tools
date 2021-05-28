package device

import (
	"context"
	"fmt"
	"github.com/robfig/cron/v3"
	"net/http"
	"sync"
	"time"
)

const (
	apiOnPoint  = "/api/on-point/%s/%s"
	apiOffPoint = "/api/off-point/%s/%s"

	ApiVersion = "/api/version"
	ApiPing    = "/api/ping"
)

type Relay struct {
	ctx    context.Context
	cancel context.CancelFunc

	ResetTime string
	Client    http.Client
	Tag       string
	Url       string
	Cron      *cron.Cron
	CronId    cron.EntryID
	locker    sync.Mutex
	status    StatusType
}

func (r *Relay) GetId() string {
	return fmt.Sprintf("relay-%s", r.Tag)
}

func (r *Relay) GetType() Type {
	return TypeRelay
}

func (r *Relay) SetCron(cron *cron.Cron) {
	r.Cron = cron
}

func (r *Relay) setStatus(t StatusType) {
	r.locker.Lock()
	defer r.locker.Unlock()
	r.status = t
}

func (r *Relay) GetStatus() StatusType {
	r.locker.Lock()
	defer r.locker.Unlock()
	return r.status
}

func (r *Relay) Reset(branch string) {
	url := fmt.Sprintf("%s/api/off/%s", r.Url, branch)
	if branch == "" {
		url = fmt.Sprintf("%s/api/off-all", r.Url)
	}
	resp, err := r.Client.Get(url)
	if err != nil {
		r.setStatus(Disconnect)
		return
	}
	if resp.StatusCode != http.StatusOK {
		r.setStatus(Disconnect)
		return
	}
	r.setStatus(Connecting)
}

func (r *Relay) Alarm(branch string) {
	host := fmt.Sprintf("%s/api/on/%s", r.Url, branch)
	if r.ResetTime != "" {
		host = fmt.Sprintf("%s/api/on-point/%s/%s000", r.Url, branch, r.ResetTime)
	}
	resp, err := r.Client.Get(host)
	if err != nil {
		r.setStatus(Disconnect)
		return
	}
	if resp.StatusCode != http.StatusOK {
		r.setStatus(Disconnect)
		return
	}
	r.setStatus(Connecting)
}

func (r *Relay) ping() {
	r.setStatus(Connecting)
	resp, err := r.Client.Get(fmt.Sprintf("%s%s", r.Url, ApiPing))
	if err != nil {
		r.setStatus(Disconnect)
		return
	}
	if resp.StatusCode != http.StatusOK {
		r.setStatus(Disconnect)
		return
	}
	r.setStatus(Connected)
}

func (r *Relay) Run() {
	r.Client = http.Client{
		Timeout: time.Second * 3,
	}
	id, err := r.Cron.AddFunc("* */1 * * * *", r.ping)
	if err != nil {
		return
	}
	r.CronId = id
}

func (r *Relay) Close() {
	r.locker.Lock()
	defer r.locker.Unlock()
	r.Cron.Remove(r.CronId)
}
