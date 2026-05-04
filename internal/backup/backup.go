// SPDX-License-Identifier: AGPL-3.0-or-later
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

type Mode string

const (
	ModeDBOnly          Mode = "db_only"
	ModeBundleEncrypted Mode = "bundle_encrypted"
	ModeBundlePlain     Mode = "bundle_plain"
)

const (
	FormatVersion       = "1"
	BundleMagic         = "CRBP"
	BundleHeaderVersion = byte(1)
	pbkdf2Iterations    = 600_000
	saltSize            = 16
	keySize             = 32
	nonceSize           = 12

	tarDBName       = "cairn.db"
	tarKeyName      = "encryption.key"
	tarManifestName = "manifest.json"
)

var (
	ErrUnsupportedFormat = errors.New("backup format version not supported")
	ErrBadPassphrase     = errors.New("decryption failed: wrong passphrase or corrupted bundle")
	ErrNotABundle        = errors.New("input is not a Cairn bundle")
	ErrIntegrityCheck    = errors.New("sqlite integrity_check failed")
)

type Manifest struct {
	Version       string    `json:"version"`
	CairnVersion  string    `json:"cairn_version"`
	CreatedAt     time.Time `json:"created_at"`
	Mode          Mode      `json:"mode"`
	HasKey        bool      `json:"has_key"`
	SchemaVersion int64     `json:"schema_version"`
}

type Service struct {
	db           *sql.DB
	dbPath       string
	masterKey    string
	cairnVersion string
	logger       *slog.Logger
}

func NewService(db *sql.DB, dbPath, masterKey, cairnVersion string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		db:           db,
		dbPath:       dbPath,
		masterKey:    masterKey,
		cairnVersion: cairnVersion,
		logger:       logger,
	}
}

// snapshotDB writes a consistent snapshot of the live database to dst using
// SQLite VACUUM INTO. Equivalent to the online backup API for our purposes:
// it produces a defragmented copy that is consistent with concurrent writes.
func (s *Service) snapshotDB(ctx context.Context, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		_ = os.Remove(dst)
	}
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	return nil
}

func (s *Service) schemaVersion(ctx context.Context) int64 {
	var v sql.NullInt64
	row := s.db.QueryRowContext(ctx, "SELECT MAX(version_id) FROM goose_db_version")
	if err := row.Scan(&v); err != nil {
		return 0
	}
	if v.Valid {
		return v.Int64
	}
	return 0
}

// CreateDBOnly writes the SQLite DB snapshot to w.
func (s *Service) CreateDBOnly(ctx context.Context, w io.Writer) (int64, error) {
	tmp, err := os.CreateTemp("", "cairn-backup-*.db")
	if err != nil {
		return 0, fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + "-journal")
	defer os.Remove(tmpPath + "-wal")
	defer os.Remove(tmpPath + "-shm")

	if err := s.snapshotDB(ctx, tmpPath); err != nil {
		return 0, err
	}
	f, err := os.Open(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("open snapshot: %w", err)
	}
	defer f.Close()
	n, err := io.Copy(w, f)
	if err != nil {
		return n, fmt.Errorf("copy: %w", err)
	}
	return n, nil
}

// CreateBundle creates a tar.gz containing cairn.db, encryption.key, and
// manifest.json. If passphrase is non-empty, the entire archive is wrapped
// in AES-256-GCM with PBKDF2 key derivation.
func (s *Service) CreateBundle(ctx context.Context, w io.Writer, passphrase string) error {
	tmp, err := os.CreateTemp("", "cairn-backup-*.db")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + "-journal")
	defer os.Remove(tmpPath + "-wal")
	defer os.Remove(tmpPath + "-shm")

	if err := s.snapshotDB(ctx, tmpPath); err != nil {
		return err
	}
	dbBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}

	mode := ModeBundlePlain
	if passphrase != "" {
		mode = ModeBundleEncrypted
	}
	manifest := Manifest{
		Version:       FormatVersion,
		CairnVersion:  s.cairnVersion,
		CreatedAt:     time.Now().UTC(),
		Mode:          mode,
		HasKey:        s.masterKey != "",
		SchemaVersion: s.schemaVersion(ctx),
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	var archive io.Writer = w
	var encBuf *bytes.Buffer
	if passphrase != "" {
		encBuf = &bytes.Buffer{}
		archive = encBuf
	}

	gz := gzip.NewWriter(archive)
	tw := tar.NewWriter(gz)

	if err := writeTarEntry(tw, tarDBName, dbBytes); err != nil {
		return err
	}
	if err := writeTarEntry(tw, tarKeyName, []byte(s.masterKey)); err != nil {
		return err
	}
	if err := writeTarEntry(tw, tarManifestName, manifestBytes); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("tar close: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("gzip close: %w", err)
	}
	if encBuf != nil {
		return encryptAndWrite(w, encBuf.Bytes(), passphrase)
	}
	return nil
}

