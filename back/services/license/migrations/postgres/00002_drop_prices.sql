-- +goose Up
ALTER TABLE license_entitlements DROP COLUMN unit_price, DROP COLUMN currency;

-- +goose Down
ALTER TABLE license_entitlements ADD COLUMN unit_price NUMERIC(14, 2), ADD COLUMN currency VARCHAR(3);
