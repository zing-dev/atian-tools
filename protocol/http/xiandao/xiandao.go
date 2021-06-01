package xiandao

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zing-dev/atian-tools/protocol/common"
	"io"
	"net/http"
)

// Request 报警请求
type Request struct {
	LocationCode string `json:"locationCode"` //库位编码
}

// Response 服务端响应
type Response struct {
	Code int    `json:"code"` // 0 成功,大于0 失败
	Msg  string `json:"msg"`
}

type HTTP struct {
	URL    string
	Client http.Client
}

// Post 发送POST请求
func (h *HTTP) Post(request Request) (*Response, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := h.Client.Post(h.URL, common.ContentTypeJson, bytes.NewBuffer(data))
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
