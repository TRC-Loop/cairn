-- +goose Up
-- +goose StatementBegin
CREATE TABLE notification_channels (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    name                  TEXT NOT NULL UNIQUE,
    type                  TEXT NOT NULL CHECK(type IN ('email','discord','webhook')),
    enabled               BOOLEAN NOT NULL DEFAULT 1,
    config_json           TEXT NOT NULL DEFAULT '{}',
    retry_max             INTEGER NOT NULL DEFAULT 3 CHECK(retry_max >= 0 AND retry_max <= 10),
    retry_backoff_seconds INTEGER NOT NULL DEFAULT 1 CHECK(retry_backoff_seconds >= 1),
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notification_channels_type    ON notification_channels(type);
CREATE INDEX idx_notification_channels_enabled ON notification_channels(enabled);

CREATE TABLE check_notification_channels (
    check_id   INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
    channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    PRIMARY KEY (check_id, channel_id)
);

CREATE INDEX idx_check_notification_channels_channel ON check_notification_channels(channel_id);

CREATE TABLE notification_deliveries (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id        INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    event_type        TEXT NOT NULL CHECK(event_type IN ('incident_opened','incident_updated','incident_resolved','maintenance_started','maintenance_ended','check_recovered','test')),
    event_id          INTEGER NOT NULL,
    payload_json      TEXT NOT NULL,
    status            TEXT NOT NULL CHECK(status IN ('pending','sending','sent','failed')) DEFAULT 'pending',
    attempt_count     INTEGER NOT NULL DEFAULT 0,
    last_attempted_at DATETIME,
    last_error        TEXT,
    next_attempt_at   DATETIME,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at           DATETIME
);

CREATE INDEX idx_notification_deliveries_status_next  ON notification_deliveries(status, next_attempt_at);
CREATE INDEX idx_notification_deliveries_channel_time ON notification_deliveries(channel_id, created_at DESC);
CREATE INDEX idx_notification_deliveries_event        ON notification_deliveries(event_type, event_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_notification_deliveries_event;
DROP INDEX IF EXISTS idx_notification_deliveries_channel_time;
DROP INDEX IF EXISTS idx_notification_deliveries_status_next;
DROP TABLE IF EXISTS notification_deliveries;
DROP INDEX IF EXISTS idx_check_notification_channels_channel;
DROP TABLE IF EXISTS check_notification_channels;
DROP INDEX IF EXISTS idx_notification_channels_enabled;
DROP INDEX IF EXISTS idx_notification_channels_type;
DROP TABLE IF EXISTS notification_channels;
-- +goose StatementEnd
