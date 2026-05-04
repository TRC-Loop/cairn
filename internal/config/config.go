// SPDX-License-Identifier: AGPL-3.0-or-later
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ListenAddr          string
	DatabasePath        string
	EncryptionKey       string
	SQLiteBusyTimeoutMs int
	BehindTLS           bool
}

func Load() (*Config, error) {
	busyTimeout, err := strconv.Atoi(getEnv("CAIRN_SQLITE_BUSY_TIMEOUT_MS", "5000"))
	if err != nil {
		return nil, fmt.Errorf("CAIRN_SQLITE_BUSY_TIMEOUT_MS must be an integer: %w", err)
	}

	c := &Config{
		ListenAddr:          getEnv("CAIRN_LISTEN_ADDR", ":8080"),
		DatabasePath:        getEnv("CAIRN_DB_PATH", "/data/cairn.db"),
		EncryptionKey:       os.Getenv("CAIRN_ENCRYPTION_KEY"),
		SQLiteBusyTimeoutMs: busyTimeout,
		BehindTLS:           getEnv("CAIRN_BEHIND_TLS", "") == "1",
	}

	if c.EncryptionKey == "" {
		return nil, fmt.Errorf("CAIRN_ENCRYPTION_KEY environment variable is required")
	}
	if len(c.EncryptionKey) < 32 {
		return nil, fmt.Errorf("CAIRN_ENCRYPTION_KEY must be at least 32 bytes")
	}

	return c, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
