// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHTTPCheckerStatusMatcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	checker := NewHTTPChecker()
	tests := []struct {
		spec   string
		want   Status
		errSub string
	}{
		{spec: "200-299", want: StatusDegraded, errSub: "expected 200-299"},
		{spec: "200-599", want: StatusUp},
		{spec: "503", want: StatusUp},
		{spec: "", want: StatusDegraded},
	}
	for _, tc := range tests {
		cfg := json.RawMessage(fmt.Sprintf(`{"url":%q,"expected_status_codes":%q}`, srv.URL, tc.spec))
		res := checker.Run(context.Background(), cfg)
		if res.Status != tc.want {
			t.Errorf("spec=%q want=%s got=%s err=%s", tc.spec, tc.want, res.Status, res.ErrorMessage)
		}
		if tc.errSub != "" && !strings.Contains(res.ErrorMessage, tc.errSub) {
			t.Errorf("spec=%q error %q missing %q", tc.spec, res.ErrorMessage, tc.errSub)
		}
	}
}

func TestHTTPCheckerReusesTransport(t *testing.T) {
	var conns atomic.Int64
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	checker := NewHTTPChecker()
	cfg := json.RawMessage(fmt.Sprintf(`{"url":%q}`, srv.URL))

	for i := 0; i < 5; i++ {
		res := checker.Run(context.Background(), cfg)
		if res.Status != StatusUp {
			t.Fatalf("iteration %d: status=%s err=%s", i, res.Status, res.ErrorMessage)
		}
	}

	if got := conns.Load(); got > 1 {
		t.Errorf("expected single keep-alive connection across requests, got %d new conns", got)
	}
}
