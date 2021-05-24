package device

import (
	"context"
	"github.com/robfig/cron/v3"
	"golang.org/x/tools/go/ssa/interp/testdata/src/fmt"
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
	id     string
	ctx    context.Context
	cancel context.CancelFunc

	ResetTime string
	Client    http.Client
	Url       string
	Cron      *cron.Cron
	CronId    cron.EntryID
	locker    sync.Mutex
	status    StatusType
}

func (r *Relay) GetId() string {
	return fmt.Sprintf("relay-%s", r.Url)
}

func (r *Relay) GetType() Type {
	return TypeRelay
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
