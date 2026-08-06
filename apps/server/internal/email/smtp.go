package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
)

type Config struct {
	Host      string
	Port      int
	Username  string
	Password  string
	From      string
	PublicURL string
	BrandName string
}

type SMTPMailer struct {
	cfg Config
}

func NewSMTPMailer(cfg Config) *SMTPMailer {
	if strings.TrimSpace(cfg.Host) == "" {
		cfg.Host = "smtp.qq.com"
	}
	if cfg.Port <= 0 {
		cfg.Port = 465
	}
	if strings.TrimSpace(cfg.BrandName) == "" {
		cfg.BrandName = "LinguaQuest"
	}
	cfg.PublicURL = strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) Configured() bool {
	return strings.TrimSpace(m.cfg.Username) != "" && strings.TrimSpace(m.cfg.Password) != "" && strings.TrimSpace(m.cfg.From) != ""
}

func (m *SMTPMailer) SendEmailVerification(ctx context.Context, to string, username string, verifyURL string) error {
	return m.send(ctx, to, "验证你的 LinguaQuest 邮箱", "验证邮箱", fmt.Sprintf("Hi %s，欢迎加入 LinguaQuest。请在 30 分钟内验证邮箱以启用登录。", template.HTMLEscapeString(username)), "验证邮箱", verifyURL)
}

func (m *SMTPMailer) SendPasswordReset(ctx context.Context, to string, username string, resetURL string) error {
	return m.send(ctx, to, "重置你的 LinguaQuest 密码", "重置密码", fmt.Sprintf("Hi %s，我们收到了你的密码重置请求。链接将在 30 分钟后失效。", template.HTMLEscapeString(username)), "重置密码", resetURL)
}

func (m *SMTPMailer) SendUsernameRecovery(ctx context.Context, to string, usernames []string) error {
	items := make([]string, 0, len(usernames))
	for _, username := range usernames {
		items = append(items, "<li>"+template.HTMLEscapeString(username)+"</li>")
	}
	body := "我们为这个邮箱找到了以下 LinguaQuest 用户名：<ul>" + strings.Join(items, "") + "</ul>"
	return m.send(ctx, to, "找回你的 LinguaQuest 用户名", "用户名找回", body, "前往登录", m.cfg.PublicURL+"/login")
}

func (m *SMTPMailer) send(ctx context.Context, to string, subject string, title string, message string, action string, actionURL string) error {
	if !m.Configured() {
		return fmt.Errorf("SMTP 尚未配置。")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	html, err := renderTemplate(templateData{
		BrandName: m.cfg.BrandName,
		LogoURL:   m.cfg.PublicURL + "/linguaquest-mark.svg",
		Title:     title,
		Message:   template.HTML(message),
		Action:    action,
		ActionURL: actionURL,
	})
	if err != nil {
		return err
	}
	return m.deliver(to, subject, html)
}

func (m *SMTPMailer) deliver(to string, subject string, body string) error {
	var encoded bytes.Buffer
	writer := quotedprintable.NewWriter(&encoded)
	if _, err := writer.Write([]byte(body)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	fromHeader := mime.QEncoding.Encode("UTF-8", m.cfg.BrandName) + " <" + m.cfg.From + ">"
	message := strings.Join([]string{
		"From: " + fromHeader,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		encoded.String(),
	}, "\r\n")

	address := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	tlsConfig := &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}
	var client *smtp.Client
	var err error
	if m.cfg.Port == 465 {
		conn, dialErr := tls.Dial("tcp", address, tlsConfig)
		if dialErr != nil {
			return dialErr
		}
		client, err = smtp.NewClient(conn, m.cfg.Host)
	} else {
		client, err = smtp.Dial(address)
		if err == nil {
			if supported, _ := client.Extension("STARTTLS"); supported {
				err = client.StartTLS(tlsConfig)
			}
		}
	}
	if err != nil {
		return err
	}
	defer client.Quit()
	if err = client.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
		return err
	}
	if err = client.Mail(m.cfg.From); err != nil {
		return err
	}
	if err = client.Rcpt(to); err != nil {
		return err
	}
	bodyWriter, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = bodyWriter.Write([]byte(message)); err != nil {
		_ = bodyWriter.Close()
		return err
	}
	return bodyWriter.Close()
}

type templateData struct {
	BrandName string
	LogoURL   string
	Title     string
	Message   template.HTML
	Action    string
	ActionURL string
}

var mailTemplate = template.Must(template.New("mail").Parse(`<!doctype html>
<html><body style="margin:0;background:#f6f2e8;font-family:Arial,'Microsoft YaHei',sans-serif;color:#211a33">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td align="center" style="padding:32px 16px">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#fffdf8;border:1px solid #e7d19a;border-radius:18px;overflow:hidden">
<tr><td style="padding:28px 32px 12px;text-align:center"><img src="{{.LogoURL}}" width="64" height="64" alt="{{.BrandName}}" style="display:inline-block;width:64px;height:64px"/><p style="margin:10px 0 0;color:#80652a;font-weight:bold;letter-spacing:.08em">{{.BrandName}}</p></td></tr>
<tr><td style="padding:10px 32px 32px"><h1 style="font-size:24px;margin:0 0 14px">{{.Title}}</h1><div style="font-size:15px;line-height:1.7;color:#4f465f">{{.Message}}</div><p style="text-align:center;margin:26px 0 20px"><a href="{{.ActionURL}}" style="display:inline-block;padding:12px 22px;border-radius:10px;background:#d8aa2f;color:#211a33;text-decoration:none;font-weight:bold">{{.Action}}</a></p><p style="font-size:12px;color:#887f92;margin:0">如果这不是你本人操作，请忽略这封邮件。</p></td></tr>
</table></td></tr></table></body></html>`))

func renderTemplate(data templateData) (string, error) {
	var buf bytes.Buffer
	if err := mailTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
