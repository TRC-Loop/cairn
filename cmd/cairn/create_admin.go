// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/config"
	"github.com/TRC-Loop/cairn/internal/store"
	"golang.org/x/term"
)

func runCreateAdminCmd(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	db, err := store.Open(cfg.DatabasePath, cfg.SQLiteBusyTimeoutMs)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	if os.Getenv("CAIRN_SKIP_MIGRATIONS") != "1" {
		if err := runMigrations(db); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	q := store.New(db)
	return runCreateAdmin(context.Background(), db, q)
}

func runCreateAdmin(ctx context.Context, db *sql.DB, q *store.Queries) error {
	reader := bufio.NewReader(os.Stdin)

	username, err := promptLine(reader, "Username: ")
	if err != nil {
		return err
	}
	if username == "" {
		return errors.New("username required")
	}

	email, err := promptLine(reader, "Email: ")
	if err != nil {
		return err
	}
	if !strings.Contains(email, "@") {
		return errors.New("email must contain @")
	}

	displayName, err := promptLine(reader, fmt.Sprintf("Display name [%s]: ", username))
	if err != nil {
		return err
	}
	if displayName == "" {
		displayName = username
	}

	password, err := promptPassword(reader, "Password: ")
	if err != nil {
		return err
	}
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	if len(password) > 512 {
		return errors.New("password must be at most 512 characters")
	}
	confirm, err := promptPassword(reader, "Confirm password: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return errors.New("passwords do not match")
	}

	if existing, err := q.GetUserByUsername(ctx, username); err == nil {
		return fmt.Errorf("user with username %q already exists (id=%d)", username, existing.ID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lookup username: %w", err)
	}
	if existing, err := q.GetUserByEmail(ctx, email); err == nil {
		return fmt.Errorf("user with email %q already exists (id=%d)", email, existing.ID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lookup email: %w", err)
	}

	hash, err := auth.Hash(password)
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}
	u, err := q.CreateUser(ctx, store.CreateUserParams{
		Username:     username,
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: hash,
		Role:         string(auth.RoleAdmin),
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	fmt.Printf("Created admin user id=%d username=%s\n", u.ID, u.Username)
	return nil
}

func promptLine(r *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func promptPassword(r *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(b), nil
}
