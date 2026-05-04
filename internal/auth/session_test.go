// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSessionCreateAndLookup(t *testing.T) {
	_, q := openTestDB(t)
	u := createTestUser(t, q, "alice", "password-long-enough", "viewer")
	svc := NewSessionService(q, testLogger())
	ctx := context.Background()

	id, err := svc.Create(ctx, u.ID, "UA", "1.2.3.4")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(id) != 43 {
		t.Fatalf("session id length unexpected: %d", len(id))
	}
	sess, user, err := svc.Lookup(ctx, id)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if user.ID != u.ID {
		t.Fatalf("user mismatch: got %d want %d", user.ID, u.ID)
	}
	if sess.ID != id {
		t.Fatalf("session id mismatch")
	}
}

func TestSessionLookupExtendsAtHalfway(t *testing.T) {
	_, q := openTestDB(t)
	u := createTestUser(t, q, "bob", "password-long-enough", "viewer")
	svc := NewSessionService(q, testLogger())
	ctx := context.Background()

	// Start at t0.
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return t0 }
	id, err := svc.Create(ctx, u.ID, "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Advance past the halfway mark (> TTL/2).
	svc.now = func() time.Time { return t0.Add(DefaultSessionTTL/2 + time.Minute) }
	sess, _, err := svc.Lookup(ctx, id)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	expectedMin := svc.now().Add(DefaultSessionTTL - time.Second)
	if sess.ExpiresAt.Before(expectedMin) {
		t.Fatalf("expiry not extended: got %v want >= %v", sess.ExpiresAt, expectedMin)
	}
}

func TestSessionLookupExpired(t *testing.T) {
	_, q := openTestDB(t)
	u := createTestUser(t, q, "carol", "password-long-enough", "viewer")
	svc := NewSessionService(q, testLogger())
	ctx := context.Background()

	t0 := time.Now().UTC()
	svc.now = func() time.Time { return t0 }
	id, err := svc.Create(ctx, u.ID, "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	svc.now = func() time.Time { return t0.Add(DefaultSessionTTL + time.Hour) }
	if _, _, err := svc.Lookup(ctx, id); err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestSessionRevoke(t *testing.T) {
	_, q := openTestDB(t)
	u := createTestUser(t, q, "dave", "password-long-enough", "viewer")
	svc := NewSessionService(q, testLogger())
	ctx := context.Background()

	id, _ := svc.Create(ctx, u.ID, "", "")
	if err := svc.Revoke(ctx, id); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, _, err := svc.Lookup(ctx, id); err == nil {
		t.Fatal("expected error after revoke")
	}
}

func TestSessionRevokeAllForUser(t *testing.T) {
	_, q := openTestDB(t)
	u1 := createTestUser(t, q, "eve", "password-long-enough", "viewer")
	u2 := createTestUser(t, q, "frank", "password-long-enough", "viewer")
	svc := NewSessionService(q, testLogger())
	ctx := context.Background()

	s1, _ := svc.Create(ctx, u1.ID, "", "")
	s2, _ := svc.Create(ctx, u1.ID, "", "")
	s3, _ := svc.Create(ctx, u2.ID, "", "")
	if err := svc.RevokeAllForUser(ctx, u1.ID); err != nil {
		t.Fatalf("revoke all: %v", err)
	}
	if _, _, err := svc.Lookup(ctx, s1); err == nil {
		t.Fatal("s1 should be revoked")
	}
	if _, _, err := svc.Lookup(ctx, s2); err == nil {
		t.Fatal("s2 should be revoked")
	}
	if _, _, err := svc.Lookup(ctx, s3); err != nil {
		t.Fatalf("s3 should survive: %v", err)
	}
}

func TestSessionDeleteExpired(t *testing.T) {
	db, q := openTestDB(t)
	u := createTestUser(t, q, "gina", "password-long-enough", "viewer")
	svc := NewSessionService(q, testLogger())
	ctx := context.Background()

	fresh, _ := svc.Create(ctx, u.ID, "", "")
	// Insert an already-expired session directly.
	expiredID, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, datetime('now', '-1 hour'))`,
		expiredID, u.ID); err != nil {
		t.Fatalf("insert expired: %v", err)
	}

	if err := svc.DeleteExpired(ctx); err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if _, err := q.GetSession(ctx, expiredID); err == nil {
		t.Fatal("expired session should be gone")
	}
	if _, err := q.GetSession(ctx, fresh); err != nil {
		t.Fatalf("fresh session removed: %v", err)
	}
}

func TestHasAtLeast(t *testing.T) {
	cases := []struct {
		actual   Role
		required Role
		want     bool
	}{
		{RoleAdmin, RoleAdmin, true},
		{RoleAdmin, RoleEditor, true},
		{RoleAdmin, RoleViewer, true},
		{RoleEditor, RoleAdmin, false},
		{RoleEditor, RoleEditor, true},
		{RoleEditor, RoleViewer, true},
		{RoleViewer, RoleAdmin, false},
		{RoleViewer, RoleEditor, false},
		{RoleViewer, RoleViewer, true},
	}
	for _, c := range cases {
		if got := HasAtLeast(c.actual, c.required); got != c.want {
			t.Errorf("HasAtLeast(%s,%s)=%v want %v", c.actual, c.required, got, c.want)
		}
	}
}

var _ = store.User{}
