# 邮箱收件箱自动化 - 实施计划

> 目标：连接 Gmail，自动扫描未读邮件，提取 PDF/DOCX 简历附件，
> 上传存储并自动创建候选人 + 关联简历，按 email 合并候选人、按文件 hash 跳过重复。

## 已锁定的决策（开工前的共识）

| 维度 | 决策 |
|------|------|
| 连接方式 | Gmail **OAuth**（`golang.org/x/oauth2`） |
| 触发方式 | **OAuth 授权成功后立即扫描 + 手动按钮 + 每晚 23:00 定时**，共用同一套 `ScanAndImport` 核心 |
| 定时实现 | **`time.Ticker`** 后台 goroutine，零依赖 |
| 手动响应 | **异步**：接口返回 `taskId`，前端轮询状态（复用现有 screening 任务模式） |
| 附件过滤 | 默认只收 **`.pdf` / `.docx`**，其余忽略 |
| 去重 | 按 **email 合并候选人** + 按 **file SHA-256 hash 跳过简历** |
| 解析 | 邮件附件**先只存文件不解析**，后续走 `/resumes/:id/parse` |
| 候选人 name | **文件名优先**（去扩展名），无意义则回退发件人显示名 |
| 文件 key 唯一性 | 沿用现有 **uuid**（`internal/handler/resume_handler.go:69`） |
| 建表方式 | **手写 SQL** → 执行 → 跑 gorm gen 生成 model |
| 并发保护 | 同一邮箱账号同时只跑一个扫描（状态位 + `mailbox_messages` 去重） |

## 架构总览

```
触发层
 ├─ OAuth callback            (授权成功后自动入队并回跳前端，携带 taskId)
 ├─ POST /mailbox/scan        (手动, 立即返回 taskId)
 └─ time.Ticker @23:00        (定时, 后台 goroutine)
        │
        ▼
 ScanAndImport(account)  ← 手动/定时共用核心
        │
   ┌────┴───────────────────────────────────────┐
   │ 1. OAuth 取 token (过期则 refresh)           │
   │ 2. 拉未读邮件 (跳过 mailbox_messages 已处理)  │
   │ 3. 过滤附件: 只留 .pdf/.docx                 │
   │ 4. 算 SHA-256 → file_hash 命中则跳过          │
   │ 5. 查发件人 email → 命中则挂到旧候选人         │
   │ 6. name = 文件名(去扩展) ?: 发件人显示名       │
   │ 7. 上传 R2 (uuid key) + CreateWithResume      │
   │ 8. 记 mailbox_messages, 标记邮件已读           │
   └────────────────────────────────────────────┘
```

涉及的现有可复用资产：
- 文件存储：`internal/storage/uploader.go`（`Uploader` 接口，R2/本地）
- 建候选人+简历：`internal/service/candidate_service.go:123` `CreateWithResume`
- 给已有候选人加简历：`internal/service/candidate_service.go:187` `UploadResume`
- 异步任务+worker 池范式：`internal/service/screening_task_service.go`
- 候选人来源已留位：`CandidateSourceEmail`（`candidate_service.go`）

---

## Phase 0 — 准备凭据与环境（只能你做，阻塞后续）

- [ ] **Google Cloud Console**
  - [ ] 新建项目 → 启用 **Gmail API**
  - [ ] 配置 OAuth 同意屏幕（测试阶段把自己的邮箱加为「测试用户」）
  - [ ] 创建 OAuth 客户端 ID（Web 应用类型）→ 记下 `Client ID` / `Client Secret`
  - [ ] 授权回调地址：`http://localhost:8081/api/v1/mailbox/oauth/google/callback`
  - [ ] scope：`https://www.googleapis.com/auth/gmail.modify`（读邮件 + 标记已读）
- [ ] **`.env` 追加配置**
  ```
  GOOGLE_OAUTH_CLIENT_ID=
  GOOGLE_OAUTH_CLIENT_SECRET=
  GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8081/api/v1/mailbox/oauth/google/callback
  MAILBOX_SCAN_CRON_HOUR=23
  MAILBOX_ALLOWED_EXT=.pdf,.docx
  ```

> ⚠️ 风险提醒：Google 同意屏幕未发布前仅「测试用户」可授权，readonly scope 可能触发应用审核。
> 若只想先验证链路可跑通，IMAP + 应用专用密码可在半天内做出 demo，OAuth 留到产品化阶段。

---

## Phase 1 — 数据库与模型

- [ ] 新建 `migrations/` 目录存放手写 SQL（便于追溯）
- [ ] 编写并执行迁移 SQL：
  - [ ] `resumes` 加 `file_hash char(64)` + 部分唯一索引（`WHERE file_hash IS NOT NULL`）
  - [ ] `candidates` 加 `email` 唯一索引（`lower(email)`，`WHERE email IS NOT NULL`）
  - [ ] 新建表 `mailbox_accounts`：`provider, email, access_token, refresh_token, token_expiry, last_scan_at, created_at, updated_at`
  - [ ] 新建表 `mailbox_messages`：`account_id, message_id, processed_at`（防重复扫描，`(account_id, message_id)` 唯一）
  - [ ] 新建表 `mailbox_scan_tasks`：`id, account_id, trigger_source(manual/scheduled), status(pending/running/done/failed), scanned, imported, skipped, error, started_at, finished_at`
