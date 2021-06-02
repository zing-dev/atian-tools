package haosen

import (
	"github.com/aceld/zinx/utils"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

type Server struct {
	ziface.IServer
}

func NewServer() *Server {
	//return &Server{
	//	IServer: &znet.Server{
	//		Name:      "test",
	//		IPVersion: "tcp4",
	//		IP:        "192.168.0.251",
	//		Port:      9090,
	//		ConnMgr:   znet.NewConnManager(),
	//	},
	//}
	utils.GlobalObject.Name = "test"
	utils.GlobalObject.Host = "192.168.0.251"
	utils.GlobalObject.TcpPort = 9090
	return &Server{IServer: znet.NewServer()}
}

func (s *Server) Run() {

	s.AddRouter(MsgAlarm, &AlarmRouter{})
	s.AddRouter(MsgEvent, &EventRouter{})
	s.AddRouter(MsgConfig, &ConfigRouter{})
	s.AddRouter(MsgRealTimeTemp, &RealTimeTempRouter{})

	s.Serve()
}
