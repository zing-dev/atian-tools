package device

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	ColorDefault = "default"
	ColorPrimary = "primary"
	ColorSuccess = "success"
	ColorInfo    = "info"
	ColorWarning = "warning"
	ColorDanger  = "danger"

	// LocalDateTimeFormat 基于中国的时间格式化模板
	LocalDateTimeFormat = "2006-01-02 15:04:05"
	LocalDateFormat     = "2006-01-02"
	LocalTimeFormat     = "15:04:05"
)

const (
	_         Type = iota
	TypeDTS        //DTS
	TypeRelay      //继电器
	TypeApi        //HTTP API 接口
)

const (
	_ EventType = iota
	EventError
	EventAdd
	EventRun
	EventUpdate
	EventClose
	EventDelete
)
const (
	_          StatusType = iota
	UnConnect             //未连接
	Connecting            //连接中
	Connected             //已连接
	Disconnect            //已断开
)

func (s *Type) String() string {
	switch *s {
	case TypeDTS:
		return "DTS"
	case TypeRelay:
		return "继电器"
	case TypeApi:
		return "API"
	default:
		return "未知设备"
	}
}

func (s *StatusType) String() string {
	switch *s {
	case UnConnect:
		return "未连接"
	case Connecting:
		return "连接中"
	case Connected:
		return "已连接"
	case Disconnect:
		return "已断开"
	default:
		return "未知状态"
	}
}

func GetConnectMap() []Constant {
	a1, a2, a3, a4 := UnConnect, Connecting, Connected, Disconnect
	return []Constant{
		{Name: a1.String(), Value: byte(UnConnect), Color: ColorWarning},
		{Name: a2.String(), Value: byte(Connecting), Color: ColorDanger},
		{Name: a3.String(), Value: byte(Connected), Color: ColorPrimary},
		{Name: a4.String(), Value: byte(Disconnect), Color: ColorDanger},
	}
}

func GetDeviceMap() []Constant {
	a1, a2, a3 := TypeDTS, TypeRelay, TypeApi
	return []Constant{
		{Name: a1.String(), Value: byte(TypeDTS), Color: ColorPrimary},
		{Name: a2.String(), Value: byte(TypeRelay), Color: ColorPrimary},
		{Name: a3.String(), Value: byte(TypeApi), Color: ColorPrimary},
	}
}

func (t TimeLocal) MarshalJSON() ([]byte, error) {
	return []byte(t.Format(fmt.Sprintf(`"%s"`, LocalDateTimeFormat))), nil
}

func (t *TimeLocal) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var err error
	t.Time, err = time.Parse(fmt.Sprintf(`"%s"`, LocalDateTimeFormat), string(data))
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

func (t TimeLocal) String() string {
	return t.Format(LocalDateTimeFormat)
}

type (
	Type       byte
	EventType  byte
	StatusType byte

	Message struct {
		Msg   string       `json:"msg"`
		Level logrus.Level `json:"level"`
		At    TimeLocal    `json:"at"`
	}

	TimeLocal struct {
		time.Time
	}

	Constant struct {
		Name   string `json:"name"`
		Value  byte   `json:"value"`
		Color  string `json:"color,omitempty"`
		Commit string `json:"commit,omitempty"`
	}
)

var (
	NotFoundDeviceError = errors.New("未找到当前设备")
)
