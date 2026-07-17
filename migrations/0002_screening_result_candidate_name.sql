-- Persist the candidate name extracted by AI screening.
-- This migration is idempotent and preserves existing nonblank values.

BEGIN;

SET LOCAL lock_timeout = '5s';

ALTER TABLE screening_results
    ADD COLUMN IF NOT EXISTS candidate_name TEXT;

WITH extracted AS (
    SELECT
        id,
        COALESCE(
            CASE
                WHEN jsonb_typeof(raw_response #> '{output,candidate_name}') = 'string'
                THEN NULLIF(BTRIM(raw_response #>> '{output,candidate_name}'), '')
            END,
            CASE
                WHEN jsonb_typeof(raw_response -> 'candidate_name') = 'string'
                THEN NULLIF(BTRIM(raw_response ->> 'candidate_name'), '')
            END
        ) AS candidate_name
    FROM screening_results
    WHERE NULLIF(BTRIM(candidate_name), '') IS NULL
)
UPDATE screening_results
SET candidate_name = extracted.candidate_name
FROM extracted
WHERE screening_results.id = extracted.id
  AND extracted.candidate_name IS NOT NULL;

COMMIT;
