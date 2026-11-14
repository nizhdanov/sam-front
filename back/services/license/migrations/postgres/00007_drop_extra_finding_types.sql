-- +goose Up
DELETE FROM compliance_findings
WHERE finding_type NOT IN ('FORBIDDEN_SOFTWARE', 'OUTDATED_SOFTWARE', 'LICENSE_DEFICIT');

-- +goose Down
SELECT 1;
