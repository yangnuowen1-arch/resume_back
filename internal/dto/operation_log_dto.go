package dto

import "time"

type OperationLogQuery struct {
	Page     int
	PageSize int
	User     string
	Date     *time.Time
}

type RecordOperationLogRequest struct {
	Action     string
	Module     *string
	TargetType *string
	TargetID   *int64
	BeforeData *string
	AfterData  *string
	IPAddress  *string
	UserAgent  *string
}

type OperationLogResponse struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	UserID     *int64    `json:"userId"`
	User       string    `json:"user"`
	Action     string    `json:"action"`
	Details    string    `json:"details"`
	Module     *string   `json:"module"`
	TargetType *string   `json:"targetType"`
	TargetID   *int64    `json:"targetId"`
	IPAddress  *string   `json:"ipAddress"`
	UserAgent  *string   `json:"userAgent"`
	BeforeData *string   `json:"beforeData,omitempty"`
	AfterData  *string   `json:"afterData,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}
