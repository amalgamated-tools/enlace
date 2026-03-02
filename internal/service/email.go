package service

import (
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"strings"
	texttemplate "text/template"

	"github.com/google/uuid"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

// SMTPConfig holds SMTP connection settings.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// EmailService handles sending email notifications and tracking recipients.
type EmailService struct {
	cfg           SMTPConfig
	recipientRepo *repository.RecipientRepository
	baseURL       string
}

// NewEmailService creates a new EmailService instance.
func NewEmailService(cfg SMTPConfig, recipientRepo *repository.RecipientRepository, baseURL string) *EmailService {
	return &EmailService{
		cfg:           cfg,
		recipientRepo: recipientRepo,
		baseURL:       baseURL,
	}
}

// IsConfigured returns true if SMTP has sufficient configuration to send mail.
func (s *EmailService) IsConfigured() bool {
	return s.cfg.Host != "" && s.cfg.Port > 0 && s.cfg.From != ""
}

type emailTemplateData struct {
	Name        string
	Description string
	Link        string
}

var plainTextTmpl = texttemplate.Must(texttemplate.New("plain").Parse(
	`A file has been shared with you on Enlace!

{{ .Name }}
{{ if .Description }}
{{ .Description }}
{{ end }}
View it here: {{ .Link }}
`))

var htmlTmpl = htmltemplate.Must(htmltemplate.New("html").Parse(
	`<!DOCTYPE html>
<html>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f8fafc;">
  <div style="max-width: 480px; margin: 40px auto; background: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px; overflow: hidden;">
    <div style="padding: 32px;">
      <h2 style="margin: 0 0 8px; color: #0f172a; font-size: 18px;">A file has been shared with you</h2>
      <p style="margin: 0 0 16px; color: #64748b; font-size: 14px;">{{ .Name }}</p>
      {{ if .Description }}<p style="margin: 0 0 16px; color: #475569; font-size: 14px;">{{ .Description }}</p>{{ end }}
      <a href="{{ .Link }}" style="display: inline-block; padding: 10px 20px; background-color: #0f172a; color: #ffffff; text-decoration: none; border-radius: 8px; font-size: 14px; font-weight: 500;">View Share</a>
    </div>
  </div>
</body>
</html>`))

// SendShareNotification sends notification emails for a share and records recipients.
// It attempts delivery to all recipients and returns an error if any sends failed.
func (s *EmailService) SendShareNotification(ctx context.Context, share *model.Share, recipients []string) error {
	if !s.IsConfigured() {
		slog.WarnContext(ctx, "SMTP not configured, skipping email notification")
		return nil
	}

	shareLink := fmt.Sprintf("%s/#/s/%s", strings.TrimRight(s.baseURL, "/"), share.Slug)

	data := emailTemplateData{
		Name:        share.Name,
		Description: share.Description,
		Link:        shareLink,
	}

	var failed []string
	for _, email := range recipients {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}

		if err := s.sendMultipartEmail(email, share.Name, data); err != nil {
			slog.ErrorContext(ctx, "failed to send email",
				slog.String("to", email),
				slog.Any("error", err))
			failed = append(failed, email)
			continue
		}

		recipient := &model.ShareRecipient{
			ID:      uuid.NewString(),
			ShareID: share.ID,
			Email:   email,
		}
		if err := s.recipientRepo.Create(ctx, recipient); err != nil {
			slog.ErrorContext(ctx, "failed to record recipient",
				slog.String("email", email),
				slog.Any("error", err))
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to send to %d recipient(s): %s", len(failed), strings.Join(failed, ", "))
	}

	return nil
}

// ListRecipients returns all recipients for a given share.
func (s *EmailService) ListRecipients(ctx context.Context, shareID string) ([]*model.ShareRecipient, error) {
	return s.recipientRepo.ListByShare(ctx, shareID)
}

// sanitizeHeaderValue removes CR and LF characters to prevent SMTP header injection.
func sanitizeHeaderValue(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

func (s *EmailService) sendMultipartEmail(to, shareName string, data emailTemplateData) error {
	to = sanitizeHeaderValue(to)
	shareName = sanitizeHeaderValue(shareName)

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	// Write headers
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\n", s.cfg.From)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %q has been shared with you on Enlace\r\n", shareName)
	fmt.Fprintf(&msg, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&msg, "Content-Type: multipart/alternative; boundary=%s\r\n", writer.Boundary())
	fmt.Fprintf(&msg, "\r\n")

	// Plain text part
	plainHeader := textproto.MIMEHeader{}
	plainHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	plainPart, err := writer.CreatePart(plainHeader)
	if err != nil {
		return fmt.Errorf("failed to create plain text part: %w", err)
	}
	if err := plainTextTmpl.Execute(plainPart, data); err != nil {
		return fmt.Errorf("failed to render plain text template: %w", err)
	}

	// HTML part
	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlPart, err := writer.CreatePart(htmlHeader)
	if err != nil {
		return fmt.Errorf("failed to create HTML part: %w", err)
	}
	if err := htmlTmpl.Execute(htmlPart, data); err != nil {
		return fmt.Errorf("failed to render HTML template: %w", err)
	}

	writer.Close()

	// Combine headers and body
	msg.Write(body.Bytes())

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	var auth smtp.Auth
	if s.cfg.User != "" || s.cfg.Pass != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)
	}

	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, msg.Bytes())
}
