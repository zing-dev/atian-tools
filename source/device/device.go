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

	Listener func(Device)

	Event struct {
		Device    Device
		EventType EventType
	}

	Device interface {
		GetId() string
		GetType() Type

		GetStatus() StatusType
		Run() error

		Close() error
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
	event     chan Event
}

// NewManger 实例化设备管理器
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
			event:     make(chan Event, 30),
		}
	})

	go manger.run()
	return manger
}

// Register 注册事件
func (m *Manger) Register(eventType EventType, lister Listener) {
	m.listeners.Store(eventType, lister)
}

// Adds 批量添加设备
func (m *Manger) Adds(devices ...Device) {
	for _, device := range devices {
		m.devices.Store(device.GetId(), device)
		device.SetCron(m.Cron) //定时任务
		m.emit(Event{
			Device:    device,
			EventType: EventAdd,
		})
	}
}

// Add 添加设备
func (m *Manger) Add(device Device) {
	m.devices.Store(device.GetId(), device)
	device.SetCron(m.Cron) //定时任务
	m.emit(Event{
		Device:    device,
		EventType: EventAdd,
	})
}

// Run 运行设备
func (m *Manger) Run(id string) error {
	if value, ok := m.devices.Load(id); ok {
		m.emit(Event{
			Device:    value.(Device),
			EventType: EventRun,
		})
		return nil
	}
	return NotFoundDeviceError
}

// Update 更新设备
func (m *Manger) Update(device Device) {
	m.devices.Store(device.GetId(), device)
	device.SetCron(m.Cron) //定时任务
	m.emit(Event{
		Device:    device,
		EventType: EventUpdate,
	})
}

// Close 关闭设备
func (m *Manger) Close(id string) error {
	if value, ok := m.devices.Load(id); ok {
		m.emit(Event{
			Device:    value.(Device),
			EventType: EventClose,
		})
		return nil
	}
	return NotFoundDeviceError
}

// Delete 根据 id 删除设备
func (m *Manger) Delete(id string) error {
	if value, ok := m.devices.LoadAndDelete(id); ok {
		m.emit(Event{
			Device:    value.(Device),
			EventType: EventDelete,
		})
		return nil
	}
	return NotFoundDeviceError
}

// GetDevice 根据 id 获取设备
func (m *Manger) GetDevice(id string) Device {
	if value, ok := m.devices.Load(id); ok {
		return value.(Device)
	}
	return nil
}

// Range 遍历设备
func (m *Manger) Range(f func(string, Device)) {
	m.devices.Range(func(key, value interface{}) bool {
		f(key.(string), value.(Device))
		return true
	})
}

// Devices 数组形式获取设备
func (m *Manger) Devices() []Device {
	devices := make([]Device, m.Length())
	i := 0
	m.devices.Range(func(key, value interface{}) bool {
		devices[i] = value.(Device)
		i++
		return true
	})
	return devices
}

// GetStatus 获取设备的状态
func (m *Manger) GetStatus() []Status {
	m.locker.Lock()
	defer m.locker.Unlock()
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

// Length 获取设备的数量
func (m *Manger) Length() (length int) {
	m.Range(func(s string, device Device) {
		length++
	})
	return
}

func (m *Manger) emit(event Event) {
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
				go call.(Listener)(event.Device)
			}
		case <-m.Context.Done():
			return
		}
	}
}
