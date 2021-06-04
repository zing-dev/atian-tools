package websocket

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/kataras/neffos"
	"github.com/zing-dev/atian-tools/source/device"
	"log"
)

const (
	_ Type = iota
	TypeLog
	TypeAlarm
	TypeTemp
	TypeChannelSign
	TypeEvent
	TypeDevice
)

type Type byte

func (t *Type) String() string {
	switch *t {
	case TypeLog:
		return "日志"
	case TypeAlarm:
		return "报警"
	case TypeTemp:
		return "防区温度"
	case TypeChannelSign:
		return "通道温度信号"
	case TypeEvent:
		return "通道光纤事件"
	case TypeDevice:
		return "设备"
	default:
		return "未知"
	}
}

func GetWebsocketTypeMap() []device.Constant {
	t := []Type{TypeLog, TypeAlarm, TypeTemp, TypeChannelSign, TypeEvent, TypeDevice}
	constant := make([]device.Constant, len(t))
	for i, state := range t {
		constant[i] = device.Constant{Name: state.String(), Value: byte(state)}
	}
	return constant
}

type Response struct {
	Success bool        `json:"success"`
	Type    Type        `json:"type"`
	Data    interface{} `json:"data"`
}

var (
	server *neffos.Server
)

func Register(s *neffos.Server) {
	server = s
}

func Send(body []byte, server *neffos.Server) {
	server.Broadcast(nil, neffos.Message{Body: body, IsNative: true})
}

func Write(data interface{}, server *neffos.Server) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return
	}
	Send(body, server)
}

func WriteToWebsockets(t Type, data interface{}) {
	if server == nil {
		return
	}
	body, err := json.Marshal(Response{
		Success: true,
		Type:    t,
		Data:    data,
	})
	if err != nil {
		log.Println(err)
		return
	}
	Send(body, server)
}

func WriteToWebsocket(t Type, data interface{}, connections ...*websocket.Conn) {
	for _, conn := range connections {
		err := conn.WriteJSON(Response{
			Success: true,
			Type:    t,
			Data:    data,
		})
		if err != nil {
			log.Println(err)
			continue
		}
	}
}
