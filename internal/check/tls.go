// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"
)

type tlsConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	WarnDays     int    `json:"warn_days"`
	CriticalDays int    `json:"critical_days"`
	ServerName   string `json:"server_name"`
}

type TLSChecker struct{}

func (TLSChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c tlsConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("invalid config: %v", err)}
	}
	if c.Host == "" {
		return Result{Status: StatusUnknown, ErrorMessage: "host is required"}
	}
	if c.Port == 0 {
		c.Port = 443
	}
	if c.WarnDays == 0 {
		c.WarnDays = 30
	}
	if c.CriticalDays == 0 {
		c.CriticalDays = 7
	}
	if c.ServerName == "" {
		c.ServerName = c.Host
	}

	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	d := &net.Dialer{}
	start := time.Now()
	rawConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("dial: %v", err)}
	}
	defer rawConn.Close()

	if dl, ok := ctx.Deadline(); ok {
		_ = rawConn.SetDeadline(dl)
	}

	conn := tls.Client(rawConn, &tls.Config{ServerName: c.ServerName})
	if err := conn.HandshakeContext(ctx); err != nil {
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("handshake: %v", err)}
	}
	elapsed := time.Since(start)
	latencyMs := int(elapsed.Milliseconds())
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return Result{Status: StatusDown, ErrorMessage: "no peer certificates", LatencyMs: &latencyMs}
	}
	cert := state.PeerCertificates[0]

	now := time.Now()
	daysRemaining := int(cert.NotAfter.Sub(now).Hours() / 24)

	metadata := map[string]any{
		"not_before":     cert.NotBefore.UTC().Format(time.RFC3339),
		"not_after":      cert.NotAfter.UTC().Format(time.RFC3339),
		"days_remaining": daysRemaining,
		"issuer":         cert.Issuer.CommonName,
		"subject":        cert.Subject.CommonName,
		"dns_names":      cert.DNSNames,
	}

	switch {
	case now.After(cert.NotAfter):
		return Result{Status: StatusDown, ErrorMessage: "certificate expired", LatencyMs: &latencyMs, Metadata: metadata}
	case now.Before(cert.NotBefore):
		return Result{Status: StatusDown, ErrorMessage: "certificate not yet valid", LatencyMs: &latencyMs, Metadata: metadata}
	case daysRemaining <= c.CriticalDays:
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("certificate expires in %d days", daysRemaining), LatencyMs: &latencyMs, Metadata: metadata}
	case daysRemaining <= c.WarnDays:
		return Result{Status: StatusDegraded, ErrorMessage: fmt.Sprintf("certificate expires in %d days", daysRemaining), LatencyMs: &latencyMs, Metadata: metadata}
	}

	return Result{Status: StatusUp, LatencyMs: &latencyMs, Metadata: metadata}
}
