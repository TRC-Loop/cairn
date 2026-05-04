// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/store"
)

type DiscordConfig struct {
	WebhookURLEnc string `json:"webhook_url_enc"`
	Username      string `json:"username"`
	AvatarURL     string `json:"avatar_url"`
}

type DiscordSender struct {
	secretBox *crypto.SecretBox
	logger    *slog.Logger
	client    *http.Client
}

func NewDiscordSender(secretBox *crypto.SecretBox, logger *slog.Logger) *DiscordSender {
	return &DiscordSender{
		secretBox: secretBox,
		logger:    logger,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

// SetHTTPClient overrides the HTTP client (for tests).
func (s *DiscordSender) SetHTTPClient(c *http.Client) { s.client = c }

func severityDiscordColor(sev string) int {
	switch sev {
	case SeverityCritical:
		return 16711680
	case SeverityMajor:
		return 16744192
	case SeverityMinor:
		return 16776960
	case SeverityMaint:
		return 11027200
	case SeverityInfo:
		return 5763719
	default:
		return 5763719
	}
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordEmbedFooter struct {
	Text string `json:"text"`
}

type discordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	URL         string              `json:"url,omitempty"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	Footer      *discordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

type discordPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []discordEmbed `json:"embeds"`
}

func (s *DiscordSender) Send(ctx context.Context, channel store.NotificationChannel, payload Payload) error {
	var cfg DiscordConfig
	if err := json.Unmarshal([]byte(channel.ConfigJson), &cfg); err != nil {
		return fmt.Errorf("decode discord config: %w", err)
	}
	if cfg.WebhookURLEnc == "" {
		return fmt.Errorf("missing discord webhook url")
	}
	url, err := s.secretBox.DecryptString(cfg.WebhookURLEnc)
	if err != nil {
		return fmt.Errorf("decrypt webhook url: %w", err)
	}
	username := cfg.Username
	if username == "" {
		username = "Cairn"
	}
	embed := discordEmbed{
		Title:       payload.Subject,
		Description: payload.Body,
		Color:       severityDiscordColor(payload.Severity),
		Footer:      &discordEmbedFooter{Text: "Cairn"},
		Timestamp:   payload.Timestamp.UTC().Format(time.RFC3339),
	}
	if payload.Severity != "" {
		embed.Fields = append(embed.Fields, discordEmbedField{
			Name: "Severity", Value: severityLabel(payload.Severity), Inline: true,
		})
	}
	if !payload.Timestamp.IsZero() {
		embed.Fields = append(embed.Fields, discordEmbedField{
			Name: "Time", Value: payload.Timestamp.UTC().Format("2006-01-02 15:04 UTC"), Inline: true,
		})
	}
	for _, l := range payload.Links {
		embed.Fields = append(embed.Fields, discordEmbedField{
			Name: l.Label, Value: l.URL, Inline: false,
		})
		if embed.URL == "" {
			embed.URL = l.URL
		}
	}

	body, err := json.Marshal(discordPayload{
		Username:  username,
		AvatarURL: cfg.AvatarURL,
		Embeds:    []discordEmbed{embed},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Cairn/0.1")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
