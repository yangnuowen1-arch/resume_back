# AI Screening Async API Change

This document describes the frontend changes required after the resume screening flow was changed from a synchronous Dify call to an asynchronous queued job.

## Summary

Before:

```text
POST /screening-tasks/run
-> backend calls Dify synchronously
-> frontend receives final AI result in the same response
```

Now:

```text
POST /screening-tasks/run
-> backend creates a queued screening task
-> frontend receives screeningResultId immediately
-> background worker processes the task
-> frontend polls task status from GET /screening-tasks
```

The frontend should no longer expect `score`, `summary`, `matchLevel`, or `markdownReport` to be available immediately after calling `POST /screening-tasks/run`.

## Status Flow

Screening task status now follows this flow:

```text
queued -> running -> success
                  -> failed
```

Status meanings:

| Status | UI Text | Meaning |
| --- | --- | --- |
| `queued` | 排队中 | The task has been created and is waiting for the worker. |
| `running` | 筛选中 | The background worker is processing the task. |
| `success` | 已完成 | The AI screening completed successfully. |
| `failed` | 失败 | The AI screening failed. Show `errorMessage` when available. |

Terminal statuses:

```text
success, failed
```

The frontend should stop polling after the task reaches a terminal status.

## Run Screening

Create a screening task and enqueue it for background processing.

```http
POST /api/v1/screening-tasks/run
Authorization: Bearer <token>
Content-Type: application/json
```

Request body:

```json
{
  "resumeId": 1,
  "jobId": 2,
  "outputLanguage": "Chinese"
}
```

Fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `resumeId` | number | Yes | Resume ID. |
| `jobId` | number | Yes | Job ID. |
| `outputLanguage` | string | No | Output language for the AI result. Defaults to `Chinese`. |

Successful response:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "screeningResultId": 123,
    "applicationId": 456,
    "resumeId": 1,
    "jobId": 2,
    "status": "queued"
  }
}
```

Important frontend change:

- Treat this response as "task accepted", not "screening completed".
- Save `screeningResultId`.
- Show `queued` state immediately.
- Start polling task status.

## List / Poll Screening Tasks

Use this endpoint to refresh task status and table data.

```http
GET /api/v1/screening-tasks?page=1&pageSize=20&status=all
Authorization: Bearer <token>
```

Optional query params:

| Param | Type | Description |
| --- | --- | --- |
| `page` | number | Page number. Defaults to `1`. |
| `pageSize` | number | Page size. Defaults to `20`. Max `200`. |
| `keyword` | string | Candidate/job keyword. |
| `status` | string | `queued`, `running`, `success`, `failed`, or `all`. |
| `jobId` | number | Filter by job ID. |
| `candidateId` | number | Filter by candidate ID. |

Response item shape:

```json
{
  "id": 123,
  "applicationId": 456,
  "candidateId": 789,
  "candidate": "Alice",
  "candidateName": "Alice",
  "jobId": 2,
  "jobTitle": "Backend Engineer",
  "position": "Backend Engineer",
  "aiScore": 88.5,
  "status": "success",
  "date": "2026-06-17T10:00:00Z",
  "createdAt": "2026-06-17T10:00:00Z",
  "createdBy": 1,
  "matchLevel": "strong",
  "recommendation": "recommend_interview",
  "errorMessage": null
}
```

For `queued` and `running`, result fields may be empty:

```json
{
  "id": 123,
  "aiScore": null,
  "status": "running",
  "matchLevel": null,
  "recommendation": null,
  "errorMessage": null
}
```

For `failed`, show `errorMessage`:

```json
{
  "id": 123,
  "aiScore": null,
  "status": "failed",
  "errorMessage": "Dify 简历筛选失败: ..."
}
```

## Batch Analyze Candidates

Batch analyze now also creates queued screening tasks and sends them to the same background worker pool.

```http
POST /api/v1/candidates/batch-analyze
Authorization: Bearer <token>
Content-Type: application/json
```

Request body:

```json
{
  "candidateIds": [1, 2, 3],
  "jobId": 10
}
```

Successful response item shape:

```json
{
  "candidateId": 1,
  "resumeId": 11,
  "applicationId": 12,
  "screeningResultId": 13,
  "jobId": 10,
  "parseStatus": "parsed",
  "status": "queued"
}
```

Frontend behavior:

- Treat each `status = queued` item as an accepted async task.
- Use `screeningResultId` to update or track each row.
- Poll `GET /screening-tasks` the same way as single screening.
- Some candidates may return `failed` immediately if they have no valid resume/application.

## Recommended Frontend Polling Flow

After `POST /screening-tasks/run` succeeds:

1. Add or update the table row with `status = queued`.
2. Start polling every 2 seconds.
3. Call `GET /screening-tasks` with the current table filters, or at least enough filters to include the new task.
4. Find the item whose `id === screeningResultId`.
5. Update row UI based on the latest `status`.
6. Stop polling when status is `success` or `failed`.

Pseudo code:

```ts
const terminalStatuses = new Set(['success', 'failed'])

