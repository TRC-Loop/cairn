// SPDX-License-Identifier: AGPL-3.0-or-later
package config

import (
	"fmt"
	"os"
)

type Config struct {
	ListenAddr    string
	DatabasePath  string
	EncryptionKey string
}

func Load() (*Config, error) {
	c := &Config{
		ListenAddr:    getEnv("CAIRN_LISTEN_ADDR", ":8080"),
		DatabasePath:  getEnv("CAIRN_DB_PATH", "/data/cairn.db"),
		EncryptionKey: os.Getenv("CAIRN_ENCRYPTION_KEY"),
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
