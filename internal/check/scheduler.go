// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

var defaultPoolSizes = map[Type]int{
	TypeHTTP:       50,
	TypeTCP:        20,
	TypeDNS:        10,
	TypeICMP:       10,
	TypeTLS:        10,
	TypeDBPostgres: 5,
	TypeDBMySQL:    5,
	TypeDBRedis:    5,
	TypeGRPC:       5,
	TypePush:       0,
}

const shutdownGrace = 15 * time.Second

type Scheduler struct {
	db          *sql.DB
	q           *store.Queries
	registry    *Registry
	logger      *slog.Logger
	pools       map[Type]*workerPool
	tickEvery   time.Duration
	incidentSvc IncidentService
}

func NewScheduler(db *sql.DB, q *store.Queries, registry *Registry, logger *slog.Logger, incidentSvc IncidentService) *Scheduler {
	return &Scheduler{
		db:          db,
		q:           q,
		registry:    registry,
		logger:      logger,
		tickEvery:   1 * time.Second,
		incidentSvc: incidentSvc,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	wg := &sync.WaitGroup{}
	s.pools = make(map[Type]*workerPool, len(defaultPoolSizes))
	for t, size := range defaultPoolSizes {
		if size <= 0 {
			continue
		}
		s.pools[t] = newWorkerPool(size, wg, s.registry, s.db, s.q, s.incidentSvc, s.logger)
	}

	ticker := time.NewTicker(s.tickEvery)
	defer ticker.Stop()

	s.logger.Info("scheduler started", "tick", s.tickEvery.String(), "pools", len(s.pools))

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("draining scheduler")
			for _, p := range s.pools {
				close(p.jobs)
			}
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			select {
			case <-done:
				s.logger.Info("scheduler drained")
			case <-time.After(shutdownGrace):
				s.logger.Warn("scheduler drain timeout exceeded", "grace", shutdownGrace.String())
			}
			return nil
		case <-ticker.C:
			s.dispatch(ctx)
		}
	}
}

func (s *Scheduler) dispatch(ctx context.Context) {
	due, err := s.q.ListEnabledChecksDue(ctx)
	if err != nil {
		s.logger.Error("list due checks failed", "err", err)
		return
	}
	for _, chk := range due {
		t := Type(chk.Type)
		if _, ok := s.registry.Get(t); !ok {
			s.logger.Warn("unknown check type", "id", chk.ID, "type", chk.Type)
			continue
		}
		pool, ok := s.pools[t]
		if !ok {
			s.logger.Warn("no pool for check type", "id", chk.ID, "type", chk.Type)
			continue
		}
		timeout := time.Duration(chk.TimeoutSeconds) * time.Second
		job := checkJob{check: chk, deadline: time.Now().Add(timeout)}
		if !pool.submit(job) {
			s.logger.Debug("pool full, skipping", "id", chk.ID, "type", chk.Type)
		}
	}
}

type checkJob struct {
	check    store.Check
	deadline time.Time
}

type workerPool struct {
	jobs chan checkJob
	wg   *sync.WaitGroup
}

func (p *workerPool) submit(job checkJob) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

func newWorkerPool(size int, wg *sync.WaitGroup, registry *Registry, db *sql.DB, q *store.Queries, incidentSvc IncidentService, logger *slog.Logger) *workerPool {
	pool := &workerPool{
		jobs: make(chan checkJob, size),
		wg:   wg,
	}
	for i := 0; i < size; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range pool.jobs {
				runJob(job, registry, db, q, incidentSvc, logger)
			}
		}()
	}
	return pool
}

func runJob(job checkJob, registry *Registry, db *sql.DB, q *store.Queries, incidentSvc IncidentService, logger *slog.Logger) {
	ctx, cancel := context.WithDeadline(context.Background(), job.deadline)
	defer cancel()

	checker, ok := registry.Get(Type(job.check.Type))
	if !ok {
		logger.Warn("no checker for job type", "id", job.check.ID, "type", job.check.Type)
		return
	}

	result := checker.Run(ctx, []byte(job.check.ConfigJson))

	persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer persistCancel()
	if err := PersistResult(persistCtx, db, q, incidentSvc, job.check, result); err != nil {
		logger.Error("persist failed", "id", job.check.ID, "err", err)
	}
}
