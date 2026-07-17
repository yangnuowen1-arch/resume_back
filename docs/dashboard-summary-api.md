# Dashboard Summary API

```http
GET /api/v1/dashboard/summary
Authorization: Bearer <token>
```

The endpoint always reads the current database state and sends
`Cache-Control: no-store`, so refetching it after an upload, screening-status
change, or candidate-status update returns current values.

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "totalResumes": 1247,
    "pendingScreening": 38,
    "recommended": 156,
    "rejected": 892,
    "generatedAt": "2026-07-16T03:00:00Z"
  }
}
```

Metric definitions:

- `totalResumes`: all rows in `resumes`.
- `pendingScreening`: screening tasks currently `queued` or `running`.
- `recommended`: latest successful screening result for each application whose
  recommendation is `recommend_interview`; older reruns do not inflate it.
- `rejected`: candidates currently in `rejected` status.

The existing data model does not preserve full status history, so the endpoint
intentionally returns live counts rather than potentially inaccurate historical
percentage changes. Historical trend cards should be added only with metric
snapshots or status-transition events.
