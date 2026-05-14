-- +goose Up
-- +goose StatementBegin
CREATE TABLE status_page_domains (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    status_page_id  INTEGER NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    domain          TEXT NOT NULL UNIQUE,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_status_page_domains_page ON status_page_domains(status_page_id);

CREATE TABLE status_page_monitors (
    status_page_id  INTEGER NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    check_id        INTEGER NOT NULL REFERENCES checks(id) ON DELETE CASCADE,
    display_order   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (status_page_id, check_id)
);

CREATE INDEX idx_status_page_monitors_page ON status_page_monitors(status_page_id, display_order);

ALTER TABLE status_page_components ADD COLUMN show_monitors_default TEXT NOT NULL DEFAULT 'off' CHECK(show_monitors_default IN ('off','default_open','default_closed'));

ALTER TABLE status_pages ADD COLUMN hide_powered_by BOOLEAN NOT NULL DEFAULT 0;

ALTER TABLE status_pages ADD COLUMN show_history BOOLEAN NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE status_pages DROP COLUMN show_history;
ALTER TABLE status_pages DROP COLUMN hide_powered_by;
ALTER TABLE status_page_components DROP COLUMN show_monitors_default;
DROP INDEX IF EXISTS idx_status_page_monitors_page;
DROP TABLE IF EXISTS status_page_monitors;
DROP INDEX IF EXISTS idx_status_page_domains_page;
DROP TABLE IF EXISTS status_page_domains;
-- +goose StatementEnd
