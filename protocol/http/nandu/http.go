package nandu

import (
	"atian.tools/log"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	CodeAlarm = 1
	CodePing  = 99

	ContentTypeJson = "application/json;charset=UTF-8"
)

type Request struct {
	LocationCode string `json:"locationCode"`
	Status       int    `json:"status"`
}

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (r *Response) String() string {
	return fmt.Sprintf("状态码: %d,结果: %s", r.Code, r.Msg)
}

type HTTP struct {
	ctx    context.Context
	cancel context.CancelFunc
	Url    string
	Client http.Client
}

func New(ctx context.Context, url string) *HTTP {
	ctx, cancel := context.WithCancel(ctx)
	return &HTTP{
		ctx:    ctx,
		cancel: cancel,
		Url:    url,
		Client: http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (h *HTTP) Ping() {
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
			response, err := h.Send(Request{
				LocationCode: "",
				Status:       CodePing,
			})
			if err != nil {
				log.L.Error(fmt.Sprintf("发送心跳错误: %s", err))
				return
			}
			log.L.Info("心跳: ", response.String())
			time.Sleep(time.Second * 30)
		}
	}
}

func (h *HTTP) Send(request Request) (*Response, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	response, err := h.Client.Post(h.Url, ContentTypeJson, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("状态码返回异常: %s", response.Status))
	}

	data, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	r := new(Response)
	err = json.Unmarshal(data, r)
	if err != nil {
		return nil, err
	}
	return r, nil
}
