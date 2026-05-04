-- +goose Up
-- +goose StatementBegin
ALTER TABLE checks ADD COLUMN push_token TEXT;
CREATE UNIQUE INDEX idx_checks_push_token ON checks(push_token) WHERE push_token IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_checks_push_token;
ALTER TABLE checks DROP COLUMN push_token;
-- +goose StatementEnd
