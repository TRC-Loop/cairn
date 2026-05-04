// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"testing"
)

func TestTCPCheckerUp(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	cfg := json.RawMessage(fmt.Sprintf(`{"host":"127.0.0.1","port":%d}`, port))
	res := TCPChecker{}.Run(context.Background(), cfg)
	if res.Status != StatusUp {
		t.Fatalf("expected up, got %s err=%s", res.Status, res.ErrorMessage)
	}
}

func TestTCPCheckerDown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	_, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	cfg := json.RawMessage(fmt.Sprintf(`{"host":"127.0.0.1","port":%d}`, port))
	res := TCPChecker{}.Run(context.Background(), cfg)
	if res.Status != StatusDown {
		t.Fatalf("expected down, got %s", res.Status)
	}
}
