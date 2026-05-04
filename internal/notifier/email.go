// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/store"
)

type EmailConfig struct {
	SMTPHost        string   `json:"smtp_host"`
	SMTPPort        int      `json:"smtp_port"`
	SMTPStartTLS    bool     `json:"smtp_starttls"`
	SMTPUsername    string   `json:"smtp_username"`
	SMTPPasswordEnc string   `json:"smtp_password_enc"`
	FromAddress     string   `json:"from_address"`
	FromName        string   `json:"from_name"`
	ToAddresses     []string `json:"to_addresses"`
}

type EmailSender struct {
	secretBox *crypto.SecretBox
	logger    *slog.Logger
	dialer    EmailDialer
}

// EmailDialer abstracts the SMTP send for tests.
type EmailDialer func(addr string, auth smtp.Auth, from string, to []string, msg []byte, useImplicitTLS bool, useSTARTTLS bool, host string) error

func defaultDialer(addr string, auth smtp.Auth, from string, to []string, msg []byte, useImplicitTLS bool, useSTARTTLS bool, host string) error {
	if useImplicitTLS {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return fmt.Errorf("tls dial: %w", err)
		}
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
		defer c.Close()
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		for _, rcpt := range to {
			if err := c.Rcpt(rcpt); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msg); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return c.Quit()
	}
	if useSTARTTLS {
		c, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		defer c.Close()
		if err := c.Hello("cairn"); err != nil {
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		for _, rcpt := range to {
			if err := c.Rcpt(rcpt); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msg); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return c.Quit()
	}
	return smtp.SendMail(addr, auth, from, to, msg)
}

func NewEmailSender(secretBox *crypto.SecretBox, logger *slog.Logger) *EmailSender {
	return &EmailSender{secretBox: secretBox, logger: logger, dialer: defaultDialer}
}

