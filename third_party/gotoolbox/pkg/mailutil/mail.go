package mailutil

import (
	"crypto/tls"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/netutil"

	gomail "gopkg.in/gomail.v2"
)

type MailSender interface {
	SendEmail(to /*收件人地址*/ []string, topic /*邮件标题*/, content /*邮件内容*/ string) error
}

var _ MailSender = &smtpSender{}

func NewSMTPMailSender(c SMTPServerConfig) (MailSender, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &smtpSender{
		SMTPServerIp:   c.SMTPServerIp,
		SMTPServerPort: c.SMTPServerPort,
		TLSEnabled:     c.TLSEnabled,
		SMTPAccount:    c.SMTPAccount,
		SMTPPassword:   c.SMTPPassword,
		SenderName:     c.SenderName,
		SenderMail:     c.SenderMail,
	}, nil
}

type SMTPServerConfig struct {
	SMTPServerIp   string //SMTP服务器地址
	SMTPServerPort int    //SMTP服务器端口
	TLSEnabled     bool   //SMTP是否开启TLS服务
	SMTPAccount    string //SMTP账号
	SMTPPassword   string //SMTP密码
	SenderName     string //发件人名称: 收件人看到的发件者信息
	SenderMail     string //发件人地址
}

type smtpSender struct {
	SMTPServerIp   string
	SMTPServerPort int
	TLSEnabled     bool
	SMTPAccount    string
	SMTPPassword   string
	SenderName     string
	SenderMail     string
}

func (smtp *SMTPServerConfig) Validate() error {
	if strings.TrimSpace(smtp.SMTPServerIp) == "" ||
		strings.TrimSpace(smtp.SMTPAccount) == "" ||
		strings.TrimSpace(smtp.SMTPAccount) == "" ||
		strings.TrimSpace(smtp.SMTPPassword) == "" ||
		strings.TrimSpace(smtp.SenderName) == "" ||
		strings.TrimSpace(smtp.SenderMail) == "" {
		return fmt.Errorf("invalid smtp config, required field is empty")
	}
	if !netutil.IsValidPort(smtp.SMTPServerPort) {
		return fmt.Errorf("invalid port:" + strconv.Itoa(smtp.SMTPServerPort))
	}

	return nil
}

// SendEmail 发送邮件
// topic: 标题, 至多支持4096长度
func (smtp *smtpSender) SendEmail(to /*收件人地址*/ []string, topic /*邮件标题*/, content /*邮件内容*/ string) error {
	if len(to) == 0 {
		return fmt.Errorf("must set receiver mail")
	}

	for _, v := range to {
		if !IsValidEmail(v) {
			return fmt.Errorf("invalid reciever email:%v", v)
		}
	}

	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("topic is empty")
	}

	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("content is empty")
	}

	m := gomail.NewMessage()
	m.SetHeader("Sender", smtp.SenderMail)
	m.SetHeader("From", m.FormatAddress(smtp.SMTPAccount, smtp.SenderName))
	m.SetHeader("To", to...)
	m.SetHeader("Subject", topic)
	m.SetBody("text/plain", content)

	d := gomail.NewDialer(smtp.SMTPServerIp, smtp.SMTPServerPort, smtp.SMTPAccount, smtp.SMTPPassword)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	if err := d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}

func IsValidEmail(email string) bool {
	m, err := regexp.MatchString("^\\w+([\\.-]?\\w+)*@\\w+([\\.-]?\\w+)*(\\.\\w{2,7})+$", email)
	if err != nil {
		return false
	}
	return m
}
