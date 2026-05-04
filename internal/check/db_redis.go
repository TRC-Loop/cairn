// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisConfig struct {
	Address  string `json:"address"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	Command  string `json:"command"`
}

type RedisChecker struct{}

func (RedisChecker) Run(ctx context.Context, cfg json.RawMessage) Result {
	var c redisConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return Result{Status: StatusUnknown, ErrorMessage: fmt.Sprintf("invalid config: %v", err)}
	}
	if c.Address == "" {
		return Result{Status: StatusUnknown, ErrorMessage: "address is required"}
	}
	if c.Command == "" {
		c.Command = "PING"
	}

	start := time.Now()
	client := redis.NewClient(&redis.Options{
		Addr:     c.Address,
		Password: c.Password,
		DB:       c.DB,
	})
	defer client.Close()

	args := strings.Fields(c.Command)
	if len(args) == 0 {
		return Result{Status: StatusUnknown, ErrorMessage: "command is empty"}
	}
	cmdArgs := make([]any, len(args))
	for i, a := range args {
		cmdArgs[i] = a
	}

	res, err := client.Do(ctx, cmdArgs...).Result()
	if err != nil {
		return Result{Status: StatusDown, ErrorMessage: scrubRedisErr(err, c.Password)}
	}

	if strings.EqualFold(args[0], "PING") {
		s, _ := res.(string)
		if !strings.EqualFold(s, "PONG") {
			return Result{Status: StatusDown, ErrorMessage: fmt.Sprintf("unexpected PING response: %v", res)}
		}
	}

	elapsed := time.Since(start)
	latencyMs := int(elapsed.Milliseconds())

	metadata := map[string]any{}
	if info, err := client.Info(ctx, "server").Result(); err == nil {
		if v := extractRedisVersion(info); v != "" {
			metadata["server_version"] = v
		}
	}

	return Result{Status: StatusUp, LatencyMs: &latencyMs, Metadata: metadata}
}

func scrubRedisErr(err error, password string) string {
	msg := err.Error()
	if password != "" {
		msg = strings.ReplaceAll(msg, password, "****")
	}
	return msg
}

func extractRedisVersion(info string) string {
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "redis_version:") {
			return strings.TrimPrefix(line, "redis_version:")
		}
	}
	return ""
}
