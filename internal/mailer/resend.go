package mailer

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/resend/resend-go/v4"
)

const invitationTemplate = `<!doctype html>
<html lang="zh-CN">
  <body>
    <p>您好，</p>
    <p>您已受邀加入 Shadmin。请在邀请有效期内设置用户名和密码：</p>
    <p><a href="{{.URL}}">接受邀请</a></p>
    {{if .Message}}<p>邀请留言：{{.Message}}</p>{{end}}
    <p>如果您没有预期收到此邮件，可以忽略它。</p>
  </body>
</html>`

type ResendInvitationMailer struct {
	client *resend.Client
	from   string
}

func NewResendInvitationMailer(apiKey, from string) *ResendInvitationMailer {
	return &ResendInvitationMailer{
		client: resend.NewCustomClient(&http.Client{Timeout: 10 * time.Second}, apiKey),
		from:   from,
	}
}

func (m *ResendInvitationMailer) SendInvitation(ctx context.Context, to, invitationURL, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil || m.client == nil || strings.TrimSpace(m.client.ApiKey) == "" {
		return fmt.Errorf("resend API key is not configured")
	}
	if strings.TrimSpace(m.from) == "" {
		return fmt.Errorf("resend sender address is not configured")
	}

	data := struct {
		URL     string
		Message string
	}{URL: invitationURL, Message: strings.TrimSpace(message)}
	var htmlBody strings.Builder
	if err := template.Must(template.New("invitation").Parse(invitationTemplate)).Execute(&htmlBody, data); err != nil {
		return fmt.Errorf("render invitation email: %w", err)
	}

	plainText := "您好，\n\n您已受邀加入 Shadmin。请在邀请有效期内设置用户名和密码：\n" + invitationURL
	if data.Message != "" {
		plainText += "\n\n邀请留言：" + data.Message
	}

	_, err := m.client.Emails.Send(&resend.SendEmailRequest{
		From:    m.from,
		To:      []string{to},
		Subject: "您已受邀加入 Shadmin",
		Html:    htmlBody.String(),
		Text:    plainText,
	})
	if err != nil {
		return fmt.Errorf("send invitation email through Resend: %w", err)
	}
	return nil
}
