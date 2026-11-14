-- +goose Up
CREATE TABLE catalog_suggestions (
    id UUID PRIMARY KEY,
    group_key TEXT NOT NULL,
    canonical_name TEXT NOT NULL,
    publisher TEXT,
    is_software BOOLEAN NOT NULL DEFAULT TRUE,
    items JSONB NOT NULL DEFAULT '[]',
    model TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    product_id UUID REFERENCES software_products(id),
    reviewed_by TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX uq_catalog_suggestions_pending ON catalog_suggestions (group_key) WHERE status = 'PENDING';
CREATE INDEX idx_catalog_suggestions_status ON catalog_suggestions (status, created_at);

-- +goose Down
DROP TABLE catalog_suggestions;
