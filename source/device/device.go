package device

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/robfig/cron/v3"
	"log"
	"sync"
)

const (
	_ Type = iota
	TypeDTS
	TypeRelay

	_ EventType = iota
	EventError
	EventAdd
	EventRun
	EventUpdate
	EventClose
	EventDelete

	_ StatusType = iota
	Connecting
	Connected
	Disconnect
)

type (
	Type       byte
	EventType  byte
	StatusType byte

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
		Cron(*cron.Cron)
		Run()

		Close()
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
	cron    *cron.Cron

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
			cron:      cron.New(cron.WithSeconds()),
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
		device.Cron(m.cron)
		m.omit(EventListener{
			Device:    device,
			EventType: EventAdd,
		})
	}
}

func (m *Manger) Add(device Device) {
	m.devices.Store(device.GetId(), device)
	device.Cron(m.cron)
	m.omit(EventListener{
		Device:    device,
		EventType: EventAdd,
	})
}

func (m *Manger) Update(device Device) {
	m.devices.Store(device.GetId(), device)
	device.Cron(m.cron)
	m.omit(EventListener{
		Device:    device,
		EventType: EventUpdate,
	})
}

func (m *Manger) Delete(device Device) {
	m.devices.LoadAndDelete(device.GetId())
	m.omit(EventListener{
		Device:    device,
		EventType: EventDelete,
	})
}

func (m *Manger) WriteToWebsocket(connections ...*websocket.Conn) {
	status := m.GetStatus()
	for _, conn := range connections {
		err := conn.WriteJSON(status)
		if err != nil {
			log.Println(err)
			continue
		}
	}
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

func (m *Manger) omit(event EventListener) {
	select {
	case m.event <- event:
	default:
		log.Println("lost")
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
