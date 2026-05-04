// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

func TestDiscordSenderPostsEmbed(t *testing.T) {
	sb := testSecretBox(t)
	enc, err := sb.EncryptString("https://discord.example/webhook/abc")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewDiscordSender(sb, discardLogger())
	sender.SetHTTPClient(srv.Client())

	cfgJSON := `{"webhook_url_enc":"` + enc + `","username":"Cairn","avatar_url":""}`
	channel := store.NotificationChannel{ID: 1, Type: ChannelDiscord, ConfigJson: cfgJSON}

	// Override decoded URL by re-encrypting srv.URL so the request actually hits httptest.
	enc2, _ := sb.EncryptString(srv.URL)
	channel.ConfigJson = `{"webhook_url_enc":"` + enc2 + `","username":"Cairn"}`

	payload := Payload{
		Subject:   "Outage",
		Body:      "Service is down",
		Severity:  SeverityCritical,
		Timestamp: time.Now().UTC(),
	}
	if err := sender.Send(context.Background(), channel, payload); err != nil {
		t.Fatalf("send: %v", err)
	}

	embeds, ok := got["embeds"].([]any)
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %v", got["embeds"])
	}
	embed := embeds[0].(map[string]any)
	if embed["title"] != "Outage" {
		t.Errorf("title=%v", embed["title"])
	}
	if int(embed["color"].(float64)) != severityDiscordColor(SeverityCritical) {
		t.Errorf("color=%v want %d", embed["color"], severityDiscordColor(SeverityCritical))
	}
	if got["username"] != "Cairn" {
		t.Errorf("username=%v", got["username"])
	}
}

func TestDiscordSenderHTTPError(t *testing.T) {
	sb := testSecretBox(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	enc, _ := sb.EncryptString(srv.URL)
	sender := NewDiscordSender(sb, discardLogger())
	sender.SetHTTPClient(srv.Client())
	channel := store.NotificationChannel{ConfigJson: `{"webhook_url_enc":"` + enc + `"}`}

	err := sender.Send(context.Background(), channel, Payload{Subject: "x", Timestamp: time.Now()})
	if err == nil {
		t.Fatal("expected error from non-2xx response")
	}
}
