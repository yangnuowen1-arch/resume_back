-- Move legacy runtime schema initialization into an explicit migration.
-- Run this migration before starting a backend version that relies on these columns.

BEGIN;

SET LOCAL lock_timeout = '5s';

-- jobs.dynamic_fields supersedes the former custom_fields column.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'jobs'
          AND column_name = 'custom_fields'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'jobs'
          AND column_name = 'dynamic_fields'
    ) THEN
        ALTER TABLE jobs RENAME COLUMN custom_fields TO dynamic_fields;
    END IF;
END $$;

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS dynamic_fields JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE candidates
    ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'new',
    ADD COLUMN IF NOT EXISTS current_position_category VARCHAR(100),
    ADD COLUMN IF NOT EXISTS position_category_id BIGINT,
    ADD COLUMN IF NOT EXISTS current_job_id BIGINT;

ALTER TABLE resumes
    ADD COLUMN IF NOT EXISTS file_key TEXT,
    ADD COLUMN IF NOT EXISTS parse_status VARCHAR(30) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS parse_error TEXT,
    ADD COLUMN IF NOT EXISTS parsed_at TIMESTAMP WITHOUT TIME ZONE;

-- Preserve the existing file-key and parse-status normalization behavior as a
-- one-time data migration instead of re-running it on every application start.
UPDATE resumes
SET file_key = regexp_replace(file_url, '^r2://[^/]+/', '')
WHERE (file_key IS NULL OR file_key = '')
  AND file_url LIKE 'r2://%/%';

UPDATE resumes
SET file_key = regexp_replace(file_url, '^/uploads/', '')
WHERE (file_key IS NULL OR file_key = '')
  AND file_url LIKE '/uploads/%';

UPDATE resumes
SET file_key = regexp_replace(file_url, '^https?://[^/]+/(.*)$', '\1')
WHERE (file_key IS NULL OR file_key = '')
  AND file_url ~ '^https?://[^/]+/resumes/';

UPDATE resumes
SET parse_status = 'parsed',
    parsed_at = COALESCE(parsed_at, updated_at)
WHERE parse_status = 'pending'
  AND raw_text IS NOT NULL
  AND BTRIM(raw_text) <> '';

ALTER TABLE screening_results
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMP WITHOUT TIME ZONE,
    ADD COLUMN IF NOT EXISTS finished_at TIMESTAMP WITHOUT TIME ZONE,
    ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMP WITHOUT TIME ZONE,
    ADD COLUMN IF NOT EXISTS requirements JSONB;

COMMIT;
