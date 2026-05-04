// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	probing "github.com/prometheus-community/pro-bing"
)

type icmpConfig struct {
	Host  string `json:"host"`
	Count int    `json:"count"`
}

type ICMPChecker struct{}

func (ICMPChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c icmpConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("invalid config: %v", err)}
	}
	if c.Host == "" {
		return Result{Status: StatusUnknown, ErrorMessage: "host is required"}
	}
	if c.Count <= 0 {
		c.Count = 3
	}

	pinger, err := probing.NewPinger(c.Host)
	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("create pinger: %v", err)}
	}
	pinger.Count = c.Count
	// On Windows, raw ICMP is the only mode available; on Linux without CAP_NET_RAW
	// the unprivileged UDP path is required.
	if runtime.GOOS == "windows" {
		pinger.SetPrivileged(true)
	}

	if err := pinger.RunWithContext(ctx); err != nil {
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("ping: %v", err)}
	}
	stats := pinger.Statistics()

	avgMs := int(stats.AvgRtt.Milliseconds())
	metadata := map[string]any{
		"packets_sent":        stats.PacketsSent,
		"packets_received":    stats.PacketsRecv,
		"packet_loss_percent": stats.PacketLoss,
		"avg_rtt_ms":          stats.AvgRtt.Milliseconds(),
		"min_rtt_ms":          stats.MinRtt.Milliseconds(),
		"max_rtt_ms":          stats.MaxRtt.Milliseconds(),
	}

	switch {
	case stats.PacketsRecv == 0:
		return Result{Status: StatusDown, ErrorMessage: "all packets lost", Metadata: metadata}
	case stats.PacketsRecv < stats.PacketsSent:
		return Result{Status: StatusDegraded, LatencyMs: &avgMs, Metadata: metadata, ErrorMessage: fmt.Sprintf("packet loss %.1f%%", stats.PacketLoss)}
	default:
		return Result{Status: StatusUp, LatencyMs: &avgMs, Metadata: metadata}
	}
}
