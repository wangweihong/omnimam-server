package dingtalk

import (
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/hash"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
)



type Message struct {
	MsgType  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown"`
}


func Send(message Message, dingTalkRobotURL string, secret string, key string) error {
	if key != "" {
		message.Markdown.Title += key
	}
	if secret != "" {
		now := time.Now().UnixNano() / 1000000
		stringToSign := fmt.Sprintf("%v\n%v", now, secret)
		signData, _ := hash.NewSha256().HmacSum(stringToSign, secret)
		sign := url.QueryEscape(string(signData))
		dingTalkRobotURL = fmt.Sprintf("%v&timestamp=%v&sign=%v", dingTalkRobotURL, now, sign)
	}

	httpResp, err := httpcli.NewHttpRequestBuilder().
		POST().
		WithEndpoint(dingTalkRobotURL).
		WithBody("", message).Build().Invoke()
	if err != nil {
		return errors.WithStack(err)
	}
	ret := Result{}
	if err := httpResp.Decode(&ret); err != nil {
		return errors.WithStack(err)
	}
	if ret.ErrorCode != 0 {
		return errors.Errorf("invoke %v error,errorcode:%v, errorMEssage:%v",
			dingTalkRobotURL, ret.ErrorCode, ret.ErrorMessage)
	}
	return nil
}
