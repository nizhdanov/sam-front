-- +goose Up
ALTER TABLE software_products DROP CONSTRAINT software_products_policy_status_check;
UPDATE software_products SET policy_status = CASE WHEN policy_status = 'FORBIDDEN' THEN 'FORBIDDEN' ELSE 'ACTUAL' END;
ALTER TABLE software_products RENAME COLUMN policy_status TO status;
ALTER TABLE software_products RENAME COLUMN policy_comment TO status_comment;
ALTER TABLE software_products RENAME COLUMN description TO purpose;
ALTER TABLE software_products
    ALTER COLUMN status SET DEFAULT 'ACTUAL',
    ADD CONSTRAINT software_products_status_check CHECK (status IN ('ACTUAL', 'OUTDATED', 'FORBIDDEN')),
    ADD COLUMN section TEXT,
    ADD COLUMN class TEXT,
    ADD COLUMN in_bpo BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN current_version TEXT;
CREATE INDEX idx_products_section ON software_products (section, class);

ALTER TABLE license_agreements DROP CONSTRAINT license_agreements_license_type_check;
ALTER TABLE license_agreements ADD COLUMN cost_model VARCHAR(20);
UPDATE license_agreements SET cost_model = CASE license_type
    WHEN 'OPEN_SOURCE' THEN 'FREE'
    WHEN 'FREEWARE' THEN 'FREE'
    WHEN 'TRIAL' THEN 'SHAREWARE'
    ELSE 'PAID' END;
UPDATE license_agreements SET license_type = CASE license_type
    WHEN 'OPEN_SOURCE' THEN 'OPEN_SOURCE'
    WHEN 'TRIAL' THEN 'TRIAL'
    ELSE 'COMMERCIAL' END;
ALTER TABLE license_agreements
    ALTER COLUMN license_type SET DEFAULT 'COMMERCIAL',
    ALTER COLUMN cost_model SET NOT NULL,
    ALTER COLUMN cost_model SET DEFAULT 'PAID',
    ADD CONSTRAINT license_agreements_license_type_check CHECK (license_type IN ('OPEN_SOURCE', 'COMMERCIAL', 'TRIAL')),
    ADD CONSTRAINT license_agreements_cost_model_check CHECK (cost_model IN ('FREE', 'PAID', 'SHAREWARE')),
    ADD COLUMN file_name TEXT;
CREATE INDEX idx_agreements_text_trgm ON license_agreements USING gin (agreement_text gin_trgm_ops);

ALTER TABLE devices DROP COLUMN employee_id;
DROP TABLE employees;
ALTER TABLE devices
    ADD COLUMN device_type VARCHAR(20) NOT NULL DEFAULT 'PHYSICAL' CHECK (device_type IN ('PHYSICAL', 'VIRTUAL')),
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'ACTIVE')),
    ADD COLUMN responsible TEXT,
    ADD COLUMN location TEXT;
UPDATE devices SET status = 'ACTIVE',
    device_type = CASE WHEN serial_number ~* '^(vmware|virtualbox|parallels)' THEN 'VIRTUAL' ELSE 'PHYSICAL' END;

ALTER TABLE license_entitlements DROP CONSTRAINT license_entitlements_license_metric_check;
UPDATE license_entitlements SET license_metric = 'PER_DEVICE' WHERE license_metric = 'PER_USER';
ALTER TABLE license_entitlements
    ADD CONSTRAINT license_entitlements_license_metric_check
        CHECK (license_metric IN ('PER_DEVICE', 'UNLIMITED', 'CONCURRENT', 'UNKNOWN'));

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_by TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor TEXT NOT NULL,
    entity TEXT NOT NULL,
    entity_id TEXT,
    action TEXT NOT NULL,
    summary TEXT NOT NULL,
    changes JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_audit_created ON audit_log (created_at DESC);
CREATE INDEX idx_audit_entity ON audit_log (entity, entity_id);
CREATE INDEX idx_audit_actor ON audit_log (actor);

-- +goose Down
DROP TABLE audit_log;
DROP TABLE settings;
ALTER TABLE license_entitlements DROP CONSTRAINT license_entitlements_license_metric_check;
ALTER TABLE license_entitlements
    ADD CONSTRAINT license_entitlements_license_metric_check
        CHECK (license_metric IN ('PER_DEVICE', 'PER_USER', 'UNLIMITED', 'CONCURRENT', 'UNKNOWN'));
ALTER TABLE devices DROP COLUMN device_type, DROP COLUMN status, DROP COLUMN responsible, DROP COLUMN location;
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
ALTER TABLE devices ADD COLUMN employee_id UUID REFERENCES employees(id);
DROP INDEX idx_agreements_text_trgm;
ALTER TABLE license_agreements DROP CONSTRAINT license_agreements_license_type_check;
ALTER TABLE license_agreements DROP CONSTRAINT license_agreements_cost_model_check;
ALTER TABLE license_agreements DROP COLUMN cost_model, DROP COLUMN file_name;
ALTER TABLE license_agreements
    ADD CONSTRAINT license_agreements_license_type_check
        CHECK (license_type IN ('COMMERCIAL', 'FREEWARE', 'OPEN_SOURCE', 'TRIAL', 'PROPRIETARY', 'UNKNOWN'));
DROP INDEX idx_products_section;
ALTER TABLE software_products DROP CONSTRAINT software_products_status_check;
ALTER TABLE software_products DROP COLUMN section, DROP COLUMN class, DROP COLUMN in_bpo, DROP COLUMN current_version;
ALTER TABLE software_products RENAME COLUMN purpose TO description;
ALTER TABLE software_products RENAME COLUMN status_comment TO policy_comment;
ALTER TABLE software_products RENAME COLUMN status TO policy_status;
UPDATE software_products SET policy_status = CASE WHEN policy_status = 'FORBIDDEN' THEN 'FORBIDDEN' ELSE 'ALLOWED' END;
ALTER TABLE software_products
    ALTER COLUMN policy_status SET DEFAULT 'UNKNOWN',
    ADD CONSTRAINT software_products_policy_status_check CHECK (policy_status IN ('ALLOWED', 'FORBIDDEN', 'RESTRICTED', 'UNKNOWN'));
