// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"
)

type tcpConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type TCPChecker struct{}

func (TCPChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c tcpConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("invalid config: %v", err)}
	}
	if c.Host == "" {
		return Result{Status: StatusUnknown, ErrorMessage: "host is required"}
	}
	if c.Port <= 0 || c.Port > 65535 {
		return Result{Status: StatusUnknown, ErrorMessage: "port must be 1-65535"}
	}

	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	d := &net.Dialer{}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", addr)
	elapsed := time.Since(start)
	latencyMs := int(elapsed.Milliseconds())

	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: err.Error(), LatencyMs: &latencyMs}
	}
	remoteAddr := conn.RemoteAddr().String()
	_ = conn.Close()

	metadata := map[string]any{}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		metadata["resolved_ip"] = host
	}

	return Result{
		Status:    StatusUp,
		LatencyMs: &latencyMs,
		Metadata:  metadata,
	}
}
