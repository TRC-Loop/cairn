-- +goose Up
-- +goose StatementBegin
CREATE TABLE maintenance_windows (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    title              TEXT NOT NULL,
    description        TEXT,
    starts_at          DATETIME NOT NULL,
    ends_at            DATETIME NOT NULL CHECK(ends_at > starts_at),
    state              TEXT NOT NULL CHECK(state IN ('scheduled','in_progress','completed','cancelled')) DEFAULT 'scheduled',
    created_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_maintenance_state_starts ON maintenance_windows(state, starts_at);
CREATE INDEX idx_maintenance_ends_at      ON maintenance_windows(ends_at);

CREATE TABLE maintenance_affected_components (
    maintenance_id INTEGER NOT NULL REFERENCES maintenance_windows(id) ON DELETE CASCADE,
    component_id   INTEGER NOT NULL REFERENCES components(id) ON DELETE CASCADE,
    PRIMARY KEY (maintenance_id, component_id)
);

CREATE INDEX idx_maintenance_affected_component ON maintenance_affected_components(component_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_maintenance_affected_component;
DROP TABLE IF EXISTS maintenance_affected_components;
DROP INDEX IF EXISTS idx_maintenance_ends_at;
DROP INDEX IF EXISTS idx_maintenance_state_starts;
DROP TABLE IF EXISTS maintenance_windows;
-- +goose StatementEnd
