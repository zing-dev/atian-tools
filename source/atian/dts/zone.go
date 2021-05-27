package dts

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Atian-OE/DTSSDK_Golang/dtssdk/model"
	"github.com/zing-dev/atian-tools/log"
	"math"
	"sort"
	"strings"
	"time"
)

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

func (s *Status) String() string {
	switch *s {
	case StatusOnline:
		return "在线"
	case StatusOff:
		return "离线"
	case StatusRetry:
		return "重连"
	}
	return "未知状态"
}

const (
	_ Status = iota
	StatusOnline
	StatusOff
	StatusRetry

	TagSeparator      = ";"
	TagValueSeparator = "="

	// TagWarehouse 仓库
	TagWarehouse = "warehouse" //示例 warehouse:w01
	// TagGroup 组
	TagGroup = "group" //示例 group:g001
	// TagRow 行
	TagRow = "row" //示例 row:1
	// TagColumn 列
	TagColumn = "column" //示例 column:1
	// TagLayer 层
	TagLayer = "layer" //示例 layer:1

	TagRelay = "relay" //示例 relay:A1,2,3,4
)

type (
	Status byte

	TimeLocal struct {
		time.Time
	}
	// Tag 标签 形如 k1=v1;k2=v2
	// 标签 warehouse=w1;group=g1;row=1;column=1;layer=1;relay=A1,2,3,4,5
	// Map {warehouse:w1,group:g1,row:1,column:1,layer:1,relay:A1,2,3,4,5}
	Tag map[string]string
	// Relay 继电器
	Relay map[uint8]string //标签 relay=A1,2,3,4,5 Map {A:1,2,3,4,5}

	// Temperature 温度信息
	Temperature struct {
		Max float32    `json:"max"`
		Avg float32    `json:"avg"`
		Min float32    `json:"min"`
		At  *TimeLocal `json:"at,omitempty"`
	}

	// Alarm 报警防区信息
	Alarm struct {
		Location  float32                `json:"location"`
		AlarmAt   *TimeLocal             `json:"alarm_at"`
		AlarmType model.DefenceAreaState `json:"alarm_type"`
	}

	BaseZone struct {
		Id        uint    `json:"id,omitempty"`
		Name      string  `json:"name,omitempty"`
		ChannelId byte    `json:"channel_id,omitempty"`
		Host      string  `json:"host,omitempty"`
		Start     float32 `json:"start,omitempty"`
		Finish    float32 `json:"finish,omitempty"`
		Tag       Tag     `json:"tag,omitempty"`
		Relay     Relay   `json:"relays,omitempty"`
	}

	// Zone 防区信息
	Zone struct {
		BaseZone
		*ZoneExtend
		*Temperature //防区温度详情
		*Alarm       //报警防区信息
	}

	// ZoneExtend 防区扩展信息
	ZoneExtend struct {
		Warehouse string `json:"warehouse,omitempty"`
		Group     string `json:"group,omitempty"`
		Row       int    `json:"row,omitempty"`
		Column    int    `json:"column,omitempty"`
		Layer     int    `json:"layer,omitempty"`
	}

	// Zones 防区集合
	Zones []*Zone

	// SortZones 排序防区集合
	SortZones Zones

	//lessFunc 比较函数
	lessFunc func(p1, p2 *Zone) bool

	multiSorter struct {
		zones SortZones
		less  []lessFunc
	}

	// ChannelZones  主机下的防区集合
	ChannelZones struct {
		ChannelId int32  `json:"channel_id,omitempty"`
		Host      string `json:"host,omitempty"`
		Zones     Zones
	}

	// ZonesTemp DTS所有防区温度
	ZonesTemp struct {
		DeviceId  string    `json:"device_id"`
		Host      string    `json:"host,omitempty"`
		CreatedAt TimeLocal `json:"created_at"`
		Zones     Zones     `json:"zones"`
	}

	// ZonesAlarm 报警防区信息集合
	ZonesAlarm struct {
		Zones     Zones      `json:"zones"`
		DeviceId  string     `json:"device_id"`
		Host      string     `json:"host,omitempty"`
		CreatedAt *TimeLocal `json:"created_at"`
	}

	// ChannelSignal DTS某一通道温度信号
	ChannelSignal struct {
		DeviceId   string     `json:"device_id"`
		ChannelId  int32      `json:"channel_id"`
		RealLength float32    `json:"real_length"`
		Host       string     `json:"host,omitempty"`
		Signal     []float32  `json:"signal"`
		CreatedAt  *TimeLocal `json:"created_at"`
	}

	// ChannelEvent  DTS某一通道事件
	ChannelEvent struct {
		Host          string           `json:"host,omitempty"`
		ChannelId     int32            `json:"channel_id"`
		DeviceId      string           `json:"device_id"`
		EventType     model.FiberState `json:"event_type"`
		ChannelLength float32          `json:"channel_length"`
		CreatedAt     *TimeLocal       `json:"created_at"`
	}
)

func (ms *multiSorter) Sort(zones SortZones) {
	ms.zones = zones
	sort.Sort(ms)
}

func OrderedBy(less ...lessFunc) *multiSorter {
	return &multiSorter{
		less: less,
	}
}

func (ms *multiSorter) Len() int {
	return len(ms.zones)
}

func (ms *multiSorter) Swap(i, j int) {
	ms.zones[i], ms.zones[j] = ms.zones[j], ms.zones[i]
}

func (ms *multiSorter) Less(i, j int) bool {
	p, q := &ms.zones[i], &ms.zones[j]
	var k int
	for k = 0; k < len(ms.less)-1; k++ {
		less := ms.less[k]
		switch {
		case less(*p, *q):
			return true
		case less(*q, *p):
			return false
		}
	}
	return ms.less[k](*p, *q)
}

func (t *Temperature) JSON() string {
	data, _ := json.Marshal(t)
	return string(data)
}

func (t *Temperature) String() string {
	return fmt.Sprintf("最大温度: %.3f ,最小温度: %.3f ,平均温度: %.3f", t.Max, t.Min, t.Avg)
}

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
	str += "]"
	return
}

func (t *ZonesTemp) JSON() string {
	data, _ := json.Marshal(t)
	return string(data)
}

// ChannelZones  防区以通道形式分组
func (zones SortZones) ChannelZones() []SortZones {
	var (
		channel []SortZones
		i, j    int
	)
	for {
		if i >= len(zones) {
			break
		}
		for j = i + 1; j < len(zones) && zones[i].ChannelId == zones[j].ChannelId; j++ {
		}
		channel = append(channel, zones[i:j])
		i = j
	}
	return channel
}

// ZoneMapSign 防区与信号的映射
func ZoneMapSign(sign []float32, start, end, scale float32) ([]float32, error) {
	if start > end {
		return nil, errors.New(fmt.Sprintf("开始位置 %.3f 大于终点位置 %.3f", start, end))
	}
	var s, e = 0, 0
	if math.Mod(float64(start), float64(scale)) == 0 {
		s = int(start / scale)
	} else {
		s = int(start/scale) + 1
	}
	if math.Mod(float64(end), float64(scale)) == 0 {
		e = int(end/scale) + 2
	} else {
		e = int(end/scale) + 1
	}
	if len(sign) < s || len(sign) < e {
		return nil, errors.New(fmt.Sprintf("开始位置 %.3f 索引 %d 或终点位置 %.3f 索引 %d 与温度信号长度 %d 映射失败", start, s, end, e, len(sign)))
	}
	return sign[s:e], nil
}

// Id 设备Id和防区Id绑定
func Id(deviceId, zoneId uint) uint {
	return deviceId*1e6 + zoneId
}
