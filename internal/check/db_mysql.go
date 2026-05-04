// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type mysqlConfig struct {
	DSN   string `json:"dsn"`
	Query string `json:"query"`
}

type MySQLChecker struct{}

func (MySQLChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c mysqlConfig
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
	db, err := sql.Open("mysql", c.DSN)
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
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
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
