// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type httpConfig struct {
	URL                  string            `json:"url"`
	Method               string            `json:"method,omitempty"`
	ExpectedStatusCodes  string            `json:"expected_status_codes,omitempty"`
	ExpectedBodyContains string            `json:"expected_body_contains,omitempty"`
	ExpectedJSONPath     string            `json:"expected_json_path,omitempty"`
	ExpectedJSONValue    string            `json:"expected_json_value,omitempty"`
	FollowRedirects      *bool             `json:"follow_redirects,omitempty"`
	InsecureSkipVerify   bool              `json:"insecure_skip_verify,omitempty"`
	Headers              map[string]string `json:"headers,omitempty"`
}

type HTTPChecker struct {
	transport         *http.Transport
	insecureTransport *http.Transport
}

func NewHTTPChecker() *HTTPChecker {
	return &HTTPChecker{
		transport:         newHTTPTransport(false),
		insecureTransport: newHTTPTransport(true),
	}
}

func newHTTPTransport(insecureSkipVerify bool) *http.Transport {
	return &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}
}

func (h *HTTPChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c httpConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("invalid config: %v", err)}
	}
	if c.URL == "" {
		return Result{Status: StatusUnknown, ErrorMessage: "url is required"}
	}
	if c.Method == "" {
		c.Method = http.MethodGet
	}
	followRedirects := true
	if c.FollowRedirects != nil {
		followRedirects = *c.FollowRedirects
	}

	transport := h.transport
	if c.InsecureSkipVerify {
		transport = h.insecureTransport
	}
	client := &http.Client{Transport: transport}
	if !followRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	req, err := http.NewRequestWithContext(ctx, c.Method, c.URL, nil)
	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("build request: %v", err)}
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: err.Error()}
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	elapsed := time.Since(start)
	latencyMs := int(elapsed.Milliseconds())

	if readErr != nil {
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("read body: %v", readErr)}
	}

	metadata := map[string]any{
		"status_code":         resp.StatusCode,
		"response_size_bytes": len(body),
		"content_type":        resp.Header.Get("Content-Type"),
	}
	if resp.TLS != nil {
		metadata["tls_version"] = tlsVersionString(resp.TLS.Version)
	}

	result := Result{
		Status:    StatusUp,
		LatencyMs: &latencyMs,
		Metadata:  metadata,
	}

	if err := evaluateAssertions(&c, resp.StatusCode, body); err != nil {
		result.Status = StatusDegraded
		result.ErrorMessage = err.Error()
	}

	return result
}

func evaluateAssertions(c *httpConfig, statusCode int, body []byte) error {
	matcher, err := ParseStatusMatcher(c.ExpectedStatusCodes)
	if err != nil {
		return fmt.Errorf("invalid expected_status_codes: %w", err)
	}
	if !matcher.Matches(statusCode) {
		return fmt.Errorf("got %d, expected %s", statusCode, matcher.String())
	}
	if c.ExpectedBodyContains != "" {
		if !strings.Contains(string(body), c.ExpectedBodyContains) {
			return errors.New("expected body_contains not found")
		}
	}
	if c.ExpectedJSONPath != "" && c.ExpectedJSONValue != "" {
		got := gjson.GetBytes(body, c.ExpectedJSONPath).String()
		if got != c.ExpectedJSONValue {
			return fmt.Errorf("expected json %s=%q, got %q", c.ExpectedJSONPath, c.ExpectedJSONValue, got)
		}
	}
	return nil
}

func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}