func (s *EmailSender) Send(ctx context.Context, channel store.NotificationChannel, payload Payload) error {
	var cfg EmailConfig
	if err := json.Unmarshal([]byte(channel.ConfigJson), &cfg); err != nil {
		return fmt.Errorf("decode email config: %w", err)
	}
	if cfg.SMTPHost == "" || cfg.SMTPPort == 0 || cfg.FromAddress == "" || len(cfg.ToAddresses) == 0 {
		return fmt.Errorf("incomplete email config")
	}

	password := ""
	if cfg.SMTPPasswordEnc != "" {
		pw, err := s.secretBox.DecryptString(cfg.SMTPPasswordEnc)
		if err != nil {
			return fmt.Errorf("decrypt smtp password: %w", err)
		}
		password = pw
	}

	msg, err := RenderEmail(cfg, payload)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUsername, password, cfg.SMTPHost)
	}

	implicitTLS := !cfg.SMTPStartTLS && cfg.SMTPPort == 465
	if !cfg.SMTPStartTLS && !implicitTLS {
		s.logger.Info("smtp using plaintext (no TLS)", "host", cfg.SMTPHost, "port", cfg.SMTPPort)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.dialer(addr, auth, cfg.FromAddress, cfg.ToAddresses, msg, implicitTLS, cfg.SMTPStartTLS, cfg.SMTPHost)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

const emailHTMLTemplate = `<!doctype html>
<html><body style="margin:0;padding:0;background:#0d1117;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#c9d1d9;">
<table width="100%" cellpadding="0" cellspacing="0" style="max-width:600px;margin:0 auto;background:#161b22;border:1px solid #30363d;border-radius:6px;">
<tr><td style="padding:18px 20px;background:{{.HeaderColor}};color:#ffffff;border-radius:6px 6px 0 0;">
<div style="font-size:12px;text-transform:uppercase;letter-spacing:0.5px;opacity:0.8;">{{.SeverityLabel}}</div>
<div style="font-size:18px;font-weight:600;margin-top:4px;">{{.Subject}}</div>
</td></tr>
<tr><td style="padding:20px;font-size:14px;line-height:1.5;">
<p style="margin:0 0 16px 0;white-space:pre-wrap;">{{.Body}}</p>
{{if .Links}}<p style="margin:16px 0 0 0;">{{range .Links}}<a href="{{.URL}}" style="color:#58a6ff;text-decoration:none;margin-right:12px;">{{.Label}} &rarr;</a>{{end}}</p>{{end}}
</td></tr>
<tr><td style="padding:12px 20px;border-top:1px solid #30363d;font-size:12px;color:#8b949e;">
Sent by Cairn &middot; {{.Timestamp}}
</td></tr>
</table>
</body></html>`

type emailTplData struct {
	Subject       string
	Body          string
	HeaderColor   string
	SeverityLabel string
	Links         []Link
	Timestamp     string
}

func severityHeaderColor(sev string) string {
	switch sev {
	case SeverityCritical:
		return "#da3633"
	case SeverityMajor:
		return "#db6d28"
	case SeverityMinor:
		return "#9e6a03"
	case SeverityMaint:
		return "#6639ba"
	default:
		return "#238636"
	}
}

func severityLabel(sev string) string {
	if sev == "" {
		return "Notice"
	}
	return strings.ToUpper(sev[:1]) + sev[1:]
}

// RenderEmail builds the multipart/alternative message body including headers.
// Exported for testing.
func RenderEmail(cfg EmailConfig, payload Payload) ([]byte, error) {
	tpl, err := template.New("email").Parse(emailHTMLTemplate)
	if err != nil {
		return nil, err
	}
	var htmlBuf bytes.Buffer
	if err := tpl.Execute(&htmlBuf, emailTplData{
		Subject:       payload.Subject,
		Body:          payload.Body,
		HeaderColor:   severityHeaderColor(payload.Severity),
		SeverityLabel: severityLabel(payload.Severity),
		Links:         payload.Links,
		Timestamp:     payload.Timestamp.UTC().Format(time.RFC1123),
	}); err != nil {
		return nil, err
	}

	var plain bytes.Buffer
	plain.WriteString(payload.Subject)
	plain.WriteString("\n\n")
	plain.WriteString(payload.Body)
	if len(payload.Links) > 0 {
		plain.WriteString("\n\n")
		for _, l := range payload.Links {
			fmt.Fprintf(&plain, "%s: %s\n", l.Label, l.URL)
		}
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	subject := payload.Subject
	if payload.Severity != "" {
		subject = fmt.Sprintf("[%s] %s", strings.ToUpper(payload.Severity), payload.Subject)
	}
	from := cfg.FromAddress
	if cfg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromAddress)
	}

	headers := textproto.MIMEHeader{}
	headers.Set("From", from)
	headers.Set("To", strings.Join(cfg.ToAddresses, ", "))
	headers.Set("Subject", subject)
	headers.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	headers.Set("Message-ID", generateMessageID(cfg.FromAddress))
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%s", mw.Boundary()))

	var out bytes.Buffer
	for k, v := range headers {
		fmt.Fprintf(&out, "%s: %s\r\n", k, strings.Join(v, ", "))
	}
	out.WriteString("\r\n")

	plainPart, _ := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              []string{"text/plain; charset=UTF-8"},
		"Content-Transfer-Encoding": []string{"quoted-printable"},
	})
	pqp := quotedprintable.NewWriter(plainPart)
	_, _ = pqp.Write(plain.Bytes())
	_ = pqp.Close()

	htmlPart, _ := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              []string{"text/html; charset=UTF-8"},
		"Content-Transfer-Encoding": []string{"quoted-printable"},
	})
	hqp := quotedprintable.NewWriter(htmlPart)
	_, _ = hqp.Write(htmlBuf.Bytes())
	_ = hqp.Close()

	if err := mw.Close(); err != nil {
		return nil, err
	}
	out.Write(buf.Bytes())
	return out.Bytes(), nil
}

func generateMessageID(from string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	domain := "cairn.local"
	if at := strings.LastIndex(from, "@"); at >= 0 && at+1 < len(from) {
		domain = from[at+1:]
	}
	return fmt.Sprintf("<%x.%d@%s>", b, time.Now().UnixNano(), domain)
}
