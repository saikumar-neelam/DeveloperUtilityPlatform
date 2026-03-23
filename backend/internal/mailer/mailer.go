package mailer

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Config holds SMTP connection details.
type Config struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

// Mailer sends transactional emails via SMTP.
type Mailer struct {
	cfg  Config
	auth smtp.Auth
}

// New returns a Mailer. Returns nil (no-op) if Host is empty.
func New(cfg Config) *Mailer {
	if cfg.Host == "" {
		return nil
	}
	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	}
	return &Mailer{cfg: cfg, auth: auth}
}

// Send sends a plain-text email.
func (m *Mailer) Send(to, subject, body string) error {
	if m == nil {
		return nil // mailer not configured
	}
	msg := strings.Join([]string{
		fmt.Sprintf("From: WebhookDB <%s>", m.cfg.From),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%s", m.cfg.Host, m.cfg.Port)
	return smtp.SendMail(addr, m.auth, m.cfg.From, []string{to}, []byte(msg))
}
