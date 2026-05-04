// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDNSCheckerLocalhost(t *testing.T) {
	cfg := json.RawMessage(`{"host":"localhost","record_type":"A"}`)
	res := DNSChecker{}.Run(context.Background(), cfg)
	if res.Status != StatusUp {
		t.Fatalf("expected up, got %s err=%s", res.Status, res.ErrorMessage)
	}
	if c, _ := res.Metadata["record_count"].(int); c == 0 {
		t.Fatalf("expected at least one A record for localhost")
	}
}

func TestDNSCheckerExpectedContains(t *testing.T) {
	cfg := json.RawMessage(`{"host":"localhost","record_type":"A","expected_contains":"127.0.0.1"}`)
	res := DNSChecker{}.Run(context.Background(), cfg)
	if res.Status != StatusUp {
		t.Fatalf("expected up, got %s err=%s", res.Status, res.ErrorMessage)
	}
}

func TestDNSCheckerExpectedContainsMismatch(t *testing.T) {
	cfg := json.RawMessage(`{"host":"localhost","record_type":"A","expected_contains":"203.0.113.99"}`)
	res := DNSChecker{}.Run(context.Background(), cfg)
	if res.Status != StatusDegraded {
		t.Fatalf("expected degraded, got %s err=%s", res.Status, res.ErrorMessage)
	}
}
