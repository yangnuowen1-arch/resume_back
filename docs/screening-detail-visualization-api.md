# 简历筛选可视化 - 详情接口需求

本文档描述前端「简历筛选可视化详情页」所需的后端改动。目标：解析筛选完成后，提供一个左右分栏的可视化界面 —— 左侧候选人简历原文，右侧逐条岗位要求比对，并且简历中命中要求的部分被高亮圈画。

## 1. 需要新增的接口

```http
GET /api/v1/screening-tasks/:id
Authorization: Bearer <token>
```

目前只有列表接口 `GET /screening-tasks`，没有详情接口（见 `frontend-screening-async-api.md` 最后一节 "There is not yet a dedicated detail endpoint"）。前端详情页调用这个新接口。

返回沿用统一 envelope（`{ code, message, data }`），`data` 结构见下。

## 2. data 字段结构

```jsonc
{
  // —— 基础信息（列表接口已有，照搬即可）——
  "id": 123,
  "candidateName": "张伟",
  "position": "高级前端工程师",
  "aiScore": 88,
  "matchLevel": "strong",
  "recommendation": "recommend_interview",
  "summary": "AI 总评文本……",

  // —— 新增字段 1：简历原文 ——
  "resumeText": "简历完整纯文本……",   // 即 resumes 表的 rawText

  // —— 新增字段 2：结构化要求比对 ——
  "requirements": [
    {
      "id": "r1",                          // 任意唯一标识，前端用它做联动
      "label": "5年以上前端开发经验",        // 岗位的某一条要求
      "status": "pass",                    // 只能是 pass / partial / miss
      "comment": "经验较浅",                // 可选，命中说明，可为 null
      "evidence": [                        // 命中的原文证据，可为空数组
        {
          "text": "拥有 6 年以上前端开发经验",  // 必须是 resumeText 的精确子串
          "start": 120,                       // 在 resumeText 中的字符起始下标（可选）
          "end": 135                          // 结束下标（可选）
        }
      ]
    }
  ]
}
```

## 3. 字段约束（决定高亮能否生效，最关键）

| 字段 | 要求 |
| --- | --- |
| `resumeText` | 简历纯文本。前端高亮就是在这段文本上切片，所以它必须和 evidence 里的片段来自同一份文本。 |
| `requirements[].status` | 严格三选一：`pass` / `partial` / `miss`，对应前端绿 / 黄 / 红。 |
| `evidence[].text` | **必须是 `resumeText` 里逐字一致的子串**（含标点、空格）。差一个字，前端就定位不到、无法高亮。 |
| `evidence[].start` / `end` | 字符下标，**强烈建议提供**。给了能精确定位，并能正确处理同一片段在简历中出现多次的情况；不给则前端只能用 `text` 做首次匹配，遇到重复片段会标错位置。 |

## 4. 数据怎么产生

`requirements` 本质上需要 AI（Dify）输出。建议改动点：

1. 调整 Dify 的 prompt，让它在筛选时额外输出一段 JSON，包含每条岗位要求的 `label / status / comment / evidence`，其中 `evidence.text` 直接从简历原文中**原样摘抄片段**（保证是子串）。
2. 后端拿到 Dify 的 JSON 后：
   - 落库（建议在 screening_result 表加一个 `requirements` JSON 字段）；
   - 详情接口里把 `resumeText`（从 resume 表取）和 `requirements` 一起返回。
3. `start` / `end` 偏移：可以让 Dify 给，但 AI 算下标常出错。**更稳的做法是后端拿到 `evidence.text` 后，自己用 `resumeText.indexOf(text)` 算出 `start` / `end` 再返回**，这样偏移一定准确。

## 5. 降级兼容与上线节奏

前端已做兜底：如果 `requirements` 为空或缺少 `resumeText`，会自动退回展示 `summary` / `markdownReport`，不会白屏。因此后端可以分两期上线：

- **一期**：先把详情接口和 `resumeText` 出了，`requirements` 先返回空数组 → 前端走降级态，详情页可用。
- **二期**：Dify 输出结构化 evidence 后补上 `requirements` → 前端自动切换为左右分栏高亮的完整效果。

## 6. TypeScript 类型（前端已按此实现）

```ts
type RequirementMatchStatus = "pass" | "partial" | "miss";

interface RequirementEvidence {
  text: string;        // 必须是 resumeText 的精确子串
  start?: number | null;
  end?: number | null;
}

interface ScreeningRequirement {
  id?: string | number;
  label: string;
  status: RequirementMatchStatus;
  comment?: string | null;
  evidence?: RequirementEvidence[];
}

interface ScreeningTaskDetail {
  id: number;
  candidateName?: string | null;
  position?: string | null;
  aiScore?: number | null;
  matchLevel?: string | null;
  recommendation?: string | null;
  summary?: string | null;
  resumeText?: string | null;      // 新增
  requirements?: ScreeningRequirement[];  // 新增
}
```
