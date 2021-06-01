package device

import (
	"context"
	"github.com/robfig/cron/v3"
	"github.com/zing-dev/atian-tools/log"
	"sync"
)

type (
	Status struct {
		Id     string     `json:"id"`
		Type   Type       `json:"type"`
		Status StatusType `json:"status"`
	}

	CallListener func(EventListener)

	EventListener struct {
		Device    Device
		EventType EventType
	}

	Device interface {
		GetId() string
		GetType() Type

		GetStatus() StatusType
		Run()

		Close()
		SetCron(*cron.Cron)
	}
)

var (
	once   = new(sync.Once)
	manger *Manger
)

type Manger struct {
	Context context.Context
	Cancel  context.CancelFunc
	devices sync.Map
	locker  sync.Mutex
	Cron    *cron.Cron

	listeners sync.Map
	event     chan EventListener
}

func NewManger(ctx context.Context) *Manger {
	once.Do(func() {
		ctx, cancel := context.WithCancel(ctx)
		manger = &Manger{
			Context:   ctx,
			Cancel:    cancel,
			devices:   sync.Map{},
			locker:    sync.Mutex{},
			Cron:      cron.New(cron.WithSeconds()),
			listeners: sync.Map{},
			event:     make(chan EventListener, 30),
		}
	})

	go manger.run()
	return manger
}

func (m *Manger) RegisterEvent(eventType EventType, lister CallListener) {
	m.listeners.Store(eventType, lister)
}

func (m *Manger) Adds(devices ...Device) {
	for _, device := range devices {
		m.devices.Store(device.GetId(), device)
		device.SetCron(m.Cron) //定时任务
		m.emit(EventListener{
			Device:    device,
			EventType: EventAdd,
		})
	}
}

func (m *Manger) Add(device Device) {
	m.devices.Store(device.GetId(), device)
	m.emit(EventListener{
		Device:    device,
		EventType: EventAdd,
	})
}

func (m *Manger) Update(device Device) {
	m.devices.Store(device.GetId(), device)
	m.emit(EventListener{
		Device:    device,
		EventType: EventUpdate,
	})
}

func (m *Manger) Delete(device Device) {
	value, ok := m.devices.LoadAndDelete(device.GetId())
	if ok {
		m.emit(EventListener{
			Device:    value.(Device),
			EventType: EventDelete,
		})
	}
}

func (m *Manger) GetDevice(id string) Device {
	if value, ok := m.devices.Load(id); ok {
		return value.(Device)
	}
	return nil
}

func (m *Manger) Range() []Device {
	devices := make([]Device, m.Length())
	i := 0
	m.devices.Range(func(key, value interface{}) bool {
		devices[i] = value.(Device)
		i++
		return true
	})
	return devices
}

func (m *Manger) GetStatus() []Status {
	status := make([]Status, m.Length())
	i := 0
	m.devices.Range(func(key, value interface{}) bool {
		device := value.(Device)
		status[i] = Status{
			Id:     device.GetId(),
			Type:   device.GetType(),
			Status: device.GetStatus(),
		}
		i++
		return true
	})
	return status
}

func (m *Manger) Length() (length int) {
	m.devices.Range(func(key, value interface{}) bool {
		length++
		return true
	})
	return
}

func (m *Manger) emit(event EventListener) {
	select {
	case m.event <- event:
	default:
		log.L.Error("event lost")
	}
}

func (m *Manger) run() {
	for {
		select {
		case event := <-m.event:
			if call, ok := manger.listeners.Load(event.EventType); ok {
				go call.(CallListener)(event)
			}
		case <-m.Context.Done():
			return
		}
	}
}
