-- +goose Up
-- +goose StatementBegin
CREATE TABLE status_pages (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    slug               TEXT NOT NULL UNIQUE,
    title              TEXT NOT NULL,
    description        TEXT,
    logo_url           TEXT,
    accent_color       TEXT,
    custom_footer_html TEXT,
    password_hash      TEXT,
    is_default         BOOLEAN NOT NULL DEFAULT 0,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_status_pages_is_default ON status_pages(is_default);
CREATE UNIQUE INDEX idx_status_pages_one_default ON status_pages(is_default) WHERE is_default = 1;

CREATE TABLE status_page_components (
    status_page_id INTEGER NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    component_id   INTEGER NOT NULL REFERENCES components(id) ON DELETE CASCADE,
    display_order  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (status_page_id, component_id)
);

CREATE INDEX idx_status_page_components_order ON status_page_components(status_page_id, display_order);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_status_page_components_order;
DROP TABLE IF EXISTS status_page_components;
DROP INDEX IF EXISTS idx_status_pages_one_default;
DROP INDEX IF EXISTS idx_status_pages_is_default;
DROP TABLE IF EXISTS status_pages;
-- +goose StatementEnd
