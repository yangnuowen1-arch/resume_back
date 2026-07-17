APP_PORT=8081

DB_HOST=192.168.31.157
DB_PORT=5432
DB_USER=app_user
DB_PASSWORD=change_this_password
DB_NAME=app_db
DB_SSLMODE=disable

# Optional: Cloudflare R2 resume storage.
# If these are not set, uploaded resumes are saved under local uploads/.
R2_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com
R2_BUCKET=resume
R2_ACCESS_KEY_ID=your_r2_s3_access_key_id
R2_SECRET_ACCESS_KEY=your_r2_s3_secret_access_key
# Required when enabling mailbox imports, so candidate-list resumeFileUrl is browser-openable.
R2_PUBLIC_BASE_URL=

# Optional: Dify workflow integration for AI resume screening.
# DIFY_BASE_URL can be either https://api.dify.ai or https://api.dify.ai/v1.
DIFY_BASE_URL=https://api.dify.ai/v1
DIFY_API_KEY=app-nofTI11DJdwON3CUQBkaW0Ai
DIFY_USER=resume_back
DIFY_RESUME_FILE_INPUT_NAME=resume_file
DIFY_JOB_CONTEXT_INPUT_NAME=job_context
DIFY_OUTPUT_LANGUAGE_INPUT_NAME=output_language
DIFY_RESULT_OUTPUT_NAME=screening_result
DIFY_SCREENING_WORKER_COUNT=3

# Optional: after Gmail OAuth succeeds, redirect the browser back to this frontend URL.
# The callback appends mailboxConnected, accountId, email, and taskId query parameters.
# Leave empty to return the callback JSON directly.
MAILBOX_OAUTH_SUCCESS_REDIRECT_URL=http://localhost:5173/candidates
