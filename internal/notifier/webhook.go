// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/store"
)

type WebhookConfig struct {
	URL          string            `json:"url"`
	SecretEnc    string            `json:"secret_enc"`
	Method       string            `json:"method"`
	ExtraHeaders map[string]string `json:"extra_headers"`
}

type WebhookSender struct {
	secretBox *crypto.SecretBox
	logger    *slog.Logger
	client    *http.Client
	now       func() time.Time
}

func NewWebhookSender(secretBox *crypto.SecretBox, logger *slog.Logger) *WebhookSender {
	return &WebhookSender{
		secretBox: secretBox,
		logger:    logger,
		client:    &http.Client{Timeout: 15 * time.Second},
		now:       time.Now,
	}
}

func (s *WebhookSender) SetHTTPClient(c *http.Client) { s.client = c }
func (s *WebhookSender) SetNow(fn func() time.Time)   { s.now = fn }

// SignBody returns the X-Cairn-Signature header value for the given timestamp,
// secret, and body. Format: "t=<unix_seconds>,v1=<hmac_hex>" where the HMAC is
// over "<timestamp>.<body>".
func SignBody(secret string, ts time.Time, body []byte) string {
	tsStr := fmt.Sprintf("%d", ts.UTC().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsStr))
	mac.Write([]byte("."))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%s,v1=%s", tsStr, sig)
}

func (s *WebhookSender) Send(ctx context.Context, channel store.NotificationChannel, payload Payload) error {
	var cfg WebhookConfig
	if err := json.Unmarshal([]byte(channel.ConfigJson), &cfg); err != nil {
		return fmt.Errorf("decode webhook config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("missing webhook url")
	}
	method := strings.ToUpper(cfg.Method)
	if method == "" {
		method = http.MethodPost
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Cairn/0.1")

	if cfg.SecretEnc != "" {
		secret, err := s.secretBox.DecryptString(cfg.SecretEnc)
		if err != nil {
			return fmt.Errorf("decrypt webhook secret: %w", err)
		}
		req.Header.Set("X-Cairn-Signature", SignBody(secret, s.now(), body))
	}
	for k, v := range cfg.ExtraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
