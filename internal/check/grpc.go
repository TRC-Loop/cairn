// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type grpcConfig struct {
	Address            string `json:"address"`
	Service            string `json:"service"`
	TLS                *bool  `json:"tls"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

type GRPCChecker struct{}

func (GRPCChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c grpcConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("invalid config: %v", err)}
	}
	if c.Address == "" {
		return Result{Status: StatusUnknown, ErrorMessage: "address is required"}
	}
	useTLS := true
	if c.TLS != nil {
		useTLS = *c.TLS
	}

	var creds credentials.TransportCredentials
	if useTLS {
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: c.InsecureSkipVerify})
	} else {
		creds = insecure.NewCredentials()
	}

	start := time.Now()
	conn, err := grpc.NewClient(c.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("dial: %v", err)}
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: c.Service})
	elapsed := time.Since(start)
	latencyMs := int(elapsed.Milliseconds())

	if err != nil {
		st, _ := status.FromError(err)
		return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("health check: %s", st.Message()), LatencyMs: &latencyMs}
	}

	healthStatus := resp.GetStatus().String()
	metadata := map[string]any{"health_status": healthStatus}

	switch resp.GetStatus() {
	case healthpb.HealthCheckResponse_SERVING:
		return Result{Status: StatusUp, LatencyMs: &latencyMs, Metadata: metadata}
	case healthpb.HealthCheckResponse_NOT_SERVING, healthpb.HealthCheckResponse_SERVICE_UNKNOWN:
		return Result{Status: StatusDegraded, LatencyMs: &latencyMs, Metadata: metadata, ErrorMessage: healthStatus}
	default:
		return Result{Status: StatusDown, LatencyMs: &latencyMs, Metadata: metadata, ErrorMessage: healthStatus}
	}
}
