-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE employees (
    id UUID PRIMARY KEY,
    external_id TEXT UNIQUE,
    full_name TEXT NOT NULL,
    department TEXT,
    position TEXT,
    email TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE devices (
    id UUID PRIMARY KEY,
    serial_number TEXT NOT NULL UNIQUE,
    hostname TEXT,
    employee_id UUID REFERENCES employees(id),
    operating_system TEXT,
    last_inventory_at TIMESTAMPTZ,
    last_snapshot_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_devices_employee ON devices(employee_id);

CREATE TABLE license_agreements (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    license_type VARCHAR(30) NOT NULL DEFAULT 'UNKNOWN'
        CHECK (license_type IN ('COMMERCIAL', 'FREEWARE', 'OPEN_SOURCE', 'TRIAL', 'PROPRIETARY', 'UNKNOWN')),
    agreement_text TEXT,
    source_url TEXT,
    commercial_allowed BOOLEAN,
    review_status VARCHAR(30) NOT NULL DEFAULT 'UNREVIEWED',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE software_products (
    id UUID PRIMARY KEY,
    canonical_name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    publisher TEXT,
    normalized_publisher TEXT NOT NULL DEFAULT '',
    edition TEXT,
    agreement_id UUID REFERENCES license_agreements(id),
    policy_status VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN'
        CHECK (policy_status IN ('ALLOWED', 'FORBIDDEN', 'RESTRICTED', 'UNKNOWN')),
    description TEXT,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_normalized ON software_products(normalized_name);
CREATE INDEX idx_products_trgm ON software_products USING gin (normalized_name gin_trgm_ops);

CREATE TABLE software_versions (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES software_products(id),
    version TEXT NOT NULL,
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN'
        CHECK (lifecycle_status IN ('CURRENT', 'OUTDATED', 'UNSUPPORTED', 'UNKNOWN')),
    release_date DATE,
    end_of_support DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, version)
);

CREATE TABLE software_aliases (
    id UUID PRIMARY KEY,
    source_key TEXT NOT NULL UNIQUE,
    raw_name TEXT NOT NULL,
    raw_publisher TEXT,
    raw_version TEXT,
    product_id UUID NOT NULL REFERENCES software_products(id),
    version_id UUID REFERENCES software_versions(id),
    match_method VARCHAR(30) NOT NULL CHECK (match_method IN ('EXACT', 'FUZZY', 'LLM', 'MANUAL')),
    reviewed_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_aliases_product ON software_aliases(product_id);

CREATE TABLE installations (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    product_id UUID REFERENCES software_products(id),
    version_id UUID REFERENCES software_versions(id),
    source_key TEXT NOT NULL,
    raw_name TEXT NOT NULL,
    raw_publisher TEXT,
    raw_version TEXT,
    match_status VARCHAR(30) NOT NULL
        CHECK (match_status IN ('MATCHED', 'UNKNOWN', 'REQUIRES_REVIEW', 'PENDING_MATCH')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    removed_at TIMESTAMPTZ,
    UNIQUE (device_id, source_key)
);

CREATE INDEX idx_installations_device ON installations(device_id);
CREATE INDEX idx_installations_product_active ON installations(product_id) WHERE is_active = TRUE;
CREATE INDEX idx_installations_source_key ON installations(source_key);

CREATE TABLE license_entitlements (
    id UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES software_products(id),
    agreement_id UUID REFERENCES license_agreements(id),
    license_metric VARCHAR(30) NOT NULL
        CHECK (license_metric IN ('PER_DEVICE', 'PER_USER', 'UNLIMITED', 'CONCURRENT', 'UNKNOWN')),
    quantity INTEGER NOT NULL CHECK (quantity >= 0),
    unit_price NUMERIC(14, 2),
    currency VARCHAR(3),
    valid_from TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    purchase_reference TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entitlements_product ON license_entitlements(product_id) WHERE is_active = TRUE;

CREATE TABLE matching_candidates (
    id UUID PRIMARY KEY,
    source_key TEXT NOT NULL UNIQUE,
    raw_name TEXT NOT NULL,
    raw_publisher TEXT,
    raw_version TEXT,
    candidate_ids JSONB NOT NULL DEFAULT '[]',
    suggested_product UUID REFERENCES software_products(id),
    match_method VARCHAR(30),
    confidence_score NUMERIC(4, 3),
    reason TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    reviewed_by TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_matching_status ON matching_candidates(status, created_at);

CREATE TABLE processed_snapshots (
    inventory_id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id),
    collected_at TIMESTAMPTZ NOT NULL,
    result_status VARCHAR(30) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE compliance_findings (
    id UUID PRIMARY KEY,
    finding_key TEXT NOT NULL UNIQUE,
    finding_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('CRITICAL', 'HIGH', 'MEDIUM', 'LOW')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('OPEN', 'ACKNOWLEDGED', 'RESOLVED')),
    device_id UUID REFERENCES devices(id),
    product_id UUID REFERENCES software_products(id),
    installation_id UUID REFERENCES installations(id),
    details JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_detected_at TIMESTAMPTZ NOT NULL,
    acknowledged_by TEXT,
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_findings_status ON compliance_findings(status, severity);
CREATE INDEX idx_findings_device ON compliance_findings(device_id);
CREATE INDEX idx_findings_product ON compliance_findings(product_id);

CREATE TABLE agreement_analyses (
    id UUID PRIMARY KEY,
    agreement_id UUID NOT NULL REFERENCES license_agreements(id),
    content_hash TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    analysis_json JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'APPROVED')),
    reviewed_by TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (agreement_id, content_hash, prompt_version)
);

-- +goose Down
DROP TABLE agreement_analyses;
DROP TABLE compliance_findings;
DROP TABLE processed_snapshots;
DROP TABLE matching_candidates;
DROP TABLE license_entitlements;
DROP TABLE installations;
DROP TABLE software_aliases;
DROP TABLE software_versions;
DROP TABLE software_products;
DROP TABLE license_agreements;
DROP TABLE devices;
DROP TABLE employees;