- [ ] 跑 gorm gen，生成 `internal/dal/model/*.gen.go` + `internal/dal/query/*.gen.go`
- [ ] 校验生成的 model 字段类型正确

---

## Phase 2 — 配置与依赖

- [ ] `internal/config/config.go` 增加 OAuth / 扫描相关配置项与读取逻辑
- [ ] 增加 `MailboxEnabled()` 之类的开关（参照现有 `R2Enabled()` / `DifyEnabled()`）
- [ ] `go get` 依赖：
  - [ ] `golang.org/x/oauth2`（google endpoint）
  - [ ] `google.golang.org/api/gmail/v1`

---

## Phase 3 — 邮箱 Provider 抽象层（`internal/mailbox/`）

- [x] 定义 `Provider` 接口：`AuthURL()`, `Exchange(code)`, `RefreshToken()`, `ListUnread()`, `FetchAttachments(msgID)`, `MarkRead(msgID)`
- [x] 定义统一的 `Message` / `Attachment` 数据结构（含发件人 email、显示名、文件名、字节流）
- [x] 实现 `GmailProvider`（Gmail API）
- [x] 单元测试：附件过滤、发件人解析（可用 mock，不依赖真实邮箱）

---

## Phase 4 — 扫描导入核心（`internal/service/mailbox_service.go`）

- [x] `ScanAndImport(ctx, accountID)`：实现架构图中 8 步流程
- [x] 去重逻辑：
  - [x] 文件 SHA-256 → 查 `resumes.file_hash` 命中则跳过
  - [x] 发件人 email → `candidateRepo.FindByEmail` 命中则 `UploadResume` 挂到旧候选人；否则 `CreateWithResume`
- [x] name 取法：文件名去扩展名；无意义（如 `resume.pdf`）回退发件人显示名/邮箱前缀
- [x] 附件过滤：按 `MAILBOX_ALLOWED_EXT` 白名单
- [x] 记录 `mailbox_messages`，并发安全（同账号同时只跑一个）
- [x] 单元测试：去重命中/未命中、合并、name 回退、过滤

---

## Phase 5 — 异步任务与触发

- [x] 任务表驱动的异步扫描（参照 `screening_task_service.go` worker 池范式）
  - [x] 创建 `mailbox_scan_tasks` 记录 → 入队 → worker 执行 `ScanAndImport` → 回写状态与统计
- [x] 定时触发：`time.Ticker` goroutine，每晚 `MAILBOX_SCAN_CRON_HOUR` 点对所有账号入队扫描任务
- [x] 在 `cmd/server/main.go` 或 `router.go` 装配启动定时器

---

## Phase 6 — HTTP 接口与路由（`internal/handler/mailbox_handler.go`）

- [x] `GET  /mailbox/oauth/:provider/url`     获取授权跳转地址
- [x] `GET  /mailbox/oauth/:provider/callback` OAuth 回调，存 token 到 `mailbox_accounts`
- [x] `GET  /mailbox/accounts`                 列出已连接邮箱账号
- [x] `DELETE /mailbox/accounts/:id`           解绑账号
- [x] `POST /mailbox/scan`                     手动触发扫描，返回 `taskId`
- [x] `GET  /mailbox/scan/:taskId`             查询扫描任务状态/统计
- [x] 路由注册到 `internal/router/router.go` 的 `private` group
- [x] 补 swagger 注释

---

## Phase 7 — 联调与文档

- [ ] 用真实 Gmail 测试账号端到端跑通（连接 → 发带附件邮件 → 手动扫描 → 候选人出现）
- [ ] 验证去重：重复发同一附件 → 跳过；同发件人不同附件 → 挂到同候选人
- [ ] 验证定时：临时改 cron 小时测试自动触发
- [ ] 写前端对接文档（参照现有 `docs/frontend-screening-async-api.md` 风格）

---

## 关键依赖与阻塞关系

```
Phase 0 (凭据) ──┐
                 ├──► Phase 7 端到端联调（必须有真实凭据）
Phase 1 (DB) ───►Phase 2 ──►Phase 3 ──►Phase 4 ──►Phase 5 ──►Phase 6
```

- Phase 1~6 的编码不阻塞于 Phase 0，可先写 + 用 mock 单测。
- 只有 Phase 7 端到端联调强依赖 Phase 0 的真实 OAuth 凭据。

## 待实现时再定的小问题（不阻塞开工）

- [ ] token 是否加密存储（生产建议加密 `access_token`/`refresh_token`）
- [ ] 扫描范围：只扫「未读」还是「未处理」（`mailbox_messages` 已能兜底防重）
- [ ] 单封邮件多附件 / 附件大小下限（过滤签名档图片）的具体阈值
