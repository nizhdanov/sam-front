-- +goose Up
ALTER TABLE imports
    ADD COLUMN loaded_rows INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN duplicate_rows INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN excluded_rows INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN error_rows INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE imports DROP COLUMN loaded_rows, DROP COLUMN duplicate_rows, DROP COLUMN excluded_rows, DROP COLUMN error_rows;
