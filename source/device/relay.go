package device

import (
	"context"
	"fmt"
	"github.com/robfig/cron/v3"
	"net/http"
	"strconv"
	"strings"
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
	URL       string
	Cron      *cron.Cron
	CronId    cron.EntryID
	locker    sync.Mutex
	status    StatusType
}

func NewRelay(ctx context.Context, tag, url, reset string) *Relay {
	ctx, cancel := context.WithCancel(ctx)
	return &Relay{
		ctx:       ctx,
		cancel:    cancel,
		ResetTime: reset,
		Client:    http.Client{Timeout: time.Second * 3},
		Tag:       tag,
		URL:       url,
		status:    UnConnect,
	}
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
	url := fmt.Sprintf("%s/api/off/%s", r.URL, branch)
	if branch == "" {
		url = fmt.Sprintf("%s/api/off-all", r.URL)
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
	r.setStatus(Connected)
}

func (r *Relay) Alarms(branch string) {
	for _, b := range strings.Split(branch, ",") {
		i, err := strconv.Atoi(b)
		if err != nil {
			continue
		}
		if i <= 0 || i > 32 {
			continue
		}
		go r.Alarm(b)
	}
}

func (r *Relay) Alarm(branch string) {
	host := fmt.Sprintf("%s/api/on/%s", r.URL, branch)
	if r.ResetTime != "" {
		host = fmt.Sprintf("%s/api/on-point/%s/%s000", r.URL, branch, r.ResetTime)
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
	r.setStatus(Connected)
}

func (r *Relay) ping() {
	r.setStatus(Connecting)
	resp, err := r.Client.Get(fmt.Sprintf("%s%s", r.URL, ApiPing))
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

func (r *Relay) Run() error {
	r.ping()
	id, err := r.Cron.AddFunc("0 */1 * * * *", r.ping)
	if err != nil {
		return err
	}
	r.CronId = id
	return nil
}

func (r *Relay) Close() error {
	r.cancel()
	r.Cron.Remove(r.CronId)
	r.setStatus(UnConnect)
	return nil
}
