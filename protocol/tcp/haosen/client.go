package haosen

import (
	"context"
	"encoding/xml"
	"github.com/aceld/zinx/znet"
	"github.com/zing-dev/atian-tools/source/device"
	"io"
	"net"
)

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
	Conn   net.Conn
	Host   string
	status device.StatusType
}

func NewClient(ctx context.Context, host string) *Client {
	ctx, cancel := context.WithCancel(ctx)
	return &Client{
		ctx:    ctx,
		cancel: cancel,
		Host:   host,
		status: device.UnConnect,
	}
}

func (c *Client) Connect() error {
	c.status = device.Connecting
	conn, err := net.Dial("tcp", c.Host)
	if err != nil {
		c.status = device.UnConnect
		return err
	}
	c.status = device.Connected
	c.Conn = conn
	return err
}

func (c *Client) Send(id uint32, message interface{}) (*Response, error) {
	if c.status != device.Connected {
		return nil, nil
	}
	data, err := xml.Marshal(message)
	if err != nil {
		return nil, err
	}

	pack := znet.NewDataPack()
	data, err = pack.Pack(znet.NewMsgPackage(id, data))
	if err != nil {
		return nil, err
	}

	_, err = c.Conn.Write(data)
	if err != nil {
		return nil, err
	}

	//先读出流中的head部分
	headData := make([]byte, pack.GetHeadLen())
	_, err = io.ReadFull(c.Conn, headData) //ReadFull 会把msg填充满为止
	if err != nil {
		return nil, err
	}
	//将headData字节流 拆包到msg中
	msgHead, err := pack.Unpack(headData)
	if err != nil {
		return nil, err
	}

	if msgHead.GetDataLen() > 0 {
		//msg 是有data数据的，需要再次读取data数据
		msg := msgHead.(*znet.Message)
		msg.Data = make([]byte, msg.GetDataLen())

		//根据dataLen从io中读取字节流
		_, err := io.ReadFull(c.Conn, msg.Data)
		if err != nil {
			return nil, err
		}

		r := new(Response)
		err = xml.Unmarshal(msg.Data, r)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
	return nil, err
}

func (c *Client) Close() error {
	c.cancel()
	return nil
}