async function runScreening(resumeId: number, jobId: number) {
  const response = await api.post('/screening-tasks/run', {
    resumeId,
    jobId,
    outputLanguage: 'Chinese',
  })

  const task = response.data
  upsertTaskRow(task.screeningResultId, {
    status: task.status,
    resumeId: task.resumeId,
    jobId: task.jobId,
    applicationId: task.applicationId,
  })

  pollScreeningTask(task.screeningResultId)
}

function pollScreeningTask(screeningResultId: number) {
  const timer = window.setInterval(async () => {
    const response = await api.get('/screening-tasks', {
      params: {
        page: 1,
        pageSize: 20,
        status: 'all',
      },
    })

    const item = response.data.items.find(
      (task: ScreeningTaskItem) => task.id === screeningResultId,
    )

    if (!item) return

    upsertTaskRow(item.id, item)

    if (terminalStatuses.has(item.status)) {
      window.clearInterval(timer)
    }
  }, 2000)
}
```

## UI Changes

Button behavior:

- After clicking "Run Screening", disable the button for that row while status is `queued` or `running`.
- Do not show a success toast like "筛选完成" immediately after `POST /run`.
- Instead show "任务已提交" or "已加入筛选队列".

Table state:

| Status | Row UI |
| --- | --- |
| `queued` | Show "排队中"; score/result fields empty. |
| `running` | Show "筛选中"; optionally show spinner. |
| `success` | Show score, match level, recommendation. |
| `failed` | Show failure state and `errorMessage`. |

Polling:

- Poll every 2 seconds.
- Stop polling after `success` or `failed`.
- Also stop polling when the user leaves the page or closes the modal.

## TypeScript Types

```ts
type ScreeningTaskStatus = 'queued' | 'running' | 'success' | 'failed'

interface RunScreeningRequest {
  resumeId: number
  jobId: number
  outputLanguage?: string
}

interface RunScreeningResponse {
  screeningResultId: number
  applicationId: number
  resumeId: number
  jobId: number
  status: ScreeningTaskStatus
}

interface ScreeningTaskItem {
  id: number
  applicationId: number
  candidateId?: number | null
  candidate?: string | null
  candidateName?: string | null
  jobId: number
  jobTitle: string
  position: string
  aiScore?: number | null
  status: ScreeningTaskStatus
  date: string
  createdAt: string
  createdBy?: number | null
  matchLevel?: string | null
  recommendation?: string | null
  errorMessage?: string | null
}
```

## Current Limitation

The current backend has list polling through:

```http
GET /api/v1/screening-tasks
```

There is not yet a dedicated detail endpoint like:

```http
GET /api/v1/screening-tasks/:id
```

If the frontend needs to poll a single task more efficiently or display full detail fields such as `summary` and `markdownReport`, the backend should add a detail endpoint in the next step.
