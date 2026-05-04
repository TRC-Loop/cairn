// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresConfig struct {
	DSN   string `json:"dsn"`
	Query string `json:"query"`
}

type PostgresChecker struct{}

func (PostgresChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c postgresConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("invalid config: %v", err)}
	}
	if c.DSN == "" {
		return Result{Status: StatusUnknown, ErrorMessage: "dsn is required"}
	}
	if c.Query == "" {
		c.Query = "SELECT 1"
	}

	start := time.Now()
	db, err := sql.Open("pgx", c.DSN)
	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: redactDBError(err, c.DSN)}
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return Result{Status: StatusDown, ErrorMessage: redactDBError(err, c.DSN)}
	}

	if _, err := db.ExecContext(ctx, c.Query); err != nil {
		return Result{Status: StatusDown, ErrorMessage: redactDBError(err, c.DSN)}
	}

	var version string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		version = ""
	}

	elapsed := time.Since(start)
	latencyMs := int(elapsed.Milliseconds())

	metadata := map[string]any{}
	if version != "" {
		metadata["server_version"] = version
	}

	return Result{Status: StatusUp, LatencyMs: &latencyMs, Metadata: metadata}
}


