// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/TRC-Loop/cairn/internal/backup"
)

type BackupHandler struct {
	svc    *backup.Service
	logger *slog.Logger
}

func NewBackupHandler(svc *backup.Service, logger *slog.Logger) *BackupHandler {
	return &BackupHandler{svc: svc, logger: logger}
}

type backupDownloadRequest struct {
	Mode       string `json:"mode"`
	Passphrase string `json:"passphrase"`
}

func (h *BackupHandler) Download(w http.ResponseWriter, r *http.Request) {
	var req backupDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	mode := backup.Mode(strings.TrimSpace(req.Mode))
	fields := map[string]string{}
	switch mode {
	case backup.ModeDBOnly, backup.ModeBundlePlain, backup.ModeBundleEncrypted:
	case "":
		fields["mode"] = FieldRequired
	default:
		fields["mode"] = FieldInvalidValue
	}
	if mode == backup.ModeBundleEncrypted {
		if req.Passphrase == "" {
			fields["passphrase"] = FieldRequired
		} else if len(req.Passphrase) < 12 {
			fields["passphrase"] = FieldTooShort
		}
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	var (
		filename, contentType string
	)
	switch mode {
	case backup.ModeDBOnly:
		filename = fmt.Sprintf("cairn-backup-%s.db", stamp)
		contentType = "application/x-sqlite3"
	case backup.ModeBundleEncrypted:
		filename = fmt.Sprintf("cairn-backup-%s.cbackup", stamp)
		contentType = "application/octet-stream"
	case backup.ModeBundlePlain:
		filename = fmt.Sprintf("cairn-backup-%s.tar.gz", stamp)
		contentType = "application/gzip"
		h.logger.Warn("plain backup with key generated",
			"remote_ip", r.RemoteAddr)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	w.Header().Set("Cache-Control", "no-store")

	switch mode {
	case backup.ModeDBOnly:
		if _, err := h.svc.CreateDBOnly(r.Context(), w); err != nil {
			h.logger.Error("backup db-only failed", "err", err)
			return
		}
	case backup.ModeBundleEncrypted:
		if err := h.svc.CreateBundle(r.Context(), w, req.Passphrase); err != nil {
			h.logger.Error("backup encrypted bundle failed", "err", err)
			return
		}
	case backup.ModeBundlePlain:
		if err := h.svc.CreateBundle(r.Context(), w, ""); err != nil {
			h.logger.Error("backup plain bundle failed", "err", err)
			return
		}
	}
}
