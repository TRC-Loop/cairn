// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	pathpkg "path"
	"strings"
	"time"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/check"
	"github.com/TRC-Loop/cairn/internal/spa"
	"github.com/TRC-Loop/cairn/internal/statuspage"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(
	logger *slog.Logger,
	db *sql.DB,
	q *store.Queries,
	incidentSvc check.IncidentService,
	statusPageHandler *statuspage.Handler,
	sessionSvc *auth.SessionService,
	authHandler *AuthHandler,
	setupHandler *SetupHandler,
	checksHandler *ChecksHandler,
	componentsHandler *ComponentsHandler,
	statusPagesHandler *StatusPagesHandler,
	statusPageDomainsHandler *StatusPageDomainsHandler,
	notificationsHandler *NotificationsHandler,
	incidentsHandler *IncidentsHandler,
	maintenanceHandler *MaintenanceHandler,
	usersHandler *UsersHandler,
	systemSettingsHandler *SystemSettingsHandler,
	retentionSettingsHandler *RetentionSettingsHandler,
	backupHandler *BackupHandler,
	twofaHandler *TwoFAHandler,
	behindTLS bool,
	version, revision string,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz)
	r.Get("/api/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": version, "revision": revision})
	})

	r.Post("/push/{token}", pushHandler(logger, db, q, incidentSvc))

	r.Route("/api", func(r chi.Router) {
		r.Use(auth.CSRFMiddleware(logger, behindTLS))
		if setupHandler != nil {
			r.Get("/setup/status", setupHandler.Status)
			r.Post("/setup/complete", setupHandler.Complete)
		}
		if authHandler != nil {
			r.Post("/auth/login", authHandler.Login)
			r.Post("/auth/logout", authHandler.Logout)
			if twofaHandler != nil {
				r.Post("/auth/login/2fa", twofaHandler.LoginVerify)
			}
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAuth(sessionSvc, logger))
				r.Get("/auth/me", authHandler.Me)
				r.Get("/status/summary", statusSummaryHandler(q, logger))
				r.Post("/preview-markdown", previewMarkdown)
				if twofaHandler != nil {
					r.Get("/auth/2fa/status", twofaHandler.Status)
					r.Post("/auth/2fa/setup", twofaHandler.Setup)
					r.Post("/auth/2fa/confirm", twofaHandler.Confirm)
					r.Post("/auth/2fa/disable", twofaHandler.Disable)
					r.Post("/auth/2fa/regenerate-recovery-codes", twofaHandler.Regenerate)
				}

				if checksHandler != nil {
					r.Get("/monitors", checksHandler.List)
					r.Get("/monitors/{id}", checksHandler.Get)
					r.Get("/monitors/{id}/results", checksHandler.RecentResults)
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleEditor, logger))
						r.Post("/monitors", checksHandler.Create)
						r.Patch("/monitors/{id}", checksHandler.Update)
						r.Delete("/monitors/{id}", checksHandler.Delete)
					})

					r.Group(func(r chi.Router) {
						r.Use(deprecatedChecksMiddleware(logger))
						r.Get("/checks", checksHandler.List)
						r.Get("/checks/{id}", checksHandler.Get)
						r.Get("/checks/{id}/results", checksHandler.RecentResults)
						r.Group(func(r chi.Router) {
							r.Use(auth.RequireRole(auth.RoleEditor, logger))
							r.Post("/checks", checksHandler.Create)
							r.Patch("/checks/{id}", checksHandler.Update)
							r.Delete("/checks/{id}", checksHandler.Delete)
						})
					})
				}

				if componentsHandler != nil {
					r.Get("/components", componentsHandler.List)
					r.Get("/components/{id}", componentsHandler.Get)
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleEditor, logger))
						r.Post("/components", componentsHandler.Create)
						r.Patch("/components/{id}", componentsHandler.Update)
						r.Delete("/components/{id}", componentsHandler.Delete)
						r.Post("/components/{id}/reorder", componentsHandler.Reorder)
					})
				}

				if notificationsHandler != nil {
					r.Get("/notification-channels", notificationsHandler.List)
					r.Get("/notification-channels/{id}", notificationsHandler.Get)
					r.Get("/notification-channels/{id}/deliveries", notificationsHandler.ListDeliveries)
					r.Get("/notification-deliveries/{id}", notificationsHandler.GetDelivery)
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleEditor, logger))
						r.Post("/notification-channels", notificationsHandler.Create)
						r.Patch("/notification-channels/{id}", notificationsHandler.Update)
						r.Delete("/notification-channels/{id}", notificationsHandler.Delete)
						r.Post("/notification-channels/{id}/test", notificationsHandler.Test)
					})
				}

				if incidentsHandler != nil {
					r.Get("/incidents", incidentsHandler.List)
					r.Get("/incidents/{id}", incidentsHandler.Get)
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleEditor, logger))
						r.Post("/incidents", incidentsHandler.Create)
						r.Patch("/incidents/{id}", incidentsHandler.Update)
						r.Delete("/incidents/{id}", incidentsHandler.Delete)
						r.Post("/incidents/{id}/updates", incidentsHandler.AddUpdate)
						r.Post("/incidents/{id}/affected-checks", incidentsHandler.AddAffected)
						r.Delete("/incidents/{id}/affected-checks/{check_id}", incidentsHandler.RemoveAffected)
					})
				}

				if maintenanceHandler != nil {
					r.Get("/maintenance", maintenanceHandler.List)
					r.Get("/maintenance/{id}", maintenanceHandler.Get)
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleEditor, logger))
						r.Post("/maintenance", maintenanceHandler.Create)
						r.Patch("/maintenance/{id}", maintenanceHandler.Update)
						r.Delete("/maintenance/{id}", maintenanceHandler.Delete)
						r.Post("/maintenance/{id}/cancel", maintenanceHandler.Cancel)
						r.Post("/maintenance/{id}/end-now", maintenanceHandler.EndNow)
					})
				}

				if usersHandler != nil {
					r.Get("/users", usersHandler.List)
					r.Get("/users/{id}", usersHandler.Get)
					r.Patch("/users/{id}", usersHandler.Update)
					r.Post("/users/{id}/password", usersHandler.ChangePassword)
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleAdmin, logger))
						r.Post("/users", usersHandler.Create)
						r.Delete("/users/{id}", usersHandler.Delete)
						if twofaHandler != nil {
							r.Post("/users/{id}/reset-2fa", twofaHandler.AdminReset)
						}
					})
				}

				if systemSettingsHandler != nil {
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleAdmin, logger))
						r.Get("/system-settings", systemSettingsHandler.Get)
						r.Patch("/system-settings", systemSettingsHandler.Update)
					})
				}

				if backupHandler != nil {
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleAdmin, logger))
						r.Post("/backup/download", backupHandler.Download)
					})
				}

				if retentionSettingsHandler != nil {
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleAdmin, logger))
						r.Get("/retention-settings", retentionSettingsHandler.Get)
						r.Patch("/retention-settings", retentionSettingsHandler.Update)
					})
				}

				if statusPagesHandler != nil {
					r.Get("/status-pages", statusPagesHandler.List)
					r.Get("/status-pages/{id}", statusPagesHandler.Get)
					r.Get("/status-pages/{id}/footer", statusPagesHandler.GetFooter)
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleEditor, logger))
						r.Post("/status-pages", statusPagesHandler.Create)
						r.Patch("/status-pages/{id}", statusPagesHandler.Update)
						r.Delete("/status-pages/{id}", statusPagesHandler.Delete)
						r.Post("/status-pages/{id}/default", statusPagesHandler.SetDefault)
						r.Post("/status-pages/{id}/password", statusPagesHandler.SetPassword)
						r.Put("/status-pages/{id}/components", statusPagesHandler.SetComponents)
						r.Put("/status-pages/{id}/footer/elements", statusPagesHandler.ReplaceFooterElements)
						r.Put("/status-pages/{id}/footer/mode", statusPagesHandler.SetFooterMode)
						if statusPageDomainsHandler != nil {
							r.Get("/status-pages/{id}/domains", statusPageDomainsHandler.List)
							r.Post("/status-pages/{id}/domains", statusPageDomainsHandler.Add)
							r.Delete("/status-pages/{id}/domains/{domain_id}", statusPageDomainsHandler.Delete)
						}
					})
				}
			})
		}
	})

	if statusPageHandler != nil {
		r.Method("GET", "/static/*", http.StripPrefix("/static/", statuspage.StaticHandler()))
		r.Get("/", statusPageHandler.ServeDefault)
		r.Route("/p/{slug}", func(r chi.Router) {
			r.Get("/", statusPageHandler.ServeBySlug)
			r.Post("/unlock", statusPageHandler.HandleUnlock)
			r.Get("/api.json", statusPageHandler.ServeJSON)
			r.Group(func(r chi.Router) {
				r.Use(embedFramingMiddleware)
				r.Get("/embed.js", statusPageHandler.ServeEmbedScript)
				r.Get("/embed", statusPageHandler.ServeEmbed)
			})
		})
	}

	spaFS, err := fs.Sub(spa.FS, "dist")
	if err != nil {
		logger.Error("spa fs init failed", "err", err)
	} else {
		fileServer := http.FileServer(http.FS(spaFS))
		r.NotFound(func(w http.ResponseWriter, req *http.Request) {
			serveSPA(w, req, spaFS, fileServer)
		})
	}

	return r
}

