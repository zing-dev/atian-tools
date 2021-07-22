package xiandao

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zing-dev/atian-tools/log"
	"github.com/zing-dev/atian-tools/protocol/common"
	"io"
	"net/http"
	"time"
)

// Request 报警请求
type Request struct {
	LocationCode string `json:"locationCode"` //库位编码
	Status       int    `json:"status"`       //1报警 99心跳
}

// Response 服务端响应
type Response struct {
	Code int    `json:"code"` // 0 成功,大于0 失败
	Msg  string `json:"msg"`
}

type HTTP struct {
	ctx    context.Context
	cancel context.CancelFunc
	URL    string
	Client http.Client
}

func New(ctx context.Context, url string) *HTTP {
	ctx, cancel := context.WithCancel(ctx)
	h := &HTTP{
		ctx:    ctx,
		cancel: cancel,
		URL:    url,
		Client: http.Client{Timeout: time.Second * 2},
	}
	go h.ping()
	return h
}

// Post 发送POST请求
func (h *HTTP) Post(request Request) (*Response, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	log.L.Info(fmt.Sprintf("%s?locationCode=%s&status=%d", h.URL, request.LocationCode, request.Status))
	resp, err := h.Client.Post(fmt.Sprintf("%s?locationCode=%s&status=%d", h.URL, request.LocationCode, request.Status), common.ContentTypeJson, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("POST响应状态码不是200")
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("读取响应返回值失败: %s", err))
	}

	r := new(Response)
	err = json.Unmarshal(data, r)
	return r, err
}

func (h *HTTP) ping() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-time.After(time.Second * 30):
			response, err := h.Post(Request{LocationCode: "", Status: 99})
			if err != nil {
				log.L.Error("心跳失败: ", err)
				continue
			}

			if response.Code != 0 {
				log.L.Error(fmt.Sprintf("心跳失败,返回数据: %s", response.Msg))
				continue
			}

			log.L.Info("发送心跳成功")
		}
	}
}
