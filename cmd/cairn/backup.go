// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/TRC-Loop/cairn/internal/backup"
	"github.com/TRC-Loop/cairn/internal/config"
	"github.com/TRC-Loop/cairn/internal/store"
	"golang.org/x/term"
)

func runBackupCmd(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	var (
		output     string
		bundle     bool
		passphrase bool
		plain      bool
		force      bool
		dbPathFlag string
	)
	fs.StringVar(&output, "o", "", "output file (default: cairn-backup-{timestamp}.{ext})")
	fs.StringVar(&output, "output", "", "output file")
	fs.BoolVar(&bundle, "bundle", false, "produce a bundle (DB + encryption key)")
	fs.BoolVar(&passphrase, "passphrase", false, "encrypt the bundle with a passphrase (implies --bundle)")
	fs.BoolVar(&plain, "plain", false, "produce an UNENCRYPTED bundle (implies --bundle; requires --force)")
	fs.BoolVar(&force, "force", false, "required with --plain")
	fs.StringVar(&dbPathFlag, "db", "", "path to cairn.db (default: $CAIRN_DB_PATH or ./cairn.db)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if passphrase || plain {
		bundle = true
	}
	if passphrase && plain {
		return errors.New("choose either --passphrase or --plain, not both")
	}
	if plain && !force {
		fmt.Fprintln(os.Stderr, plainWarning)
		return errors.New("refusing to create unencrypted bundle without --force")
	}

	dbPath := resolveDBPath(dbPathFlag)
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("database not found at %s: %w", dbPath, err)
	}

	masterKey := os.Getenv("CAIRN_ENCRYPTION_KEY")
	if bundle && masterKey == "" {
		return errors.New("CAIRN_ENCRYPTION_KEY must be set to include the encryption key in a bundle")
	}

	db, err := store.Open(dbPath, 5000)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	mode := backup.ModeDBOnly
	switch {
	case passphrase:
		mode = backup.ModeBundleEncrypted
	case plain:
		mode = backup.ModeBundlePlain
	case bundle:
		mode = backup.ModeBundleEncrypted
	}

	pass := ""
	if mode == backup.ModeBundleEncrypted {
		p, err := readPassphraseConfirmed("Bundle passphrase: ", "Confirm passphrase: ")
		if err != nil {
			return err
		}
		if p == "" {
			return errors.New("empty passphrase; use --plain --force for an unencrypted bundle")
		}
		if len(p) < 12 {
			return errors.New("passphrase must be at least 12 characters")
		}
		pass = p
	}

	if output == "" {
		output = backup.SuggestFilename(mode, nowUTC())
	}

	tmp := output + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	cleanupTmp := func() { _ = os.Remove(tmp) }

	svc := backup.NewService(db, dbPath, masterKey, cairnVersion, logger)
	switch mode {
	case backup.ModeDBOnly:
		if _, err := svc.CreateDBOnly(context.Background(), out); err != nil {
			out.Close()
			cleanupTmp()
			return err
		}
	default:
		if err := svc.CreateBundle(context.Background(), out, pass); err != nil {
			out.Close()
			cleanupTmp()
			return err
		}
	}
	if err := out.Close(); err != nil {
		cleanupTmp()
		return err
	}
	if err := os.Rename(tmp, output); err != nil {
		cleanupTmp()
		return err
	}

	st, _ := os.Stat(output)
	if st != nil {
		fmt.Printf("Backup written to %s (%d bytes)\n", output, st.Size())
	} else {
		fmt.Printf("Backup written to %s\n", output)
	}
	if mode == backup.ModeBundlePlain {
		fmt.Fprintln(os.Stderr, "\n\033[33mWARNING: this bundle is UNENCRYPTED and contains your CAIRN_ENCRYPTION_KEY. Anyone with this file can read all secrets.\033[0m")
	}
	return nil
}

const plainWarning = `Refusing to create an unencrypted bundle without --force.

An unencrypted bundle includes your encryption key alongside the encrypted database.
Anyone with the bundle file can read all your secrets. Storing it on a shared drive,
cloud sync, or backup service that you don't fully trust is a security risk.

If you understand this and still want to proceed, re-run with --force.`

func resolveDBPath(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("CAIRN_DB_PATH"); v != "" {
		return v
	}
	if cfg, err := config.Load(); err == nil && cfg.DatabasePath != "" {
		return cfg.DatabasePath
	}
	return "./cairn.db"
}

func readPassphraseConfirmed(prompt1, prompt2 string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		r := bufio.NewReader(os.Stdin)
		fmt.Fprint(os.Stderr, prompt1)
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, prompt1)
	a, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, prompt2)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(a) != string(b) {
		return "", errors.New("passphrases do not match")
	}
	return string(a), nil
}
