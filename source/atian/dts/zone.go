package dts

import (
	"atian.tools/log"
	"database/sql/driver"
	"encoding/json"
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

func GetAlarmTypeString(t model.DefenceAreaState) string {
	switch t {
	case model.DefenceAreaState_Normal:
		return "状态正常"
	case model.DefenceAreaState_WarnDiffer:
		return "温差预警"
	case model.DefenceAreaState_WarnUp:
		return "温升预警"
	case model.DefenceAreaState_WarnTemp:
		return "定温预警"
	case model.DefenceAreaState_AlarmDiffer:
		return "温差报警"
	case model.DefenceAreaState_AlarmUp:
		return "温升报警"
	case model.DefenceAreaState_AlarmTemp:
		return "定温报警"
	case model.DefenceAreaState_WarnLowTemp:
		return "低温预警"
	case model.DefenceAreaState_AlarmLowTemp:
		return "低温报警"
	default:
		return "非法的防区状态"
	}
}

func GetEventTypeString(t model.FiberState) string {
	switch t {
	case model.FiberState_SSTATEISOK:
		return "光纤正常"
	case model.FiberState_SSTATUSUNFIN:
		return "光纤拔出"
	case model.FiberState_SSTATUSFIN:
		return "光纤插入"
	case model.FiberState_SSTATUSBRK:
		return "光纤断裂"
	case model.FiberState_SSTATUSTLO:
		return "光纤过长"
	case model.FiberState_SSTATUSLTM:
		return "光纤损耗过多"
	default:
		return "非法的光纤状态"
	}
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

type (
	// Tag 标签
	Tag map[string]string
	// Relay 继电器
	Relay map[uint8]string

	// Temperature 温度信息
	Temperature struct {
		Max float32    `json:"max"`
		Avg float32    `json:"avg"`
		Min float32    `json:"min"`
		At  *TimeLocal `json:"at,omitempty"`
	}

	// Zone 防区信息
	Zone struct {
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

	// ZoneExtend 防区扩展信息
	ZoneExtend struct {
		Warehouse string `json:"warehouse,omitempty"`
		Group     string `json:"group,omitempty"`
		Row       int    `json:"row,omitempty"`
		Column    int    `json:"column,omitempty"`
		Layer     int    `json:"layer,omitempty"`
	}

	// Zones 主机下的防区集合
	Zones struct {
		ChannelId int32  `json:"channel_id,omitempty"`
		Host      string `json:"host,omitempty"`
		Zones     []*Zone
	}

	// ZoneTemp 防区温度详情
	ZoneTemp struct {
		*Zone
		Temperature
	}

	// ZonesTemp DTS所有防区温度
	ZonesTemp struct {
		DeviceId  string     `json:"device_id"`
		Host      string     `json:"host,omitempty"`
		CreatedAt TimeLocal  `json:"created_at"`
		Zones     []ZoneTemp `json:"zones"`
	}

	// ZoneAlarm 报警防区信息
	ZoneAlarm struct {
		*Zone
		Temperature
		Location  float32                `json:"location"`
		AlarmAt   TimeLocal              `json:"alarm_at"`
		AlarmType model.DefenceAreaState `json:"alarm_type"`
	}

	// ZonesAlarm 报警防区信息集合
	ZonesAlarm struct {
		Zones     []ZoneAlarm `json:"zones"`
		DeviceId  string      `json:"device_id"`
		Host      string      `json:"host,omitempty"`
		CreatedAt TimeLocal   `json:"created_at"`
	}

	// ChannelSignal DTS某一通道温度信号
	ChannelSignal struct {
		DeviceId   string    `json:"device_id"`
		ChannelId  int32     `json:"channel_id"`
		RealLength float32   `json:"real_length"`
		Host       string    `json:"host,omitempty"`
		Signal     []float32 `json:"signal"`
		CreatedAt  TimeLocal `json:"created_at"`
	}

	// ChannelEvent  DTS某一通道事件
	ChannelEvent struct {
		Host          string           `json:"host,omitempty"`
		ChannelId     int32            `json:"channel_id"`
		DeviceId      string           `json:"device_id"`
		EventType     model.FiberState `json:"event_type"`
		ChannelLength float32          `json:"channel_length"`
		CreatedAt     TimeLocal        `json:"created_at"`
	}
)

func (z *Zone) JSON() string {
	data, _ := json.Marshal(z)
	return string(data)
}

func (z *Zone) String() (str string) {
	str = "Zone ["
	str += fmt.Sprintf("Id: %d ,", z.Id)
	str += fmt.Sprintf("名称: %s ,", z.Name)
	str += fmt.Sprintf("主机: %s ,", z.Host)
	str += fmt.Sprintf("通道: %d ,", z.ChannelId)
	str += fmt.Sprintf("开始位置: %.3f ,", z.Start)
	str += fmt.Sprintf("结束位置: %.3f ,", z.Finish)
	str += "标签: "
	for k, v := range z.Tag {
		str += fmt.Sprintf("[ %s : %s ],", k, v)
	}
	str = "]"
	return
}

func (t *ZonesTemp) JSON() string {
	data, _ := json.Marshal(t)
	return string(data)
}
