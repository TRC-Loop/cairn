// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"context"
	"log/slog"
	"time"
)

type SessionCleanup struct {
	svc      *SessionService
	logger   *slog.Logger
	interval time.Duration
}

func NewSessionCleanup(svc *SessionService, logger *slog.Logger) *SessionCleanup {
	return &SessionCleanup{svc: svc, logger: logger, interval: time.Hour}
}

func (c *SessionCleanup) Start(ctx context.Context) error {
	t := time.NewTicker(c.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := c.svc.DeleteExpired(ctx); err != nil {
				c.logger.Warn("delete expired sessions failed", "err", err)
				continue
			}
			c.logger.Debug("expired sessions purged")
		}
	}
}
