// SPDX-License-Identifier: AGPL-3.0-or-later
package rollup

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

type Rollup struct {
	db       *sql.DB
	q        *store.Queries
	logger   *slog.Logger
	interval time.Duration
}

func New(db *sql.DB, q *store.Queries, logger *slog.Logger, interval time.Duration) *Rollup {
	return &Rollup{
		db:       db,
		q:        q,
		logger:   logger,
		interval: interval,
	}
}

func (r *Rollup) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	r.logger.Info("rollup started", "interval", r.interval.String())

	if err := r.RunOnce(ctx); err != nil {
		r.logger.Error("rollup initial run failed", "err", err)
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("rollup stopped")
			return nil
		case <-ticker.C:
			if err := r.RunOnce(ctx); err != nil {
				r.logger.Error("rollup run failed", "err", err)
			}
		}
	}
}

func (r *Rollup) RunOnce(ctx context.Context) error {
	start := time.Now()
	if err := r.rollHourly(ctx); err != nil {
		return fmt.Errorf("hourly: %w", err)
	}
	if err := r.rollDaily(ctx); err != nil {
		return fmt.Errorf("daily: %w", err)
	}
	if err := r.enforceRetention(ctx); err != nil {
		return fmt.Errorf("retention: %w", err)
	}
	r.logger.Info("rollup completed", "elapsed", time.Since(start).String())
	return nil
}

func (r *Rollup) rollHourly(ctx context.Context) error {
	checkIDs, err := r.q.ListChecksWithRawResults(ctx)
	if err != nil {
		return err
	}
	cutoff := time.Now().UTC().Add(-1 * time.Minute).Truncate(time.Hour)

	for _, id := range checkIDs {
		oldest, ok, err := queryMinTime(ctx, r.db, "SELECT MIN(checked_at) FROM check_results WHERE check_id = ?", id)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		bucket := oldest.UTC().Truncate(time.Hour)
		for bucket.Before(cutoff) {
			if err := r.rollHourBucket(ctx, id, bucket); err != nil {
				return err
			}
			bucket = bucket.Add(time.Hour)
		}
	}
	return nil
}

