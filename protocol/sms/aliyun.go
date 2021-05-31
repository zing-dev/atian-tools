package sms

import (
	"encoding/json"
	"errors"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/responses"
	"github.com/zing-dev/atian-tools/log"
	"time"
)

type AliConfig struct {
	SMSMinute    byte     `json:"sms_minute"`
	SMSPhones    []string `json:"sms_phones"`
	AccessKey    string   `json:"access_key"`
	AccessSecret string   `json:"access_secret"`
	SignName     string   `json:"sign_name"`
	TemplateCode string   `json:"template_code"`
}

type SMS struct {
	AliConfig
	Interval          map[string]time.Time
	templateParamJson string
	interval          time.Duration
}

func NewSMS(config AliConfig) *SMS {
	return &SMS{
		interval:  time.Duration(config.SMSMinute) * time.Minute,
		AliConfig: config,
		Interval:  map[string]time.Time{},
	}
}

func (s *SMS) Send(t string, phones []string, params []map[string]string) error {
	start, ok := s.Interval[t]
	if !ok || (ok && time.Now().Sub(start) > s.interval) {
		ps, err := json.Marshal(phones)
		if err != nil {
			return err
		}
		s.interval = time.Duration(s.SMSMinute) * time.Minute
		data, err := json.Marshal(params)
		if err != nil {
			return err
		}
		s.templateParamJson = string(data)
		client, err := sdk.NewClientWithAccessKey("default", s.AccessKey, s.AccessSecret)
		if err != nil {
			return err
		}
		request := requests.NewCommonRequest()                         // 构造一个公共请求
		request.Method = "POST"                                        // 设置请求方式
		request.Domain = "dysmsapi.aliyuncs.com"                       // 指定域名则不会寻址，如认证方式为 Bearer Token 的服务则需要指定
		request.Version = "2017-05-25"                                 // 指定产品版本
		request.ApiName = "SendBatchSms"                               // 指定接口名
		request.QueryParams["RegionId"] = "cn-hangzhou"                // 指定请求的区域，不指定则使用客户端区域、默认区域
		request.QueryParams["PhoneNumberJson"] = string(ps)            //
		request.QueryParams["SignNameJson"] = s.SignName               //
		request.QueryParams["TemplateCode"] = s.TemplateCode           //
		request.QueryParams["templateParamJson"] = s.templateParamJson //
		request.TransToAcsRequest()                                    // 把公共请求转化为acs请求

		response := responses.NewCommonResponse()
		err = client.DoAction(request, response)
		if err != nil {
			return err
		}
		var result map[string]string
		err = json.Unmarshal([]byte(response.GetHttpContentString()), &result)
		if err != nil {
			return err
		}
		if result["Message"] == "OK" {
			log.L.Info("发送报警短信成功:", s.templateParamJson)
			s.Interval[t] = time.Now()
			return nil
		}
		return errors.New(result["Message"])
	}
	return errors.New("发送短信时间间隔较短")
}
