// SPDX-License-Identifier: AGPL-3.0-or-later

// Internal naming uses "check" for historical reasons; the user-facing name
// is "monitor". DB tables, sqlc-generated types, and this package retain
// the original terminology.
package check

import (
	"context"
	"encoding/json"
)

type Status string

const (
	StatusUp       Status = "up"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
	StatusUnknown  Status = "unknown"
)

type Type string

const (
	TypeHTTP       Type = "http"
	TypeTCP        Type = "tcp"
	TypeICMP       Type = "icmp"
	TypeDNS        Type = "dns"
	TypeTLS        Type = "tls"
	TypePush       Type = "push"
	TypeDBPostgres Type = "db_postgres"
	TypeDBMySQL    Type = "db_mysql"
	TypeDBRedis    Type = "db_redis"
	TypeGRPC       Type = "grpc"
)

type Result struct {
	Status       Status
	LatencyMs    *int
	ErrorMessage string
	Metadata     map[string]any
}

type Checker interface {
	Run(ctx context.Context, cfg json.RawMessage) Result
}