func (r *Rollup) rollHourBucket(ctx context.Context, checkID int64, bucketStart time.Time) error {
	bucketEnd := bucketStart.Add(time.Hour)
	rows, err := r.q.GetRawResultsForRollup(ctx, store.GetRawResultsForRollupParams{
		CheckID:     checkID,
		CheckedAt:   bucketStart,
		CheckedAt_2: bucketEnd,
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	agg := aggregateResults(rows)
	return r.q.UpsertCheckResultHourly(ctx, store.UpsertCheckResultHourlyParams{
		CheckID:       checkID,
		HourBucket:    bucketStart,
		TotalCount:    agg.total,
		UpCount:       agg.up,
		DegradedCount: agg.degraded,
		DownCount:     agg.down,
		UnknownCount:  agg.unknown,
		AvgLatencyMs:  nullFloat(agg.avg),
		MinLatencyMs:  nullInt(agg.min),
		MaxLatencyMs:  nullInt(agg.max),
		P95LatencyMs:  nullInt(agg.p95),
	})
}

func (r *Rollup) rollDaily(ctx context.Context) error {
	checkIDs, err := r.q.ListChecksWithHourly(ctx)
	if err != nil {
		return err
	}
	cutoff := time.Now().UTC().Add(-1 * time.Minute).Truncate(24 * time.Hour)

	for _, id := range checkIDs {
		oldest, ok, err := queryMinTime(ctx, r.db, "SELECT MIN(hour_bucket) FROM check_results_hourly WHERE check_id = ?", id)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		bucket := oldest.UTC().Truncate(24 * time.Hour)
		for bucket.Before(cutoff) {
			if err := r.rollDayBucket(ctx, id, bucket); err != nil {
				return err
			}
			bucket = bucket.Add(24 * time.Hour)
		}
	}
	return nil
}

func (r *Rollup) rollDayBucket(ctx context.Context, checkID int64, bucketStart time.Time) error {
	bucketEnd := bucketStart.Add(24 * time.Hour)
	rows, err := r.q.GetHourlyForRollup(ctx, store.GetHourlyForRollupParams{
		CheckID:      checkID,
		HourBucket:   bucketStart,
		HourBucket_2: bucketEnd,
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	var total, up, degraded, down, unknown int64
	var latencyWeighted float64
	var latencyWeight int64
	var minLat, maxLat *int64
	var p95Sum float64
	var p95Count int64

	for _, h := range rows {
		total += h.TotalCount
		up += h.UpCount
		degraded += h.DegradedCount
		down += h.DownCount
		unknown += h.UnknownCount
		if h.AvgLatencyMs.Valid {
			latencyWeighted += h.AvgLatencyMs.Float64 * float64(h.TotalCount)
			latencyWeight += h.TotalCount
		}
		if h.MinLatencyMs.Valid {
			v := h.MinLatencyMs.Int64
			if minLat == nil || v < *minLat {
				minLat = &v
			}
		}
		if h.MaxLatencyMs.Valid {
			v := h.MaxLatencyMs.Int64
			if maxLat == nil || v > *maxLat {
				maxLat = &v
			}
		}
		if h.P95LatencyMs.Valid {
			p95Sum += float64(h.P95LatencyMs.Int64)
			p95Count++
		}
	}

	var avg *float64
	if latencyWeight > 0 {
		v := latencyWeighted / float64(latencyWeight)
		avg = &v
	}
	var p95 *int64
	if p95Count > 0 {
		v := int64(math.Round(p95Sum / float64(p95Count)))
		p95 = &v
	}

	return r.q.UpsertCheckResultDaily(ctx, store.UpsertCheckResultDailyParams{
		CheckID:       checkID,
		DayBucket:     bucketStart,
		TotalCount:    total,
		UpCount:       up,
		DegradedCount: degraded,
		DownCount:     down,
		UnknownCount:  unknown,
		AvgLatencyMs:  nullFloat(avg),
		MinLatencyMs:  nullInt(minLat),
		MaxLatencyMs:  nullInt(maxLat),
		P95LatencyMs:  nullInt(p95),
	})
}

func (r *Rollup) enforceRetention(ctx context.Context) error {
	s, err := r.q.GetRetentionSettings(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := r.q.DeleteResultsOlderThan(ctx, now.AddDate(0, 0, -int(s.RawDays))); err != nil {
		return err
	}
	if err := r.q.DeleteHourlyOlderThan(ctx, now.AddDate(0, 0, -int(s.HourlyDays))); err != nil {
		return err
	}
	if !s.KeepDailyForever {
		if err := r.q.DeleteDailyOlderThan(ctx, now.AddDate(0, 0, -int(s.DailyDays))); err != nil {
			return err
		}
	}
	return nil
}

type hourAgg struct {
	total, up, degraded, down, unknown int64
	avg                                *float64
	min, max, p95                      *int64
}

func aggregateResults(rows []store.CheckResult) hourAgg {
	a := hourAgg{total: int64(len(rows))}
	var latencies []int64
	var sum float64
	for _, r := range rows {
		switch r.Status {
		case "up":
			a.up++
		case "degraded":
			a.degraded++
		case "down":
			a.down++
		default:
			a.unknown++
		}
		if r.LatencyMs.Valid {
			latencies = append(latencies, r.LatencyMs.Int64)
			sum += float64(r.LatencyMs.Int64)
		}
	}
	if len(latencies) > 0 {
		avg := sum / float64(len(latencies))
		a.avg = &avg
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		minV := latencies[0]
		maxV := latencies[len(latencies)-1]
		a.min = &minV
		a.max = &maxV
		var p95 int64
		if len(latencies) < 20 {
			p95 = maxV
		} else {
			idx := int(math.Ceil(0.95*float64(len(latencies)))) - 1
			if idx < 0 {
				idx = 0
			}
			if idx >= len(latencies) {
				idx = len(latencies) - 1
			}
			p95 = latencies[idx]
		}
		a.p95 = &p95
	}
	return a
}

func nullFloat(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func nullInt(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func queryMinTime(ctx context.Context, db *sql.DB, query string, args ...any) (time.Time, bool, error) {
	var raw any
	if err := db.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		return time.Time{}, false, err
	}
	if raw == nil {
		return time.Time{}, false, nil
	}
	switch v := raw.(type) {
	case time.Time:
		return v, true, nil
	case string:
		t, err := parseSQLiteTime(v)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("parse time %q: %w", v, err)
		}
		return t, true, nil
	case []byte:
		t, err := parseSQLiteTime(string(v))
		if err != nil {
			return time.Time{}, false, fmt.Errorf("parse time %q: %w", string(v), err)
		}
		return t, true, nil
	default:
		return time.Time{}, false, fmt.Errorf("unsupported time scan type %T", raw)
	}
}

var sqliteTimeFormats = []string{
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05 -0700 MST",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999Z07:00",
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.999999999Z",
	"2006-01-02T15:04:05Z",
}

func parseSQLiteTime(s string) (time.Time, error) {
	for _, layout := range sqliteTimeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time layout")
}
