-- +goose Up
-- +goose StatementBegin
CREATE TABLE status_page_footer_elements (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    status_page_id  INTEGER NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    element_type    TEXT NOT NULL CHECK(element_type IN ('link','text','separator')),
    label           TEXT,
    url             TEXT,
    open_in_new_tab BOOLEAN NOT NULL DEFAULT 1,
    display_order   INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_footer_elements_page_order ON status_page_footer_elements(status_page_id, display_order);

ALTER TABLE status_pages ADD COLUMN footer_mode TEXT NOT NULL DEFAULT 'structured' CHECK(footer_mode IN ('structured','html','both'));

UPDATE status_pages SET footer_mode = 'html' WHERE custom_footer_html IS NOT NULL AND length(trim(custom_footer_html)) > 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE status_pages DROP COLUMN footer_mode;
DROP INDEX IF EXISTS idx_footer_elements_page_order;
DROP TABLE IF EXISTS status_page_footer_elements;
-- +goose StatementEnd
