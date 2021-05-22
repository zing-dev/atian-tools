package dts

import (
	"atian.tools/log"
	"database/sql/driver"
	"fmt"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk/model"
	"strings"
	"time"
)

type TimeLocal struct {
	time.Time
}

func (t TimeLocal) MarshalJSON() ([]byte, error) {
	return []byte(t.Format(`"2006-01-02 15:04:05"`)), nil
}

func (t *TimeLocal) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var err error
	t.Time, err = time.Parse(`"`+"2006-01-02 15:04:05"+`"`, string(data))
	return err
}

func (t TimeLocal) Value() (driver.Value, error) {
	var zeroTime time.Time
	if t.Time.UnixNano() == zeroTime.UnixNano() {
		return nil, nil
	}
	return t.Time, nil
}

func (t *TimeLocal) Scan(v interface{}) error {
	value, ok := v.(time.Time)
	if ok {
		*t = TimeLocal{Time: value}
		return nil
	}
	return fmt.Errorf("can not convert %v to timestamp", v)
}

// DecodeTags 解析标签
func DecodeTags(tag string) (res map[string]string) {
	res = map[string]string{}
	if strings.HasSuffix(tag, TagSeparator) {
		tag = tag[:len(tag)-1]
	}
	for _, v := range strings.Split(tag, TagSeparator) {
		value := strings.Split(v, TagValueSeparator)
		if len(value) != 2 {
			log.L.Error(fmt.Sprintf("解析 %s 失败,模式不匹配 k=v", value))
			continue
		}
		res[value[0]] = value[1]
	}
	return
}

const (
	TagSeparator      = ";"
	TagValueSeparator = "="

	TagWarehouse = "warehouse"
	TagGroup     = "group"
	TagRow       = "row"
	TagColumn    = "column"
	TagLayer     = "layer"

	TagRelay = "relay"
)

type Temperature struct {
	Max float32    `json:"max"`
	Avg float32    `json:"avg"`
	Min float32    `json:"min"`
	At  *TimeLocal `json:"at,omitempty"`
}

type ZoneAlarm struct {
	*Zone
	Temperature
	Location  float32                `json:"location"`
	AlarmAt   TimeLocal              `json:"alarm_at"`
	AlarmType model.DefenceAreaState `json:"alarm_type"`
}

type ZonesAlarm struct {
	Zones     []ZoneAlarm `json:"zones"`
	DeviceId  string      `json:"device_id"`
	Host      string      `json:"host,omitempty"`
	CreatedAt string      `json:"created_at"`
}

type ZoneExtend struct {
	Warehouse string `json:"warehouse,omitempty"`
	Group     string `json:"group,omitempty"`
	Row       int    `json:"row,omitempty"`
	Column    int    `json:"column,omitempty"`
	Layer     int    `json:"layer,omitempty"`
}

type Tag map[string]string

type Relay map[uint8]string

type Zone struct {
	Id        uint    `json:"id,omitempty"`
	Name      string  `json:"name,omitempty"`
	ChannelId byte    `json:"channel_id,omitempty"`
	Host      string  `json:"host,omitempty"`
	Start     float32 `json:"start,omitempty"`
	Finish    float32 `json:"finish,omitempty"`
	Tag       Tag     `json:"tag,omitempty"`
	Relay     Relay   `json:"relays,omitempty"`
	ZoneExtend
}

type Zones struct {
	ChannelId int32  `json:"channel_id,omitempty"`
	Host      string `json:"host,omitempty"`
	Zones     []*Zone
}

type ZoneTemp struct {
	Zone
	Temperature
}

type ZonesTemp struct {
	DeviceId  string     `json:"device_id"`
	Host      string     `json:"host,omitempty"`
	CreatedAt string     `json:"created_at"`
	Zones     []ZoneTemp `json:"zones"`
}

type ChannelSignal struct {
	DeviceId   string    `json:"device_id"`
	ChannelId  int32     `json:"channel_id"`
	RealLength float32   `json:"real_length"`
	Host       string    `json:"host,omitempty"`
	Signal     []float32 `json:"signal"`
	CreatedAt  string    `json:"created_at"`
}

type ChannelEvent struct {
	Host          string           `json:"host,omitempty"`
	ChannelId     int32            `json:"channel_id"`
	DeviceId      string           `json:"device_id"`
	EventType     model.FiberState `json:"event_type"`
	ChannelLength float32          `json:"channel_length"`
	CreatedAt     string           `json:"created_at"`
}
