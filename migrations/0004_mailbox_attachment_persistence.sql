-- Mailbox imports are idempotent per message attachment, rather than per
-- candidate email address or global file hash. This permits the same file to
-- be attached to different messages and lets every attachment create its own
-- candidate/resume shell.

BEGIN;

-- Fail rather than indefinitely blocking mailbox/candidate writers during a
-- deployment. Retry the migration during a quieter window if this is hit.
SET LOCAL lock_timeout = '5s';

-- The former global uniqueness rules merged otherwise independent candidates
-- and attachments. Keep file_hash searchable for diagnostics/deduplication,
-- but no longer use it as an import gate.
DROP INDEX IF EXISTS uq_candidates_email_lower;
DROP INDEX IF EXISTS uq_resumes_file_hash;

CREATE INDEX IF NOT EXISTS idx_resumes_file_hash
    ON resumes (file_hash)
    WHERE file_hash IS NOT NULL;

-- Mail headers and filenames can legitimately exceed the former 100-character
-- candidate columns. Keep the candidate shell faithful to the intake metadata.
ALTER TABLE candidates
    ALTER COLUMN name TYPE VARCHAR(255),
    ALTER COLUMN email TYPE VARCHAR(255);

-- Keep processed_at compatible with the previous backend release. A separate
-- state column lets the new importer represent a partial message without
-- making an old worker's completed records look incomplete during rollout.
ALTER TABLE mailbox_messages
    ADD COLUMN IF NOT EXISTS from_email VARCHAR(255),
    ADD COLUMN IF NOT EXISTS from_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS subject TEXT,
    ADD COLUMN IF NOT EXISTS import_status VARCHAR(20) NOT NULL DEFAULT 'processed';

-- Each provider-stable attachment key is the idempotency boundary. Candidate
-- and resume references are written in the same transaction as this row.
CREATE TABLE IF NOT EXISTS mailbox_message_attachments (
    id               bigserial PRIMARY KEY,
    message_id       bigint       NOT NULL REFERENCES mailbox_messages (id) ON DELETE CASCADE,
    attachment_key   varchar(128) NOT NULL,
    attachment_index integer      NOT NULL,
    filename         varchar(255),
    content_type     varchar(255),
    file_hash        char(64),
    object_key       text,
    candidate_id     bigint       NOT NULL REFERENCES candidates (id),
    resume_id        bigint       NOT NULL REFERENCES resumes (id),
    created_at       timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_message_attachments_message_attachment
    ON mailbox_message_attachments (message_id, attachment_key);

COMMIT;