func writeTarEntry(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o600,
		Size:    int64(len(data)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("tar header %s: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("tar write %s: %w", name, err)
	}
	return nil
}

// RestoreDBOnly atomically replaces the live DB at s.dbPath with the contents of r.
// Caller must close s.db before calling and reopen after.
func (s *Service) RestoreDBOnly(ctx context.Context, r io.Reader) error {
	return RestoreDBFile(ctx, s.dbPath, r)
}

// RestoreDBFile atomically replaces the file at dbPath with the contents of r,
// after running PRAGMA integrity_check on the incoming data.
func RestoreDBFile(ctx context.Context, dbPath string, r io.Reader) error {
	tmpPath := dbPath + ".restore.tmp"
	tmp, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("copy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}

	if err := verifyIntegrity(ctx, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		_ = os.Remove(dbPath + suffix)
	}
	if err := os.Rename(tmpPath, dbPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// verifyIntegrity opens the SQLite file at path and runs PRAGMA integrity_check.
func verifyIntegrity(ctx context.Context, path string) error {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&mode=ro", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("%w: open: %v", ErrIntegrityCheck, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var result string
	row := db.QueryRowContext(ctx, "PRAGMA integrity_check")
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("%w: %v", ErrIntegrityCheck, err)
	}
	if result != "ok" {
		return fmt.Errorf("%w: %s", ErrIntegrityCheck, result)
	}
	return nil
}

// RestoreBundle parses an encrypted or plain bundle, validates the manifest,
// extracts the key, and atomically replaces the live DB at s.dbPath.
// Returns the manifest and the encryption key bytes.
func (s *Service) RestoreBundle(ctx context.Context, r io.Reader, passphrase string) (*Manifest, []byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read input: %w", err)
	}
	plain, err := MaybeDecryptBundle(data, passphrase)
	if err != nil {
		return nil, nil, err
	}

	manifest, dbBytes, keyBytes, err := readBundle(plain)
	if err != nil {
		return nil, nil, err
	}
	if err := validateManifest(manifest, s.cairnVersion, s.logger); err != nil {
		return manifest, nil, err
	}
	if err := RestoreDBFile(ctx, s.dbPath, bytes.NewReader(dbBytes)); err != nil {
		return manifest, nil, err
	}
	return manifest, keyBytes, nil
}

// MaybeDecryptBundle returns the plain tar.gz bytes for an input that may be
// encrypted (with magic header) or already plain. If encrypted, passphrase
// must be non-empty.
func MaybeDecryptBundle(data []byte, passphrase string) ([]byte, error) {
	if len(data) >= 4 && string(data[:4]) == BundleMagic {
		if passphrase == "" {
			return nil, errors.New("passphrase required for encrypted bundle")
		}
		return decryptBundle(data, passphrase)
	}
	return data, nil
}

// readBundle parses a plain tar.gz produced by CreateBundle.
func readBundle(plain []byte) (*Manifest, []byte, []byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(plain))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: gzip: %v", ErrNotABundle, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var manifest *Manifest
	var dbBytes, keyBytes []byte
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%w: tar: %v", ErrNotABundle, err)
		}
		buf, err := io.ReadAll(tr)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read %s: %w", hdr.Name, err)
		}
		switch hdr.Name {
		case tarDBName:
			dbBytes = buf
		case tarKeyName:
			keyBytes = buf
		case tarManifestName:
			var m Manifest
			if err := json.Unmarshal(buf, &m); err != nil {
				return nil, nil, nil, fmt.Errorf("manifest json: %w", err)
			}
			manifest = &m
		}
	}
	if manifest == nil {
		return nil, nil, nil, fmt.Errorf("%w: missing manifest", ErrNotABundle)
	}
	if dbBytes == nil {
		return nil, nil, nil, fmt.Errorf("%w: missing database", ErrNotABundle)
	}
	return manifest, dbBytes, keyBytes, nil
}

