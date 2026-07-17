# 邮箱收件箱自动化 - 实施计划

> 目标：连接 Gmail，自动扫描未读邮件，提取 PDF/DOCX 简历附件，
> 将每个合格附件作为独立导入单元：按附件文件名创建候选人壳、上传原文件，
> 再持久化 candidate、resume、邮件元数据及它们的关联；同一邮件附件重试时不重复创建。

## 已锁定的决策（开工前的共识）

| 维度 | 决策 |
|------|------|
| 连接方式 | Gmail **OAuth**（`golang.org/x/oauth2`） |
| 触发方式 | **OAuth 授权成功后立即扫描 + 手动按钮 + 每晚 23:00 定时**，共用同一套 `ScanAndImport` 核心 |
| 定时实现 | **`time.Ticker`** 后台 goroutine，零依赖 |
| 手动响应 | **异步**：接口返回 `taskId`，前端轮询状态（复用现有 screening 任务模式） |
| 附件过滤 | 默认只收 **`.pdf` / `.docx`**，其余忽略 |
| 导入粒度 | **每个合格附件一个候选人壳**，不按发件人 email 合并候选人 |
| 幂等 | 以 **邮箱账号 + provider message ID + attachment key** 唯一（缺少 provider attachment ID 时以附件序号 + hash 兜底）；不以全局文件 hash 去重，因此相同文件来自不同邮件也保留各自来源 |
| 解析 | 邮件附件**先只存文件不解析**，后续走 `/resumes/:id/parse` |
| 候选人 name | **附件文件名去扩展名**；这是解析前的暂定显示名，可由后续人工或专门的解析补全流程校正 |
| R2 对象 key | 对 `accountID + messageID + attachmentKey` 求摘要，使用 `resumes/mailbox/<sha256>.<ext>`；保存 `file_key` 和 `resume.file_url` |
| 邮件留痕 | 持久化邮件元数据，并以附件记录关联其 `candidate_id`、`resume_id` 和 R2 key，方便追溯来源与重试 |
| 建表方式 | **手写 SQL** → 执行 → 跑 gorm gen 生成 model |
| 并发保护 | 同一邮箱账号同时只跑一个扫描；数据库的邮件、附件唯一约束兜底跨 worker/进程的幂等 |

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
   │ 2. 拉未读邮件，按 message ID 查询/预留幂等记录 │
   │ 3. 过滤附件: 只留 .pdf/.docx                  │
   │ 4. 以 message + attachment key 领取导入单元    │
   │    （已完成的附件跳过；失败/未完成的可重试）     │
   │ 5. 创建候选人壳：name = 附件文件名(去扩展)      │
   │ 6. 用确定性 key 上传原文件到 R2                │
   │ 7. 事务写 candidate + resume + 邮件/附件关联   │
   │    （回填 candidate_id、resume_id、R2 key）    │
   │ 8. 标记附件完成；该邮件所有附件完成后标为已读    │
   └────────────────────────────────────────────┘
```

涉及的现有可复用资产：
- 文件存储：`internal/storage/uploader.go`（`Uploader` 接口，R2/本地）
- 候选人、简历仓储及数据库事务：用于将 candidate、resume 和邮件附件关联原子回填
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
  # 候选人列表中的 resumeFileUrl 需要可直接打开时，配置公开 R2 域名。
  R2_PUBLIC_BASE_URL=https://resume.example.com
  ```

> ⚠️ 风险提醒：Google 同意屏幕未发布前仅「测试用户」可授权，readonly scope 可能触发应用审核。
> 若只想先验证链路可跑通，IMAP + 应用专用密码可在半天内做出 demo，OAuth 留到产品化阶段。

---

## Phase 1 — 数据库与模型

- [ ] 新建 `migrations/` 目录存放手写 SQL（便于追溯）
- [ ] 编写并执行迁移 SQL：
  - [ ] 新建表 `mailbox_accounts`：`provider, email, access_token, refresh_token, token_expiry, last_scan_at, created_at, updated_at`
  - [ ] 新建/扩展 `mailbox_messages`：`id, account_id, message_id, from_email, from_name, subject, import_status, processed_at`；`(account_id, message_id)` 唯一，用于保存邮件元数据与处理状态
  - [ ] 新建 `mailbox_message_attachments`：`message_id, attachment_key, attachment_index, filename, content_type, file_hash, object_key, candidate_id, resume_id`；`(message_id, attachment_key)` 唯一，用于附件级幂等和来源关联
  - [ ] `resumes.file_key`/`file_url` 保存同一确定性 R2 对象的 key 与访问地址；不建立全局文件 hash 去重约束，也不按候选人 email 建唯一约束
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
- [x] 定义统一的 `Message` / `Attachment` 数据结构（含 provider 的邮件/附件 ID、发件人、主题、文件名、字节流）
- [x] 实现 `GmailProvider`（Gmail API）
- [x] 单元测试：附件过滤、发件人解析（可用 mock，不依赖真实邮箱）

---

## Phase 4 — 扫描导入核心（`internal/service/mailbox_service.go`）

- [x] `ScanAndImport(ctx, accountID)`：实现架构图中 8 步流程
- [x] 以每个附件为单位创建候选人壳：`name` 取附件文件名去扩展名，候选人来源为 email；不合并同发件人或同内容文件
- [x] 幂等逻辑：先按 message ID 查询完成状态，再以 `(message_id, attachment_key)` 持久化附件；已完成附件跳过，未完成附件安全重试
- [x] 原文件上传：使用 `resumes/mailbox/<sha256>.<ext>` 作为确定性 R2 key；上传后的 key 和访问地址分别写入附件记录及 resume 元数据
- [x] 持久化：R2 上传成功后，在一个事务中写入 candidate、resume，并补齐邮件/附件元数据及关联、回填 `candidate_id`、`resume_id`；上传后崩溃可用相同 key 重试
- [x] 附件过滤：按 `MAILBOX_ALLOWED_EXT` 白名单
- [x] 邮件状态：仅在该邮件的所有合格附件都完成后标记已读；邮件/附件唯一约束与账号扫描锁共同保证并发安全
- [x] 候选人列表回填：显示文件名派生的候选人名与其 `resume.file_url`，使原始简历可直接访问
- [x] 单元测试：同一邮件附件重试幂等、同发件人多附件各建候选人、相同内容不同邮件保留、确定性 key、关联回填和过滤

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
- [ ] 验证附件幂等：重复扫描/重试同一邮件附件不重复建候选人；同发件人不同附件分别建候选人；相同内容来自不同邮件仍保留各自记录
- [ ] 验证 R2：同一附件重试使用同一确定性 key，候选人列表显示文件名派生的 name 与可访问的简历 URL
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
- [ ] 扫描范围：只扫「未读」还是「未处理」（邮件及附件级幂等记录已能兜底防重）
- [ ] 单封邮件多附件 / 附件大小下限（过滤签名档图片）的具体阈值
