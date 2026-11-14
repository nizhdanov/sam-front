-- +goose Up
CREATE TABLE imports (
    id UUID PRIMARY KEY,
    filename TEXT NOT NULL,
    storage_path TEXT,
    source VARCHAR(30) NOT NULL CHECK (source IN ('CSV', 'AGENT')),
    status VARCHAR(30) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    total_rows INTEGER NOT NULL DEFAULT 0,
    total_devices INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_imports_created ON imports(created_at DESC);
CREATE INDEX idx_imports_status ON imports(status);

CREATE TABLE inventory_snapshots (
    id UUID PRIMARY KEY,
    import_id UUID NOT NULL REFERENCES imports(id),
    serial_number TEXT NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL,
    snapshot_type VARCHAR(20) NOT NULL DEFAULT 'FULL',
    software JSONB NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    attempt_token UUID,
    lease_until TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    processing_started_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    UNIQUE (import_id, serial_number),
    CHECK (jsonb_typeof(software) = 'array')
);

CREATE INDEX idx_snapshots_import_status ON inventory_snapshots(import_id, status);
CREATE INDEX idx_snapshots_serial ON inventory_snapshots(serial_number);

CREATE TABLE import_errors (
    id UUID PRIMARY KEY,
    import_id UUID NOT NULL REFERENCES imports(id),
    snapshot_id UUID REFERENCES inventory_snapshots(id),
    row_number INTEGER,
    error_code TEXT NOT NULL,
    error_message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_import_errors_import ON import_errors(import_id);

-- +goose Down
DROP TABLE import_errors;
DROP TABLE inventory_snapshots;
DROP TABLE imports;
