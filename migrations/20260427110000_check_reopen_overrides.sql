-- +goose Up
-- +goose StatementBegin
ALTER TABLE checks ADD COLUMN reopen_window_seconds INTEGER;
ALTER TABLE checks ADD COLUMN reopen_mode TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE checks DROP COLUMN reopen_window_seconds;
ALTER TABLE checks DROP COLUMN reopen_mode;
-- +goose StatementEnd
