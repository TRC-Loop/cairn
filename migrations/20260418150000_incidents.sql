-- +goose Up
-- +goose StatementBegin
CREATE TABLE incidents (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    title                 TEXT NOT NULL,
    status                TEXT NOT NULL CHECK(status IN ('investigating','identified','monitoring','resolved')) DEFAULT 'investigating',
    severity              TEXT NOT NULL CHECK(severity IN ('minor','major','critical')) DEFAULT 'minor',
    started_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at           DATETIME,
    auto_created          BOOLEAN NOT NULL DEFAULT 0,
    triggering_check_id   INTEGER REFERENCES checks(id) ON DELETE SET NULL,
    created_by_user_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_incidents_status     ON incidents(status);
CREATE INDEX idx_incidents_started_at ON incidents(started_at DESC);

CREATE TABLE incident_updates (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id         INTEGER NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    status              TEXT NOT NULL CHECK(status IN ('investigating','identified','monitoring','resolved')),
    message             TEXT NOT NULL,
    posted_by_user_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    auto_generated      BOOLEAN NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_incident_updates_incident_created ON incident_updates(incident_id, created_at ASC);

CREATE TABLE incident_affected_checks (
    incident_id INTEGER NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    check_id    INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
    PRIMARY KEY (incident_id, check_id)
);

CREATE INDEX idx_incident_affected_checks_check_id ON incident_affected_checks(check_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_incident_affected_checks_check_id;
DROP TABLE IF EXISTS incident_affected_checks;
DROP INDEX IF EXISTS idx_incident_updates_incident_created;
DROP TABLE IF EXISTS incident_updates;
DROP INDEX IF EXISTS idx_incidents_started_at;
DROP INDEX IF EXISTS idx_incidents_status;
DROP TABLE IF EXISTS incidents;
-- +goose StatementEnd
