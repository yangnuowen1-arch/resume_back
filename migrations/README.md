# Database migrations

The backend only opens database connections. It never applies DDL or schema/data migrations at startup.

Apply every SQL file in this directory in lexical order before deploying a backend version that depends on it. The existing migrations are written to be safe to re-run, but production deployments should record applied versions through the deployment system or a migration tool.

Migration `0004_mailbox_attachment_persistence.sql` uses a five-second lock timeout. Apply it before or together with the backend that uses attachment-level mailbox persistence; if the migration times out, retry it in a quieter deployment window rather than allowing an unbounded database lock. Its `import_status` default remains `processed`, so records written by an older mailbox worker are recognized as complete by the new worker. Still drain any active mailbox scan jobs before running old and new importer versions concurrently: the older version does not create attachment-level links and therefore cannot participate in the new per-attachment idempotency protocol.

For a manual PostgreSQL deployment, run each file explicitly with `psql` using the target environment's connection settings, for example:

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0001_email_inbox_automation.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0002_screening_result_candidate_name.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0003_runtime_schema_columns.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0004_mailbox_attachment_persistence.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0005_resume_file_type_mime.sql
```
