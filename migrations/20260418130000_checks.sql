-- +goose Up
-- +goose StatementBegin
CREATE TABLE components (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT NOT NULL,
    description   TEXT,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE checks (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    name                  TEXT NOT NULL,
    type                  TEXT NOT NULL CHECK(type IN ('http','tcp','icmp','dns','tls','push','db_postgres','db_mysql','db_redis','grpc')),
    enabled               BOOLEAN NOT NULL DEFAULT 1,
    interval_seconds      INTEGER NOT NULL DEFAULT 60 CHECK(interval_seconds >= 10),
    timeout_seconds       INTEGER NOT NULL DEFAULT 10,
    retries               INTEGER NOT NULL DEFAULT 0,
    failure_threshold     INTEGER NOT NULL DEFAULT 3,
    recovery_threshold    INTEGER NOT NULL DEFAULT 1,
    config_json           TEXT NOT NULL DEFAULT '{}',
    component_id          INTEGER REFERENCES components(id) ON DELETE SET NULL,
    last_status           TEXT NOT NULL DEFAULT 'unknown' CHECK(last_status IN ('up','degraded','down','unknown')),
    last_latency_ms       INTEGER,
    last_checked_at       DATETIME,
    consecutive_failures  INTEGER NOT NULL DEFAULT 0,
    consecutive_successes INTEGER NOT NULL DEFAULT 0,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_checks_enabled_last_checked ON checks(enabled, last_checked_at);
CREATE INDEX idx_checks_type                 ON checks(type);
CREATE INDEX idx_checks_component_id         ON checks(component_id);

CREATE TABLE check_results (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    check_id               INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
    checked_at             DATETIME NOT NULL,
    status                 TEXT NOT NULL CHECK(status IN ('up','degraded','down','unknown')),
    latency_ms             INTEGER,
    error_message          TEXT,
    response_metadata_json TEXT
);

CREATE INDEX idx_check_results_check_id_checked_at ON check_results(check_id, checked_at DESC);
CREATE INDEX idx_check_results_checked_at          ON check_results(checked_at);

CREATE TABLE check_results_hourly (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    check_id         INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
    hour_bucket      DATETIME NOT NULL,
    total_count      INTEGER NOT NULL,
    up_count         INTEGER NOT NULL,
    degraded_count   INTEGER NOT NULL,
    down_count       INTEGER NOT NULL,
    unknown_count    INTEGER NOT NULL,
    avg_latency_ms   REAL,
    min_latency_ms   INTEGER,
    max_latency_ms   INTEGER,
    p95_latency_ms   INTEGER,
    UNIQUE(check_id, hour_bucket)
);

CREATE INDEX idx_check_results_hourly_check_id_hour_bucket ON check_results_hourly(check_id, hour_bucket DESC);

CREATE TABLE check_results_daily (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    check_id         INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
    day_bucket       DATETIME NOT NULL,
    total_count      INTEGER NOT NULL,
    up_count         INTEGER NOT NULL,
    degraded_count   INTEGER NOT NULL,
    down_count       INTEGER NOT NULL,
    unknown_count    INTEGER NOT NULL,
    avg_latency_ms   REAL,
    min_latency_ms   INTEGER,
    max_latency_ms   INTEGER,
    p95_latency_ms   INTEGER,
    UNIQUE(check_id, day_bucket)
);

CREATE INDEX idx_check_results_daily_check_id_day_bucket ON check_results_daily(check_id, day_bucket DESC);

CREATE TABLE retention_settings (
    id                  INTEGER PRIMARY KEY CHECK(id = 1),
    raw_days            INTEGER NOT NULL DEFAULT 7,
    hourly_days         INTEGER NOT NULL DEFAULT 30,
    daily_days          INTEGER NOT NULL DEFAULT 180,
    keep_daily_forever  BOOLEAN NOT NULL DEFAULT 0,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO retention_settings (id) VALUES (1);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS check_results_daily;
DROP TABLE IF EXISTS check_results_hourly;
DROP TABLE IF EXISTS check_results;
DROP TABLE IF EXISTS checks;
DROP TABLE IF EXISTS components;
DROP TABLE IF EXISTS retention_settings;
-- +goose StatementEnd
