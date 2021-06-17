package haosen

import (
	"encoding/xml"
	"fmt"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"github.com/zing-dev/atian-tools/log"
	"time"
)

const (
	Header = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"

	CMDAlarm        = "871001"
	CMDEvent        = "871002"
	CMDConfig       = "871003"
	CMDRealTimeTemp = "871004"

	MsgAlarm uint32 = iota + 1
	MsgEvent
	MsgRealTimeTemp
	MsgConfig

	GUIDFormat      = "20060102150405999"
	LocalTimeFormat = "2006-01-02 15:04:05"
)

//func DecodeUTF16(b []byte) ([]byte, error) {
//	r, _, err := transform.Bytes(unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder(), b)
//	return r, err
//}
//
//func EncodeUTF16(b []byte) ([]byte, error) {
//	r, _, err := transform.Bytes(unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder(), b)
//	return r, err
//}

type Response struct {
	XMLName   xml.Name `xml:"Setting"`
	CMD       string   `xml:"CMD"`       //871001，Command Code
	GUID      string   `xml:"GUID"`      //GUID，唯一标识符(yyyyMMddHHmmssfff)
	ErrorCode string   `xml:"ErrorCode"` //0000000 : Accepted Others : Error Code
	ErrorMsg  string   `xml:"ErrorMsg"`
	DeviceId  string   `xml:"deviceId"`  //设备序列号
	TimeStamp string   `xml:"timeStamp"` //报警发生时间，以 yyyy-MM-dd HH:mm:ss表示
}

type Zone struct {
	ZoneId      string `xml:"zoneId"`      //库位名称
	Line        string `xml:"line"`        //拉线号
	X           int    `xml:"x"`           //货架行号
	Y           int    `xml:"y"`           //货架列号
	Z           int    `xml:"z"`           //货架排号
	Temperature string `xml:"temperature"` //当前位置温度℃
}

type Alarm struct {
	XMLName xml.Name `xml:"alarm"`
	Zone
}

type Alarms struct {
	XMLName xml.Name `xml:"alarms>alarm"`
	Count   int      `xml:"Count,attr"` //有报警的库位数量
	Alarms  []Alarm
}

// AlarmRequest  温度报警信息
type AlarmRequest struct {
	XMLName   xml.Name `xml:"Setting"`
	CMD       string   `xml:"CMD"`       //871001，Command Code
	GUID      string   `xml:"GUID"`      //GUID，唯一标识符(yyyyMMddHHmmssfff)
	DeviceId  string   `xml:"deviceId"`  //设备序列号
	TimeStamp string   `xml:"timeStamp"` //报警发生时间，以 yyyy-MM-dd HH:mm:ss表示
	Alarms    Alarms
}

// EventRequest 设备事件
type EventRequest struct {
	XMLName   xml.Name `xml:"Setting"`
	CMD       string   `xml:"CMD"`       //871002，Command Code
	GUID      string   `xml:"GUID"`      //GUID，唯一标识符(yyyyMMddHHmmssfff)
	DeviceId  string   `xml:"deviceId"`  //设备序列号
	TimeStamp string   `xml:"timeStamp"` //报警发生时间，以 yyyy-MM-dd HH:mm:ss表示
	EventType string   `xml:"eventType"`
	ChannelId string   `xml:"channelId"` //设备通道
}

// ConfigRequest  上报配置
type ConfigRequest struct {
	XMLName          xml.Name `xml:"Setting"`
	CMD              string   `xml:"CMD"`               //871003，Command Code
	GUID             string   `xml:"GUID"`              //GUID，唯一标识符(yyyyMMddHHmmssfff)
	DeviceId         string   `xml:"deviceId"`          //设备序列号
	RealtimeInterval string   `xml:"realtime_interval"` //设置实时温度上报的时间间隔，单位秒；设置为0时，停止实时温度上报。
}

type Data struct {
	XMLName xml.Name `xml:"data"`
	Zone
}

type Datas struct {
	XMLName xml.Name `xml:"datas>data"`
	Count   int      `xml:"Count,attr"` //库位数量
	Datas   []Data
}

// RealTimeTempRequest  实时温度上报
type RealTimeTempRequest struct {
	XMLName   xml.Name `xml:"Setting"`
	CMD       string   `xml:"CMD"`       //871001，Command Code
	GUID      string   `xml:"GUID"`      //GUID，唯一标识符(yyyyMMddHHmmssfff)
	DeviceId  string   `xml:"deviceId"`  //设备序列号
	TimeStamp string   `xml:"timeStamp"` //报警发生时间，以 yyyy-MM-dd HH:mm:ss表示
	Datas     Datas
}

// AlarmRouter 报警路由
type AlarmRouter struct {
	znet.BaseRouter
}