func serveSPA(w http.ResponseWriter, r *http.Request, spaFS fs.FS, fileServer http.Handler) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	f, err := spaFS.Open(path)
	if err != nil {
		// Asset paths (JS, CSS, fonts, images) that are missing should return 404,
		// not the HTML fallback — serving HTML for a JS import causes a MIME type error.
		if ext := pathpkg.Ext(path); ext != "" && ext != ".html" {
			http.NotFound(w, r)
			return
		}
		fallback, ferr := spaFS.Open("200.html")
		if ferr != nil {
			http.NotFound(w, r)
			return
		}
		defer fallback.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.Copy(w, fallback)
		return
	}
	f.Close()

	if strings.HasPrefix(path, "_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if strings.HasSuffix(path, ".html") {
		w.Header().Set("Cache-Control", "no-cache")
	}
	fileServer.ServeHTTP(w, r)
}

func deprecatedChecksMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Warn("deprecated path", "path", r.URL.Path, "hint", "use /api/monitors/*")
			next.ServeHTTP(w, r)
		})
	}
}

// embedFramingMiddleware strips X-Frame-Options at header-commit time so
// the iframe content is frameable regardless of what upstream handlers or
// global middleware set. Headers set after WriteHeader cannot be removed
// from the response, so we intercept via a wrapped ResponseWriter.
func embedFramingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&frameStripper{ResponseWriter: w}, r)
	})
}

