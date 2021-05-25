package websocket

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/kataras/neffos"
	"log"
)

type Response struct {
	Success bool        `json:"success"`
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
}

var (
	server *neffos.Server
)

func Register(s *neffos.Server)  {
	server = s
}

func WriteToWebsockets(t string, data interface{}) {
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
	for _, conn := range server.GetConnections() {
		ok := conn.Write(neffos.Message{
			Body:     body,
			IsNative: true,
		})
		if !ok {
			log.Println(err)
			continue
		}
	}
}

func WriteToWebsocket(t string, data interface{}, connections ...*websocket.Conn) {
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