// Handle  Handle
func (r *AlarmRouter) Handle(request ziface.IRequest) {
	zlog.Debug("Call AlarmRouter Handle")
	var (
		alarm    = new(AlarmRequest)
		response = &Response{
			CMD:       CMDAlarm,
			GUID:      time.Now().Format(GUIDFormat),
			TimeStamp: time.Now().Format(LocalTimeFormat),
		}
	)
	err := xml.Unmarshal(request.GetData(), alarm)
	if err != nil {
		response.ErrorCode = "1"
		response.ErrorMsg = fmt.Sprintf("解析上传数据异常: %s", err)
		response.DeviceId = alarm.DeviceId
	} else {
		response.ErrorCode = "0000000"
		response.ErrorMsg = "ok"
		response.DeviceId = alarm.DeviceId
	}
	data, err := xml.Marshal(response)
	if err != nil {
		log.L.Error("Marshal ", err)
		return
	}
	data = append([]byte(Header), data...)
	err = request.GetConnection().SendBuffMsg(MsgAlarm, data)
	if err != nil {
		zlog.Error(err)
	}
}

// EventRouter  报警路由
type EventRouter struct {
	znet.BaseRouter
}

// Handle  Handle
func (r *EventRouter) Handle(request ziface.IRequest) {
	zlog.Debug("Call EventRouter Handle")
	var (
		event    = new(EventRequest)
		response = &Response{
			CMD:       CMDEvent,
			GUID:      time.Now().Format(GUIDFormat),
			TimeStamp: time.Now().Format(LocalTimeFormat),
		}
	)
	err := xml.Unmarshal(request.GetData(), event)
	if err != nil {
		response.ErrorCode = "1"
		response.ErrorMsg = fmt.Sprintf("解析上传数据异常: %s", err)
		response.DeviceId = event.DeviceId
	} else {
		response.ErrorCode = "0000000"
		response.ErrorMsg = "ok"
		response.DeviceId = event.DeviceId
	}
	data, err := xml.Marshal(response)
	if err != nil {
		log.L.Error("Marshal ", err)
		return
	}
	data = append([]byte(Header), data...)
	err = request.GetConnection().SendBuffMsg(MsgEvent, data)
	if err != nil {
		zlog.Error(err)
	}
}

// ConfigRouter 配置路由
type ConfigRouter struct {
	znet.BaseRouter
}

// Handle  Handle
func (r *ConfigRouter) Handle(request ziface.IRequest) {
	zlog.Debug("Call ConfigRouter Handle")
	var (
		config   = new(ConfigRequest)
		response = &Response{
			CMD:       CMDConfig,
			GUID:      time.Now().Format(GUIDFormat),
			TimeStamp: time.Now().Format(LocalTimeFormat),
		}
	)
	err := xml.Unmarshal(request.GetData(), config)
	if err != nil {
		response.ErrorCode = "1"
		response.ErrorMsg = fmt.Sprintf("解析上传数据异常: %s", err)
		response.DeviceId = config.DeviceId
	} else {
		response.ErrorCode = "0000000"
		response.ErrorMsg = "ok"
		response.DeviceId = config.DeviceId
	}
	data, err := xml.Marshal(response)
	if err != nil {
		log.L.Error("Marshal ", err)
		return
	}
	data = append([]byte(Header), data...)
	err = request.GetConnection().SendBuffMsg(MsgConfig, data)
	if err != nil {
		zlog.Error(err)
	}
}

// RealTimeTempRouter  实时温度路由
type RealTimeTempRouter struct {
	znet.BaseRouter
}

// Handle 温度处理
func (r *RealTimeTempRouter) Handle(request ziface.IRequest) {
	zlog.Debug("Call RealtimeTempRouter Handle")
	var (
		temp     = new(RealTimeTempRequest)
		response = &Response{
			CMD:       CMDRealTimeTemp,
			GUID:      time.Now().Format(GUIDFormat),
			TimeStamp: time.Now().Format(LocalTimeFormat),
		}
	)

	err := xml.Unmarshal(request.GetData(), temp)
	if err != nil {
		response.ErrorCode = "1"
		response.ErrorMsg = fmt.Sprintf("解析上传数据异常: %s", err)
		response.DeviceId = temp.DeviceId
	} else {
		response.ErrorCode = "0000000"
		response.ErrorMsg = "ok"
		response.DeviceId = temp.DeviceId
	}
	data, err := xml.Marshal(response)
	if err != nil {
		log.L.Error("Marshal ", err)
		return
	}
	data = append([]byte(Header), data...)
	err = request.GetConnection().SendBuffMsg(MsgRealTimeTemp, data)
	if err != nil {
		zlog.Error(err)
	}
}
