// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func runHealthcheckCmd() error {
	addr := os.Getenv("CAIRN_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host := "127.0.0.1"
	if i := strings.LastIndex(addr, ":"); i >= 0 && i < len(addr)-1 {
		addr = addr[i:]
	} else {
		addr = ":8080"
	}
	url := "http://" + host + addr + "/healthz"

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
