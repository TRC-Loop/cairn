// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/TRC-Loop/cairn/internal/backup"
	"golang.org/x/term"
)

func nowUTC() time.Time { return time.Now().UTC() }

func runRestoreCmd(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	var (
		input      string
		passphrase string
		dbPathFlag string
		force      bool
		keyOutput  string
	)
	fs.StringVar(&input, "i", "", "backup file to restore")
	fs.StringVar(&input, "input", "", "backup file to restore")
	fs.StringVar(&passphrase, "passphrase", "", "decrypt with this passphrase (prompts if needed)")
	fs.StringVar(&dbPathFlag, "db", "", "path to cairn.db (default: $CAIRN_DB_PATH or ./cairn.db)")
	fs.BoolVar(&force, "force", false, "skip 'DB exists' confirmation")
	fs.StringVar(&keyOutput, "key-output", "", "where to write the encryption key from a bundle")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if input == "" {
		return errors.New("--input/-i required")
	}

	dbPath := resolveDBPath(dbPathFlag)
	if _, err := os.Stat(dbPath + "-wal"); err == nil {
		return errors.New("cannot restore: the database appears to be in use (WAL file exists). Stop Cairn first, then re-run")
	}

	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	kind := backup.DetectKind(data)
	if kind == backup.KindUnknown {
		return errors.New("could not identify backup type (not SQLite, gzip, or Cairn bundle)")
	}

	if _, err := os.Stat(dbPath); err == nil && !force {
		fmt.Printf("DB at %s exists. Overwrite? [y/N] ", dbPath)
		r := bufio.NewReader(os.Stdin)
		ans, _ := r.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			return errors.New("aborted")
		}
	}

	switch kind {
	case backup.KindDBOnly:
		if err := backup.RestoreDBFile(context.Background(), dbPath, openInput(input)); err != nil {
			return err
		}
		fmt.Println("Restore complete. Start Cairn normally.")
		return nil
	case backup.KindBundleEncrypted:
		if passphrase == "" {
			p, err := readPassphraseSingle("Bundle passphrase: ")
			if err != nil {
				return err
			}
			passphrase = p
		}
	}

	// bundle path (encrypted or plain): use Service.RestoreBundle
	svc := backup.NewService(nil, dbPath, "", cairnVersion, logger)
	manifest, key, err := svc.RestoreBundle(context.Background(), openInput(input), passphrase)
	if err != nil {
		return err
	}
	if err := emitKey(key, keyOutput); err != nil {
		return err
	}
	fmt.Printf("Restore complete from backup created %s. Start Cairn with the displayed encryption key.\n", manifest.CreatedAt.Format(time.RFC3339))
	return nil
}

func openInput(path string) io.Reader {
	f, err := os.Open(path)
	if err != nil {
		return strings.NewReader("")
	}
	return f
}

func emitKey(key []byte, out string) error {
	if len(key) == 0 {
		fmt.Println("(no encryption key was bundled in the backup)")
		return nil
	}
	if out != "" {
		if err := os.WriteFile(out, key, 0o600); err != nil {
			return fmt.Errorf("write key: %w", err)
		}
		fmt.Printf("Encryption key written to %s (mode 0600). Set CAIRN_ENCRYPTION_KEY from its contents.\n", out)
		return nil
	}
	bar := strings.Repeat("=", 80)
	fmt.Println(bar)
	fmt.Println("ENCRYPTION KEY (set this as CAIRN_ENCRYPTION_KEY before starting Cairn):")
	fmt.Println()
	fmt.Println("  " + string(key))
	fmt.Println()
	fmt.Println("This key is required to decrypt notification channel secrets and similar data")
	fmt.Println("in the restored database. Store it in a password manager or secrets vault.")
	fmt.Println("Without this key, the database is largely usable but encrypted fields are lost.")
	fmt.Println(bar)
	return nil
}

func readPassphraseSingle(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		r := bufio.NewReader(os.Stdin)
		fmt.Fprint(os.Stderr, prompt)
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
