package service

import (
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	"strings"
	texttemplate "text/template"

	"github.com/google/uuid"
	mail "github.com/wneessen/go-mail"

	"github.com/amalgamated-tools/enlace/internal/model"
	"github.com/amalgamated-tools/enlace/internal/repository"
)

// SMTPConfig holds SMTP connection settings.
type SMTPConfig struct {
	Host      string
	Port      int
	User      string
	Pass      string
	From      string
	TLSPolicy string
}

// MailSender abstracts the go-mail client for testing.
type MailSender interface {
	DialAndSendWithContext(ctx context.Context, msgs ...*mail.Msg) error
}

// EmailService handles sending email notifications and tracking recipients.
type EmailService struct {
	cfg           SMTPConfig
	recipientRepo *repository.RecipientRepository
	baseURL       string
	sender        MailSender
}

// NewEmailService creates a new EmailService instance.
func NewEmailService(cfg SMTPConfig, recipientRepo *repository.RecipientRepository, baseURL string) *EmailService {
	svc := &EmailService{
		cfg:           cfg,
		recipientRepo: recipientRepo,
		baseURL:       baseURL,
	}

	// Only create the mail client when SMTP is configured.
	if cfg.Host != "" && cfg.Port > 0 && cfg.From != "" {
		opts := []mail.Option{
			mail.WithPort(cfg.Port),
			mail.WithTLSPortPolicy(parseTLSPolicy(cfg.TLSPolicy)),
		}
		if cfg.User != "" && cfg.Pass != "" {
			opts = append(opts,
				mail.WithSMTPAuth(mail.SMTPAuthPlain),
				mail.WithUsername(cfg.User),
				mail.WithPassword(cfg.Pass),
			)
		} else if cfg.User != "" || cfg.Pass != "" {
			slog.Error("incomplete SMTP auth configuration: both user and pass must be set; skipping SMTP auth",
				slog.String("user_set", fmt.Sprintf("%t", cfg.User != "")),
				slog.String("pass_set", fmt.Sprintf("%t", cfg.Pass != "")),
			)
		}
		client, err := mail.NewClient(cfg.Host, opts...)
		if err != nil {
			slog.Error("failed to create mail client", slog.Any("error", err))
		} else {
			svc.sender = client
		}
	}

	return svc
}

// parseTLSPolicy converts a string TLS policy to a go-mail TLSPolicy.
func parseTLSPolicy(policy string) mail.TLSPolicy {
	switch strings.ToLower(policy) {
	case "mandatory":
		return mail.TLSMandatory
	case "none", "notls":
		return mail.NoTLS
	default:
		return mail.TLSOpportunistic
	}
}

// SetSender overrides the mail sender (for testing).
func (s *EmailService) SetSender(sender MailSender) {
	s.sender = sender
}

// IsConfigured returns true if SMTP has sufficient configuration to send mail.
func (s *EmailService) IsConfigured() bool {
	return s.cfg.Host != "" && s.cfg.Port > 0 && s.cfg.From != "" && s.sender != nil
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
// It batches all messages over a single SMTP connection and returns an error if any sends failed.
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

	// Render templates once for all recipients.
	var plainBuf, htmlBuf bytes.Buffer
	if err := plainTextTmpl.Execute(&plainBuf, data); err != nil {
		return fmt.Errorf("render plain text template: %w", err)
	}
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return fmt.Errorf("render HTML template: %w", err)
	}
	plainBody := plainBuf.String()
	htmlBody := htmlBuf.String()

	// Build one message per valid recipient.
	var msgs []*mail.Msg
	var validEmails []string
	for _, email := range recipients {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		m := mail.NewMsg()
		if err := m.From(s.cfg.From); err != nil {
			slog.ErrorContext(ctx, "invalid From address", slog.Any("error", err))
			return fmt.Errorf("invalid From address: %w", err)
		}
		if err := m.To(email); err != nil {
			slog.ErrorContext(ctx, "invalid To address", slog.String("to", email), slog.Any("error", err))
			continue
		}
		m.Subject(fmt.Sprintf("%s has been shared with you on Enlace", share.Name))
		m.SetDate()
		m.SetMessageID()
		m.SetBodyString(mail.TypeTextPlain, plainBody)
		m.AddAlternativeString(mail.TypeTextHTML, htmlBody)
		msgs = append(msgs, m)
		validEmails = append(validEmails, email)
	}

	if len(msgs) == 0 {
		return nil
	}

	// Send all messages over a single connection.
	sendErr := s.sender.DialAndSendWithContext(ctx, msgs...)
	if sendErr != nil {
		slog.ErrorContext(ctx, "failed to send emails", slog.Any("error", sendErr))
	}

	// Determine if the failure was connection-level (no per-message errors set)
	// vs per-message (individual messages have HasSendError).
	connectionFailure := sendErr != nil
	if connectionFailure {
		for _, m := range msgs {
			if m.HasSendError() {
				connectionFailure = false
				break
			}
		}
	}

	// Record successful sends and track failures.
	var failed []string
	for i, m := range msgs {
		email := validEmails[i]

		if connectionFailure || m.HasSendError() {
			errDetail := m.SendError()
			if errDetail == nil {
				errDetail = sendErr
			}
			slog.ErrorContext(ctx, "failed to send email",
				slog.String("to", email),
				slog.Any("error", errDetail))
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
			failed = append(failed, email)
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
