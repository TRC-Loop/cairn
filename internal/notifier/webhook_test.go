// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

func TestSignBodyRecompute(t *testing.T) {
	secret := "super-secret"
	body := []byte(`{"hello":"world"}`)
	ts := time.Unix(1700000000, 0)
	got := SignBody(secret, ts, body)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("1700000000"))
	mac.Write([]byte("."))
	mac.Write(body)
	want := fmt.Sprintf("t=1700000000,v1=%s", hex.EncodeToString(mac.Sum(nil)))
	if got != want {
		t.Fatalf("signature mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestWebhookSenderSetsSignature(t *testing.T) {
	sb := testSecretBox(t)
	encSecret, _ := sb.EncryptString("hmac-secret")

	var sigHeader string
	var bodyBytes []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader = r.Header.Get("X-Cairn-Signature")
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewWebhookSender(sb, discardLogger())
	sender.SetHTTPClient(srv.Client())
	fixed := time.Unix(1700000000, 0)
	sender.SetNow(func() time.Time { return fixed })

	cfgJSON := fmt.Sprintf(`{"url":%q,"method":"POST","secret_enc":%q}`, srv.URL, encSecret)
	channel := store.NotificationChannel{ConfigJson: cfgJSON}

	payload := Payload{Subject: "hi", Body: "world", Timestamp: fixed}
	if err := sender.Send(context.Background(), channel, payload); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.HasPrefix(sigHeader, "t=1700000000,v1=") {
		t.Fatalf("unexpected signature header: %q", sigHeader)
	}
	want := SignBody("hmac-secret", fixed, bodyBytes)
	if sigHeader != want {
		t.Fatalf("signature mismatch:\n got %q\nwant %q", sigHeader, want)
	}
}

func TestWebhookSenderExtraHeaders(t *testing.T) {
	sb := testSecretBox(t)
	got := http.Header{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewWebhookSender(sb, discardLogger())
	sender.SetHTTPClient(srv.Client())
	cfgJSON := fmt.Sprintf(`{"url":%q,"method":"POST","extra_headers":{"X-Source":"cairn","X-Env":"test"}}`, srv.URL)
	channel := store.NotificationChannel{ConfigJson: cfgJSON}

	if err := sender.Send(context.Background(), channel, Payload{Subject: "x"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if got.Get("X-Source") != "cairn" || got.Get("X-Env") != "test" {
		t.Fatalf("extra headers not forwarded: %v", got)
	}
}
