package dingtalk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
)

// https://open.dingtalk.com/document/orgapp/message-types-and-data-format
const (
	// 文本消息
	MessageTypeText = "text"
	// 图片消息
	MessageTypeImage = "image"
	// 语音消息
	MessageTypeVoice = "voice"
	// 文件消息
	MessageTypeFile = "file"
	// 链接消息
	MessageTypeLink = "link"
	// OA消息
	MessageTypeOA = "oa"
	// Markdown消息
	MessageTypeMarkdown = "markdown"
	// 卡片消息
	MessageTypeCard = "action_card"
)

type Text struct {
	Content string `json:"content"`
}

type Image struct {
	MediaID string `json:"media_id"`
}

type Voice struct {
	MediaID  string `json:"media_id"`
	Duration string `json:"duration"`
}

type File struct {
	MediaID string `json:"media_id"`
}

type Markdown struct {
	Title string `json:"title"`
	// https://jenkinsci.github.io/dingtalk-plugin/advance/markdown.html
	// 除了正常markdown语法子集,可以通过特殊标签给文本加上颜色、字体等效果
	// 如<font color=red size=3 >红色-正常大小文字</font>
	Text string `json:"text"`
}

type Link struct {
	MessageURL string `json:"messageUrl"`
	PictureURL string `json:"picUrl"`
	Title      string `json:"title"`
	Text       string `json:"text"`
}

type OA struct {
	MessageURL string `json:"message_url"`
	Head       struct {
		Bgcolor string `json:"bgcolor"`
		Text    string `json:"text"`
	} `json:"head"`
	Body struct {
		Title string `json:"title"`
		Form  []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"form"`
		Rich struct {
			Num  string `json:"num"`
			Unit string `json:"unit"`
		} `json:"rich"`
		Content   string `json:"content"`
		Image     string `json:"image"`
		FileCount string `json:"file_count"`
		Author    string `json:"author"`
	} `json:"body"`
}

// 钉钉机器人群消息
type RobotMessage struct {
	MsgType  string    `json:"msgtype"`
	Text     *Text     `json:"text"`
	Image    *Image    `json:"image"`
	Link     *Link     `json:"link"`
	Voice    *Voice    `json:"voice"`
	File     *File     `json:"file"`
	OA       *OA       `json:"oa"`
	Markdown *Markdown `json:"markdown"`
}

type Result struct {
	ErrorCode    int    `json:"errcode"`
	ErrorMessage string `json:"errmsg"`
}

func NewTextMessage(text Text) *RobotMessage {
	return &RobotMessage{
		MsgType: MessageTypeText,
		Text:    &text,
	}
}

func NewMarkdownMessage(markdown Markdown) *RobotMessage {
	return &RobotMessage{
		MsgType:  MessageTypeMarkdown,
		Markdown: &markdown,
	}
}

func SendRobot(message *RobotMessage, dingTalkRobotURL string, secret string, keyword string) error {
	builder := httpcli.NewHttpRequestBuilder().
		POST().
		WithEndpoint(dingTalkRobotURL).
		AddHeaderParam("Content-Type", "application/json").
		WithBody("", message)

	if secret != "" {
		now := time.Now().UnixNano() / 1000000

		stringToSign := fmt.Sprintf("%v\n%v", now, secret)

		h := sha256.New()
		if _, err := io.WriteString(h, stringToSign); err != nil {
			return err
		}
		h.Sum(nil)

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(stringToSign))

		signDatabytes := mac.Sum(nil)
		signData := base64.StdEncoding.EncodeToString(signDatabytes)
		sign := url.QueryEscape(string(signData))
		dingTalkRobotURL = fmt.Sprintf("%v&timestamp=%v&sign=%v", dingTalkRobotURL, now, sign)

		builder.WithEndpoint(dingTalkRobotURL)
		builder.AddQueryParam("timestamp", now)
		builder.AddQueryParam("sign", sign)
	}
	// 自定义关键词
	if keyword != "" {
		message.Markdown.Title += keyword
	}
	resp, err := builder.Build().Invoke()
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return errors.Errorf("invoke %v error,status code:%v,message:%v",
			dingTalkRobotURL, resp.GetStatusCode(), resp.GetBody())
	}
	ret := &Result{}
	if err := resp.Decode(ret); err != nil {
		return errors.WithStack(err)
	}

	if ret.ErrorCode != 0 {
		return errors.Errorf("invoke %v error,errorcode:%v, errorcode:%v",
			dingTalkRobotURL, ret.ErrorCode, resp.GetBody())
	}

	return nil
}
