-- +goose Up
ALTER TABLE software_products ADD COLUMN policy_comment TEXT;
ALTER TABLE compliance_findings ADD COLUMN acknowledge_comment TEXT, ADD COLUMN acknowledged_at TIMESTAMPTZ;
CREATE UNIQUE INDEX uq_products_identity ON software_products (normalized_name, normalized_publisher, LOWER(COALESCE(edition, '')))
    WHERE archived_at IS NULL;

-- +goose Down
DROP INDEX uq_products_identity;
ALTER TABLE compliance_findings DROP COLUMN acknowledge_comment, DROP COLUMN acknowledged_at;
ALTER TABLE software_products DROP COLUMN policy_comment;