type frameStripper struct {
	http.ResponseWriter
	wrote bool
}

func (f *frameStripper) WriteHeader(code int) {
	if !f.wrote {
		f.ResponseWriter.Header().Del("X-Frame-Options")
		f.wrote = true
	}
	f.ResponseWriter.WriteHeader(code)
}

func (f *frameStripper) Write(b []byte) (int, error) {
	if !f.wrote {
		f.ResponseWriter.Header().Del("X-Frame-Options")
		f.wrote = true
	}
	return f.ResponseWriter.Write(b)
}

func healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func pushHandler(logger *slog.Logger, db *sql.DB, q *store.Queries, incidentSvc check.IncidentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		if token == "" {
			writeError(w, http.StatusNotFound, CodeNotFound, "unknown token", nil)
			return
		}
		c, err := q.GetCheckByPushToken(r.Context(), sql.NullString{String: token, Valid: true})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, CodeNotFound, "unknown token", nil)
				return
			}
			logger.Error("push lookup failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}

		remoteIP := stripPort(r.RemoteAddr)
		res := check.Result{
			Status: check.StatusUp,
			Metadata: map[string]any{
				"received_at": time.Now().UTC().Format(time.RFC3339Nano),
				"remote_ip":   remoteIP,
			},
		}
		if err := check.PersistResult(r.Context(), db, q, incidentSvc, c, res); err != nil {
			logger.Error("push persist failed", "id", c.ID, "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "persist failed", nil)
			return
		}
		logger.Debug("push received", "id", c.ID, "remote_ip", remoteIP)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func stripPort(addr string) string {
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
