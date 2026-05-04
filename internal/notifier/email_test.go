// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"bytes"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
	"time"
)

func TestRenderEmailMultipart(t *testing.T) {
	cfg := EmailConfig{
		FromAddress: "alerts@example.com",
		FromName:    "Cairn",
		ToAddresses: []string{"oncall@example.com", "ops@example.com"},
	}
	payload := Payload{
		Subject:   "Site is down",
		Body:      "https://example.com returned 502 for 3 consecutive checks.",
		Severity:  SeverityCritical,
		Links:     []Link{{Label: "View incident", URL: "https://status.example.com/i/1"}},
		Timestamp: time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
	}

	msg, err := RenderEmail(cfg, payload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	headers, body := splitHeader(t, msg)
	if !strings.Contains(headers, "Subject: [CRITICAL] Site is down") {
		t.Errorf("subject missing severity prefix: %q", headers)
	}
	if !strings.Contains(headers, "From: Cairn <alerts@example.com>") {
		t.Errorf("from header wrong: %q", headers)
	}
	if !strings.Contains(headers, "To: oncall@example.com, ops@example.com") {
		t.Errorf("to header wrong: %q", headers)
	}
	if !strings.Contains(headers, "Message-Id: <") && !strings.Contains(headers, "Message-ID: <") {
		t.Errorf("Message-ID header missing in:\n%s", headers)
	}

	mediaType, params, err := mime.ParseMediaType(extractHeader(headers, "Content-Type"))
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	if mediaType != "multipart/alternative" {
		t.Fatalf("expected multipart/alternative, got %s", mediaType)
	}

	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	gotPlain, gotHTML := false, false
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		ct := p.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "text/plain") {
			gotPlain = true
		}
		if strings.HasPrefix(ct, "text/html") {
			gotHTML = true
		}
		_ = p
	}
	if !gotPlain || !gotHTML {
		t.Errorf("expected both text/plain and text/html parts (plain=%v html=%v)", gotPlain, gotHTML)
	}
}

func splitHeader(t *testing.T, msg []byte) (string, []byte) {
	t.Helper()
	idx := bytes.Index(msg, []byte("\r\n\r\n"))
	if idx < 0 {
		t.Fatal("no header/body separator")
	}
	return string(msg[:idx]), msg[idx+4:]
}

func extractHeader(headers, name string) string {
	for _, line := range strings.Split(headers, "\r\n") {
		if strings.HasPrefix(line, name+": ") {
			return strings.TrimPrefix(line, name+": ")
		}
	}
	return ""
}