func validateManifest(m *Manifest, currentVersion string, logger *slog.Logger) error {
	if m.Version != FormatVersion {
		return fmt.Errorf("%w: backup format %q, this build supports %q", ErrUnsupportedFormat, m.Version, FormatVersion)
	}
	if currentVersion != "" && m.CairnVersion != "" && m.CairnVersion != currentVersion {
		logger.Warn("backup cairn version differs from current",
			"backup_version", m.CairnVersion,
			"current_version", currentVersion)
	}
	logger.Info("restore manifest", "schema_version", m.SchemaVersion, "created_at", m.CreatedAt)
	return nil
}

// --- encryption helpers ---

func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, keySize, sha256.New)
}

func encryptAndWrite(w io.Writer, plaintext []byte, passphrase string) error {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("rand salt: %w", err)
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("rand nonce: %w", err)
	}
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("gcm: %w", err)
	}
	ct := gcm.Seal(nil, nonce, plaintext, []byte(BundleMagic))

	header := []byte(BundleMagic)
	header = append(header, BundleHeaderVersion)
	header = append(header, salt...)
	header = append(header, nonce...)
	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := w.Write(ct); err != nil {
		return fmt.Errorf("write ct: %w", err)
	}
	return nil
}

func decryptBundle(data []byte, passphrase string) ([]byte, error) {
	headerLen := 4 + 1 + saltSize + nonceSize
	if len(data) < headerLen+16 {
		return nil, fmt.Errorf("%w: truncated", ErrBadPassphrase)
	}
	if string(data[:4]) != BundleMagic {
		return nil, ErrNotABundle
	}
	if data[4] != BundleHeaderVersion {
		return nil, fmt.Errorf("unsupported bundle header version: %d", data[4])
	}
	salt := data[5 : 5+saltSize]
	nonce := data[5+saltSize : 5+saltSize+nonceSize]
	ct := data[headerLen:]
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	pt, err := gcm.Open(nil, nonce, ct, []byte(BundleMagic))
	if err != nil {
		return nil, ErrBadPassphrase
	}
	return pt, nil
}

// DetectKind inspects the leading bytes of a backup file to identify its type.
type Kind int

const (
	KindUnknown Kind = iota
	KindDBOnly
	KindBundlePlain
	KindBundleEncrypted
)

// DetectKind reads up to 16 bytes of header. The reader returned wraps the
// input so callers can re-read from the start.
func DetectKind(head []byte) Kind {
	if len(head) >= 4 && string(head[:4]) == BundleMagic {
		return KindBundleEncrypted
	}
	if len(head) >= 16 && string(head[:16]) == "SQLite format 3\x00" {
		return KindDBOnly
	}
	if len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b {
		return KindBundlePlain
	}
	return KindUnknown
}

// SuggestFilename returns a default filename for the given mode and timestamp.
func SuggestFilename(mode Mode, t time.Time) string {
	stamp := t.UTC().Format("20060102-150405")
	switch mode {
	case ModeDBOnly:
		return filepath.Join(".", fmt.Sprintf("cairn-backup-%s.db", stamp))
	case ModeBundleEncrypted:
		return filepath.Join(".", fmt.Sprintf("cairn-backup-%s.cbackup", stamp))
	case ModeBundlePlain:
		return filepath.Join(".", fmt.Sprintf("cairn-backup-%s.tar.gz", stamp))
	}
	return fmt.Sprintf("cairn-backup-%s", stamp)
}

