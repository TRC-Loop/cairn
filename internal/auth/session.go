// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

const DefaultSessionTTL = 7 * 24 * time.Hour

var ErrSessionExpired = errors.New("session expired")

type SessionService struct {
	q          *store.Queries
	logger     *slog.Logger
	sessionTTL time.Duration
	now        func() time.Time
}

func NewSessionService(q *store.Queries, logger *slog.Logger) *SessionService {
	return &SessionService{
		q:          q,
		logger:     logger,
		sessionTTL: DefaultSessionTTL,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (s *SessionService) Create(ctx context.Context, userID int64, userAgent, ipAddress string) (string, error) {
	id, err := GenerateSessionID()
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	expiresAt := s.now().Add(s.sessionTTL)
	_, err = s.q.CreateSession(ctx, store.CreateSessionParams{
		ID:        id,
		UserID:    userID,
		ExpiresAt: expiresAt,
		UserAgent: nullString(userAgent),
		IpAddress: nullString(ipAddress),
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

func (s *SessionService) Lookup(ctx context.Context, sessionID string) (store.Session, store.User, error) {
	row, err := s.q.GetSessionWithUser(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.Session{}, store.User{}, sql.ErrNoRows
		}
		return store.Session{}, store.User{}, fmt.Errorf("lookup session: %w", err)
	}
	now := s.now()
	if !row.Session.ExpiresAt.After(now) {
		return store.Session{}, store.User{}, ErrSessionExpired
	}
	// Sliding expiry: extend if past halfway.
	halfway := now.Add(s.sessionTTL / 2)
	if row.Session.ExpiresAt.Before(halfway) {
		newExpiry := now.Add(s.sessionTTL)
		if err := s.q.ExtendSession(ctx, store.ExtendSessionParams{ExpiresAt: newExpiry, ID: sessionID}); err != nil {
			s.logger.Warn("extend session failed", "id", sessionID, "err", err)
		} else {
			row.Session.ExpiresAt = newExpiry
		}
	}
	return row.Session, row.User, nil
}

func (s *SessionService) Revoke(ctx context.Context, sessionID string) error {
	if err := s.q.DeleteSession(ctx, sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SessionService) RevokeAllForUser(ctx context.Context, userID int64) error {
	if err := s.q.DeleteSessionsForUser(ctx, userID); err != nil {
		return fmt.Errorf("delete sessions for user: %w", err)
	}
	return nil
}

func (s *SessionService) ListForUser(ctx context.Context, userID int64) ([]store.Session, error) {
	return s.q.ListSessionsForUser(ctx, userID)
}

func (s *SessionService) DeleteExpired(ctx context.Context) error {
	return s.q.DeleteExpiredSessions(ctx)
}

func nullString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}
