# 简历筛选详情接口（分块展示）

详情页使用一个聚合接口，而不是按卡片拆成多个请求。后端一次返回筛选任务及所有页面模块，前端可直接按 `sections` 渲染截图中的「简历摘要、候选人信息、评估结论、岗位要求对比、匹配亮点、重点关注、最终筛选建议」。

```http
GET /api/v1/screening-tasks/:id
Authorization: Bearer <token>
```

响应仍使用统一 envelope：`{ "code": 0, "message": "success", "data": { ... } }`。

## 详情数据结构

`data.sections` 是详情页的主数据源。顶层原有的 `candidateName`、`summary`、`requirements`、`markdownReport` 等字段会暂时保留，以兼容已发布的前端；新页面不应再混用它们。

```jsonc
{
  "id": 30,
  "status": "success",
  "errorMessage": null,

  // 兼容旧前端的平铺字段（新页面不作为主数据源）
  "candidateName": "杨诺雯-AI全栈或应用开发",
  "position": "Frontend",
  "aiScore": 35,
  "matchLevel": "poor",
  "recommendation": "reject",
  "summary": "候选人与岗位的核心技术栈不匹配，不建议推荐。",
  "resumeText": null,
  "requirements": [],
  "markdownReport": "# 候选人简历评估报告...",

  "sections": {
    "summary": {
      "text": "候选人与岗位的核心技术栈不匹配，不建议推荐。"
    },
    "candidateInfo": {
      "name": "杨诺雯-AI全栈或应用开发",
      "appliedPosition": "Frontend",
      "currentTitle": "前端开发工程师 / AI 全栈开发",
      "yearsOfExperience": 2,
      "highestEducation": "嘉应学院·软件工程（本科）",
      "taskStatus": "success",
      "taskErrorMessage": null
    },
    "assessment": {
      "score": 35,
      "matchLevel": "poor",
      "recommendation": "reject"
    },
    "requirementsComparison": {
      "items": [
        {
          "id": "r1",
          "label": "负责 Flutter 开发 / 要会 Flutter",
          "candidateSituation": "候选人简历中无 Flutter 或 Dart 经验。",
          "status": "miss",
          "comment": "候选人简历中无 Flutter 或 Dart 经验。",
          "evidence": []
        }
      ],
      "matchedItems": [],
      "attentionItems": []
    },
    "candidateAnalysis": {
      "strengths": [],
      "weaknesses": [],
      "risks": [],
      "suggestedInterviewQuestions": []
    },
    "finalRecommendation": {
      "recommendation": "reject",
      "text": "候选人与岗位的核心技术栈不匹配，不建议推荐。"
    },
    "resume": {
      "text": null,
      "textAvailable": false,
      "highlightAvailable": false
    },
    "fallback": {
      "markdownReport": "# 候选人简历评估报告...",
      "shouldUseMarkdownFallback": false
    }
  }
}
```

## 页面模块映射

| 页面区域 | 数据路径 | 说明 |
| --- | --- | --- |
| 简历摘要 | `sections.summary` | 顶部摘要卡片。 |
| 候选人信息 | `sections.candidateInfo` | 姓名、应聘岗位、当前岗位、年限、学历、任务状态；失败时展示 `taskErrorMessage`。 |
| 评估结论 | `sections.assessment` | 分数、匹配等级、AI 建议。 |
| 岗位要求对比 | `sections.requirementsComparison.items` | 表格的完整数据。 |
| 匹配亮点 | `sections.requirementsComparison.matchedItems` | 后端已筛选，仅包含 `pass`。 |
| 重点关注 | `sections.requirementsComparison.attentionItems` | 后端已筛选，包含 `partial` 与 `miss`。 |
| 优势 / 劣势 / 风险 / 面试建议 | `sections.candidateAnalysis` | 缺失时始终返回空数组。 |
| 最终筛选建议 | `sections.finalRecommendation` | `text` 与顶部摘要可相同，供底部卡片独立渲染。 |
| PDF 预览与高亮 | `sections.resume` | `highlightAvailable` 表示当前详情有 PDF 预览地址和可用于 textLayer 匹配的 evidence。 |
| Markdown 降级 | `sections.fallback` | 仅在 `shouldUseMarkdownFallback` 为 `true` 时使用。 |

## 岗位要求对比字段

```ts
type RequirementMatchStatus = 'pass' | 'partial' | 'miss'

interface RequirementEvidence {
  // 必须是从 PDF/简历原文逐字复制的片段；前端用 pdf.js textLayer 文本索引定位。
  text: string
  // 可选：模型给出的证据解释。
  reason?: string
  // 可选：证据分类，例如 experience、skill、education、missing。
  type?: string
}

interface ScreeningRequirement {
  id: string
  label: string
  // 用于表格「候选人实际情况」列；由模型的情况说明、evidence 或 comment 生成。
  candidateSituation: string | null
  status: RequirementMatchStatus
  // 用于「证据及备注」列的解释；可以为 null。
  comment: string | null
  evidence: RequirementEvidence[]
}
```

约束：

- `items` 是完整表格，`matchedItems` 和 `attentionItems` 是同一批数据的展示分组；不要把三组内容同时堆叠成同一张表。
- `evidence.text` 由 Dify 从简历原文逐字复制，后端只做空值清洗和原样保存，不返回 `start` / `end` / page / bbox 等坐标信息。
- 前端应在 pdf.js textLayer 渲染完成后建立文本索引，用 `evidence.text` 做精确匹配或轻量 normalize 匹配，再映射到对应 span 高亮。
- `resume.text === null` 不代表结构化筛选结果不可用，也不代表 PDF 高亮不可用；新高亮以 PDF 预览和 textLayer 为准。
- `highlightAvailable === true` 只表示后端有可用于定位的 evidence 和 PDF 预览地址；实际是否命中由前端 textLayer 匹配结果决定。
- 任务状态为 `failed` 时，优先展示 `errorMessage`（或 `candidateInfo.taskErrorMessage`），不要将空的评分或 Markdown 报告误呈现为成功结果。

## Markdown 降级规则

`markdownReport` 是导出/兼容用的降级内容，不是结构化详情页的数据源。后端仅在任务状态为 `success`、且没有结构化 `requirements` 时将 `shouldUseMarkdownFallback` 设为 `true`：

```ts
const sections = response.data.sections
const useMarkdownFallback = sections.fallback.shouldUseMarkdownFallback

if (useMarkdownFallback) {
  // 使用安全的 Markdown 渲染器，并启用 GFM 表格支持。
  renderMarkdown(sections.fallback.markdownReport)
} else {
  renderStructuredScreeningDetail(sections)
}
```

禁止同时渲染完整 `markdownReport` 与结构化 `sections`，否则摘要、要求对比和建议会重复；也不能把 Markdown 当普通文本节点显示。
