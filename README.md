APP_PORT=8081

DB_HOST=192.168.1.10
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
# Optional. Set this only when the bucket is public or you use a custom public domain.
R2_PUBLIC_BASE_URL=
