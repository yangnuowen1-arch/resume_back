package dto

import "time"

type ScreeningTaskQuery struct {
	Page        int
	PageSize    int
	Keyword     string
	Status      string
	JobID       *int64
	CandidateID *int64
}

type RunResumeScreeningRequest struct {
	ResumeID       int64  `json:"resumeId" binding:"required"`
	JobID          int64  `json:"jobId" binding:"required"`
	OutputLanguage string `json:"outputLanguage"`
}

type RunResumeScreeningResponse struct {
	ScreeningResultID int64    `json:"screeningResultId"`
	ApplicationID     int64    `json:"applicationId"`
	ResumeID          int64    `json:"resumeId"`
	JobID             int64    `json:"jobId"`
	Score             *float64 `json:"score"`
	MatchLevel        *string  `json:"matchLevel,omitempty"`
	Recommendation    *string  `json:"recommendation,omitempty"`
	Summary           *string  `json:"summary,omitempty"`
	MarkdownReport    *string  `json:"markdownReport,omitempty"`
	Status            string   `json:"status"`
}

type ScreeningTaskResponse struct {
	ID             int64     `json:"id"`
	ApplicationID  int64     `json:"applicationId"`
	CandidateID    *int64    `json:"candidateId"`
	Candidate      *string   `json:"candidate"`
	CandidateName  *string   `json:"candidateName"`
	JobID          int64     `json:"jobId"`
	JobTitle       string    `json:"jobTitle"`
	Position       string    `json:"position"`
	AIScore        *float64  `json:"aiScore"`
	Status         string    `json:"status"`
	Date           time.Time `json:"date"`
	CreatedAt      time.Time `json:"createdAt"`
	CreatedBy      *int64    `json:"createdBy"`
	MatchLevel     *string   `json:"matchLevel,omitempty"`
	Recommendation *string   `json:"recommendation,omitempty"`
	ErrorMessage   *string   `json:"errorMessage,omitempty"`
}

type RequirementEvidence struct {
	Text  string `json:"text"`
	Start *int   `json:"start"`
	End   *int   `json:"end"`
}

type ScreeningRequirement struct {
	ID                 string                `json:"id"`
	Label              string                `json:"label"`
	CandidateSituation *string               `json:"candidateSituation"`
	Status             string                `json:"status"`
	Comment            *string               `json:"comment"`
	Evidence           []RequirementEvidence `json:"evidence"`
}

// ScreeningSummarySection 对应详情页顶部的「简历摘要」卡片。
type ScreeningSummarySection struct {
	Text *string `json:"text"`
}

// ScreeningCandidateInfoSection 对应详情页的「候选人信息」卡片。
type ScreeningCandidateInfoSection struct {
	Name              *string  `json:"name"`
	AppliedPosition   string   `json:"appliedPosition"`
	CurrentTitle      *string  `json:"currentTitle"`
	YearsOfExperience *float64 `json:"yearsOfExperience"`
	HighestEducation  *string  `json:"highestEducation"`
	TaskStatus        string   `json:"taskStatus"`
	TaskErrorMessage  *string  `json:"taskErrorMessage"`
}

// ScreeningAssessmentSection 对应详情页的「评估结论」卡片。
type ScreeningAssessmentSection struct {
	Score          *float64 `json:"score"`
	MatchLevel     *string  `json:"matchLevel"`
	Recommendation *string  `json:"recommendation"`
}

// ScreeningRequirementsComparisonSection 对应岗位要求表、匹配亮点和重点关注三个区域。
// Items 为完整表格数据；MatchedItems 和 AttentionItems 已按页面展示规则分组，
// 前端无需再次筛选状态。
type ScreeningRequirementsComparisonSection struct {
	Items          []ScreeningRequirement `json:"items"`
	MatchedItems   []ScreeningRequirement `json:"matchedItems"`
	AttentionItems []ScreeningRequirement `json:"attentionItems"`
}

// ScreeningCandidateAnalysisSection 对应候选人优劣势、风险及面试建议区域。
type ScreeningCandidateAnalysisSection struct {
	Strengths                   []string `json:"strengths"`
	Weaknesses                  []string `json:"weaknesses"`
	Risks                       []string `json:"risks"`
	SuggestedInterviewQuestions []string `json:"suggestedInterviewQuestions"`
}

// ScreeningFinalRecommendationSection 对应详情页底部的「最终筛选建议」卡片。
type ScreeningFinalRecommendationSection struct {
	Recommendation *string `json:"recommendation"`
	Text           *string `json:"text"`
}

// ScreeningResumeSection 为简历原文和证据高亮提供状态信息。
type ScreeningResumeSection struct {
	Text               *string `json:"text"`
	TextAvailable      bool    `json:"textAvailable"`
	HighlightAvailable bool    `json:"highlightAvailable"`
}

// ScreeningFallbackSection 仅用于结构化数据缺失时的 Markdown 降级展示。
type ScreeningFallbackSection struct {
	MarkdownReport            *string `json:"markdownReport"`
	ShouldUseMarkdownFallback bool    `json:"shouldUseMarkdownFallback"`
}

// ScreeningTaskDetailSections 按详情页的显示模块组织筛选结果。
type ScreeningTaskDetailSections struct {
	Summary                ScreeningSummarySection                `json:"summary"`
	CandidateInfo          ScreeningCandidateInfoSection          `json:"candidateInfo"`
	Assessment             ScreeningAssessmentSection             `json:"assessment"`
	RequirementsComparison ScreeningRequirementsComparisonSection `json:"requirementsComparison"`
	CandidateAnalysis      ScreeningCandidateAnalysisSection      `json:"candidateAnalysis"`
	FinalRecommendation    ScreeningFinalRecommendationSection    `json:"finalRecommendation"`
	Resume                 ScreeningResumeSection                 `json:"resume"`
	Fallback               ScreeningFallbackSection               `json:"fallback"`
}

type ScreeningTaskDetailResponse struct {
	ID             int64                  `json:"id"`
	Status         string                 `json:"status"`
	ErrorMessage   *string                `json:"errorMessage"`
	CandidateName  *string                `json:"candidateName"`
	Position       string                 `json:"position"`
	AIScore        *float64               `json:"aiScore"`
	MatchLevel     *string                `json:"matchLevel"`
	Recommendation *string                `json:"recommendation"`
	Summary        *string                `json:"summary"`
	MarkdownReport *string                `json:"markdownReport,omitempty"`
	ResumeText     *string                `json:"resumeText"`
	Requirements   []ScreeningRequirement `json:"requirements"`
	// Sections 是详情页的新主数据源；保留上方平铺字段以兼容已发布的前端。
	Sections ScreeningTaskDetailSections `json:"sections"`
}
