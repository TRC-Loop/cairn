// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/TRC-Loop/cairn/internal/api"
	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/backup"
	"github.com/TRC-Loop/cairn/internal/check"
	"github.com/TRC-Loop/cairn/internal/component"
	"github.com/TRC-Loop/cairn/internal/config"
	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/incident"
	"github.com/TRC-Loop/cairn/internal/maintenance"
	"github.com/TRC-Loop/cairn/internal/notifier"
	"github.com/TRC-Loop/cairn/internal/rollup"
	"github.com/TRC-Loop/cairn/internal/statuspage"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/TRC-Loop/cairn/migrations"
	"github.com/pressly/goose/v3"
)

var (
	cairnVersion  = "0.1.0"
	cairnRevision = "unknown"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "create-admin":
			if err := runCreateAdminCmd(logger); err != nil {
				logger.Error("create-admin failed", "err", err)
				os.Exit(1)
			}
			return
		case "backup":
			if err := runBackupCmd(logger, os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "backup:", err)
				os.Exit(1)
			}
			return
		case "restore":
			if err := runRestoreCmd(logger, os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "restore:", err)
				os.Exit(1)
			}
			return
		case "healthcheck":
			if err := runHealthcheckCmd(); err != nil {
				fmt.Fprintln(os.Stderr, "healthcheck:", err)
				os.Exit(1)
			}
			return
		case "version", "--version", "-v":
			fmt.Printf("cairn %s (%s)\n", cairnVersion, cairnRevision)
			return
		}
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	db, err := store.Open(cfg.DatabasePath, cfg.SQLiteBusyTimeoutMs)
	if err != nil {
		logger.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("database connected", "path", cfg.DatabasePath)

	if os.Getenv("CAIRN_SKIP_MIGRATIONS") != "1" {
		if err := runMigrations(db); err != nil {
			logger.Error("migrations failed", "err", err)
			os.Exit(1)
		}
		logger.Info("migrations applied")
	}

	q := store.New(db)

	reg := check.NewRegistry()
	reg.Register(check.TypeHTTP, check.NewHTTPChecker())
	reg.Register(check.TypeTCP, &check.TCPChecker{})
	reg.Register(check.TypeICMP, &check.ICMPChecker{})
	reg.Register(check.TypeDNS, &check.DNSChecker{})
	reg.Register(check.TypeTLS, &check.TLSChecker{})
	reg.Register(check.TypeDBPostgres, &check.PostgresChecker{})
	reg.Register(check.TypeDBMySQL, &check.MySQLChecker{})
	reg.Register(check.TypeDBRedis, &check.RedisChecker{})
	reg.Register(check.TypeGRPC, &check.GRPCChecker{})
	reg.Register(check.TypePush, check.PushChecker{})
	logger.Info("registered checker types", "types", []string{
		string(check.TypeHTTP),
		string(check.TypeTCP),
		string(check.TypeICMP),
		string(check.TypeDNS),
		string(check.TypeTLS),
		string(check.TypeDBPostgres),
		string(check.TypeDBMySQL),
		string(check.TypeDBRedis),
		string(check.TypeGRPC),
		string(check.TypePush),
	})

	schedCtx, schedCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	secretBox, err := crypto.NewSecretBox(cfg.EncryptionKey)
	if err != nil {
		logger.Error("failed to init secret box", "err", err)
		os.Exit(1)
	}

	componentSvc := component.NewService(db, q, logger)
	statusPageSvc := statuspage.NewService(db, q, logger)
	maintenanceSvc := maintenance.NewService(db, q, logger)
	incidentSvc := incident.NewService(db, q, logger, maintenanceSvc)

	dispatcher := notifier.NewDispatcher(db, q, secretBox, maintenanceSvc, logger)
	incidentSvc.SetNotifier(notifier.NewIncidentAdapter(dispatcher))
	maintenanceSvc.SetNotifier(notifier.NewMaintenanceAdapter(dispatcher))

	statusPageHandler := statuspage.NewHandler(statusPageSvc, componentSvc, maintenanceSvc, incidentSvc, q, logger, cfg.EncryptionKey)

	sched := check.NewScheduler(db, q, reg, logger, incidentSvc)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := sched.Start(schedCtx); err != nil {
			logger.Error("scheduler failed", "err", err)
		}
	}()

	pushMon := check.NewPushMonitor(db, q, logger, incidentSvc)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := pushMon.Start(schedCtx); err != nil {
			logger.Error("push monitor failed", "err", err)
		}
	}()

	roll := rollup.New(db, q, logger, 15*time.Minute)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := roll.Start(schedCtx); err != nil {
			logger.Error("rollup failed", "err", err)
		}
	}()

	maintSched := maintenance.NewStateScheduler(q, logger)
	maintSched.SetNotifier(notifier.NewMaintenanceAdapter(dispatcher))
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := maintSched.Start(schedCtx); err != nil {
			logger.Error("maintenance state scheduler failed", "err", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := dispatcher.Start(schedCtx); err != nil {
			logger.Error("notification dispatcher failed", "err", err)
		}
	}()

	sessionSvc := auth.NewSessionService(q, logger)
	sessionCleanup := auth.NewSessionCleanup(sessionSvc, logger)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := sessionCleanup.Start(schedCtx); err != nil {
			logger.Error("session cleanup failed", "err", err)
		}
	}()
	authHandler := api.NewAuthHandler(q, sessionSvc, logger, cfg.BehindTLS)
	setupHandler := api.NewSetupHandler(q, db, sessionSvc, logger, cfg.BehindTLS)
	checksHandler := api.NewChecksHandler(q, db, logger)
	checksHandler.SetRunner(reg, incidentSvc)
	componentsHandler := api.NewComponentsHandler(q, componentSvc, logger)
	statusPagesHandler := api.NewStatusPagesHandler(q, statusPageSvc, logger)
	notificationsHandler := api.NewNotificationsHandler(q, db, secretBox, dispatcher, logger)
	incidentsHandler := api.NewIncidentsHandler(q, incidentSvc, logger)
	maintenanceHandler := api.NewMaintenanceHandler(q, maintenanceSvc, logger)
	usersHandler := api.NewUsersHandler(q, sessionSvc, logger)
	systemSettingsHandler := api.NewSystemSettingsHandler(q, logger)
	retentionSettingsHandler := api.NewRetentionSettingsHandler(q, logger)
	backupSvc := backup.NewService(db, cfg.DatabasePath, cfg.EncryptionKey, cairnVersion, logger)
	backupHandler := api.NewBackupHandler(backupSvc, logger)
	twofaHandler := api.NewTwoFAHandler(q, sessionSvc, secretBox, logger, cfg.BehindTLS)
	authHandler.SetTwoFA(twofaHandler)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewRouter(logger, db, q, incidentSvc, statusPageHandler, sessionSvc, authHandler, setupHandler, checksHandler, componentsHandler, statusPagesHandler, notificationsHandler, incidentsHandler, maintenanceHandler, usersHandler, systemSettingsHandler, retentionSettingsHandler, backupHandler, twofaHandler, cfg.BehindTLS),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("cairn listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	schedCancel()

	drainDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(drainDone)
	}()
	select {
	case <-drainDone:
		logger.Info("background workers drained")
	case <-time.After(30 * time.Second):
		logger.Warn("background worker drain timeout exceeded")
	}

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "err", err)
	}
}

func runMigrations(db *sql.DB) error {
	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
