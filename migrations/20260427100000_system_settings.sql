-- +goose Up
-- +goose StatementBegin
CREATE TABLE system_settings (
    id                              INTEGER PRIMARY KEY CHECK(id = 1),
    incident_id_format              TEXT NOT NULL DEFAULT '#INC-{id}',
    incident_reopen_window_seconds  INTEGER NOT NULL DEFAULT 3600,
    incident_reopen_mode            TEXT NOT NULL CHECK(incident_reopen_mode IN ('always','never','flapping_only')) DEFAULT 'always',
    updated_at                      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO system_settings (id) VALUES (1);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS system_settings;
-- +goose StatementEnd
