-- 邮箱收件箱自动化 - Phase 1 数据库迁移
-- 目标数据库: PostgreSQL
-- 执行方式: psql 连接后整体执行；执行完成后重跑 gorm gen 生成 model。
-- 说明: 全部语句使用 IF NOT EXISTS / 部分索引，可重复执行。

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. resumes.file_hash —— 按文件 SHA-256 去重（命中则跳过）
--    部分唯一索引：仅对非空 hash 生效，避免历史 NULL 互相冲突。
-- ---------------------------------------------------------------------------
ALTER TABLE resumes
    ADD COLUMN IF NOT EXISTS file_hash char(64);

CREATE UNIQUE INDEX IF NOT EXISTS uq_resumes_file_hash
    ON resumes (file_hash)
    WHERE file_hash IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 2. candidates.email 唯一索引 —— 按发件人邮箱合并候选人
--    lower(email) 忽略大小写；部分索引避免历史 NULL 冲突。
--    注意：若现有数据存在重复 email，此索引会创建失败，需先清洗数据。
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX IF NOT EXISTS uq_candidates_email_lower
    ON candidates (lower(email))
    WHERE email IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 3. mailbox_accounts —— 已连接的 OAuth 邮箱账号
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mailbox_accounts (
    id            bigserial    PRIMARY KEY,
    provider      varchar(20)  NOT NULL,                 -- google / microsoft
    email         varchar(255) NOT NULL,
    access_token  text         NOT NULL,
    refresh_token text,
    token_expiry  timestamp,
    last_scan_at  timestamp,
    created_at    timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    timestamp    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 同一 provider 下同一邮箱只允许绑定一次
CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_accounts_provider_email
    ON mailbox_accounts (provider, lower(email));

-- ---------------------------------------------------------------------------
-- 4. mailbox_messages —— 已处理邮件记录，防止重复扫描入库
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mailbox_messages (
    id           bigserial PRIMARY KEY,
    account_id   bigint    NOT NULL REFERENCES mailbox_accounts (id) ON DELETE CASCADE,
    message_id   varchar(998) NOT NULL,                  -- 邮件 Message-ID / 平台消息 ID
    processed_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 同一账号下同一封邮件只记录一次（并发安全的去重底线）
CREATE UNIQUE INDEX IF NOT EXISTS uq_mailbox_messages_account_message
    ON mailbox_messages (account_id, message_id);

-- ---------------------------------------------------------------------------
-- 5. mailbox_scan_tasks —— 异步扫描任务状态（支撑手动触发 + 状态轮询）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mailbox_scan_tasks (
    id             bigserial   PRIMARY KEY,
    account_id     bigint      NOT NULL REFERENCES mailbox_accounts (id) ON DELETE CASCADE,
    trigger_source varchar(20) NOT NULL,                 -- manual / scheduled
    status         varchar(20) NOT NULL DEFAULT 'pending', -- pending/running/done/failed
    scanned        integer     NOT NULL DEFAULT 0,       -- 扫描到的邮件数
    imported       integer     NOT NULL DEFAULT 0,       -- 新建简历数
    skipped        integer     NOT NULL DEFAULT 0,       -- 去重跳过数
    error          text,
    started_at     timestamp,
    finished_at    timestamp,
    created_at     timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     timestamp   NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 按账号 + 状态查询近期任务（前端轮询 / 防并发判断「是否有进行中任务」）
CREATE INDEX IF NOT EXISTS idx_mailbox_scan_tasks_account_status
    ON mailbox_scan_tasks (account_id, status);

COMMIT;
