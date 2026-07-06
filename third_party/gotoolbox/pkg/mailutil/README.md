# mailutil — 邮件发送

**功能**：SMTP 邮件发送，支持 TLS 加密。

| 函数/方法 | 说明 |
|---|---|
| `NewSMTPMailSender(cfg) (MailSender, error)` | 创建 SMTP 邮件发送器 |
| `MailSender.SendEmail(to, topic, content) error` | 发送邮件 |
| `IsValidEmail(email) bool` | 验证邮箱格式 |
[← 返回包列表](../../README.md)
