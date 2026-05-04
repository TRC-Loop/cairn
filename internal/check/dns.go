// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

type dnsConfig struct {
	Host             string `json:"host"`
	RecordType       string `json:"record_type"`
	Resolver         string `json:"resolver"`
	ExpectedContains string `json:"expected_contains"`
}

type DNSChecker struct{}

func (DNSChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c dnsConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("invalid config: %v", err)}
	}
	if c.Host == "" {
		return Result{Status: StatusUnknown, ErrorMessage: "host is required"}
	}
	rt := strings.ToUpper(c.RecordType)
	switch rt {
	case "A", "AAAA", "CNAME", "MX", "TXT", "NS":
	default:
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("unsupported record_type %q", c.RecordType)}
	}

	resolver := net.DefaultResolver
	if c.Resolver != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := &net.Dialer{}
				return d.DialContext(ctx, network, c.Resolver)
			},
		}
	}

	start := time.Now()
	records, err := lookupDNS(ctx, resolver, rt, c.Host)
	elapsed := time.Since(start)
	latencyMs := int(elapsed.Milliseconds())

	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: err.Error(), LatencyMs: &latencyMs}
	}

	metadata := map[string]any{
		"records":      records,
		"record_count": len(records),
	}

	if c.ExpectedContains != "" {
		matched := false
		for _, r := range records {
			if strings.Contains(r, c.ExpectedContains) {
				matched = true
				break
			}
		}
		if !matched {
			return Result{
				Status:       StatusDegraded,
				LatencyMs:    &latencyMs,
				Metadata:     metadata,
				ErrorMessage: fmt.Sprintf("no record contains %q", c.ExpectedContains),
			}
		}
	}

	return Result{Status: StatusUp, LatencyMs: &latencyMs, Metadata: metadata}
}

func lookupDNS(ctx context.Context, r *net.Resolver, recordType, host string) ([]string, error) {
	switch recordType {
	case "A", "AAAA":
		network := "ip4"
		if recordType == "AAAA" {
			network = "ip6"
		}
		addrs, err := r.LookupIP(ctx, network, host)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(addrs))
		for _, a := range addrs {
			out = append(out, a.String())
		}
		return out, nil
	case "CNAME":
		cname, err := r.LookupCNAME(ctx, host)
		if err != nil {
			return nil, err
		}
		return []string{cname}, nil
	case "MX":
		mxs, err := r.LookupMX(ctx, host)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(mxs))
		for _, m := range mxs {
			out = append(out, fmt.Sprintf("%d %s", m.Pref, m.Host))
		}
		return out, nil
	case "TXT":
		return r.LookupTXT(ctx, host)
	case "NS":
		nss, err := r.LookupNS(ctx, host)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(nss))
		for _, n := range nss {
			out = append(out, n.Host)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported record_type %q", recordType)
}
