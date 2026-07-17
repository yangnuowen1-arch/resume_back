// Package mailbox 定义邮箱收件箱的 Provider 抽象层。
//
// Provider 屏蔽 Gmail 的差异，向上层（Phase 4 的扫描导入核心）
// 暴露统一的鉴权、拉未读、取附件、标已读能力。上层只依赖本包定义的
// Message / Attachment 结构，不感知任何具体邮箱平台的 API。
package mailbox

import (
	"context"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
)

// Provider 是单个邮箱平台（Gmail）的统一抽象。
//
// 鉴权分两个阶段：
//   - AuthURL / Exchange 用于 OAuth 首次授权（换取 token）。
//   - RefreshToken 用于 token 过期后的刷新。
//
// 数据面：
//   - ListUnread 拉取未读邮件（仅元信息，不含附件字节，避免一次性拉大对象）。
//   - FetchAttachments 按需拉取单封邮件的附件字节。
//   - MarkRead 处理完成后把邮件标记为已读。
type Provider interface {
	// Provider 返回平台标识（"google"），与 mailbox_accounts.provider 对应。
	Provider() string

	// AuthURL 生成 OAuth 授权跳转地址，state 用于回调防 CSRF。
	AuthURL(state string) string

	// Exchange 用回调拿到的 code 换取 token。
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)

	// RefreshToken 在 token 过期时刷新，返回新的 token。
	RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error)

	// GetUserEmail 获取授权用户的邮箱地址。
	GetUserEmail(ctx context.Context, token *oauth2.Token) (string, error)

	// ListUnread 拉取未读邮件的元信息（不含附件字节）。
	// token 由调用方保证有效（必要时先 RefreshToken）。
	ListUnread(ctx context.Context, token *oauth2.Token) ([]Message, error)

	// FetchAttachments 拉取指定邮件的全部附件字节。
	FetchAttachments(ctx context.Context, token *oauth2.Token, messageID string) ([]Attachment, error)

	// MarkRead 把指定邮件标记为已读。
	MarkRead(ctx context.Context, token *oauth2.Token, messageID string) error
}

// Message 是跨平台统一的邮件元信息。
type Message struct {
	// ID 是平台内的邮件唯一标识（Gmail message id），
	// 落库到 mailbox_messages.message_id 用于防重复扫描。
	ID string

	// FromEmail 发件人邮箱（小写规范化后用于合并候选人）。
	FromEmail string

	// FromName 发件人显示名，作为候选人 name 的回退来源。
	FromName string

	// Subject 邮件主题（仅用于日志 / 排查，不参与业务判定）。
	Subject string

	// HasAttachments 标记邮件是否带附件，供上层决定是否调用 FetchAttachments。
	HasAttachments bool
}

// Attachment 是跨平台统一的附件数据（含字节流）。
type Attachment struct {
	// ID 是邮件提供方给附件的稳定标识（例如 Gmail attachmentId / MIME partId）。
	// 用于在同一封邮件重试时识别同一个附件；Provider 无法提供时调用方可回退到序号。
	ID string

	// Filename 原始附件文件名（含扩展名）。
	Filename string

	// ContentType MIME 类型（如 application/pdf）。
	ContentType string

	// Data 附件完整字节内容。
	Data []byte
}

// Ext 返回附件的小写扩展名（含点，如 ".pdf"）。无扩展名时返回空串。
func (a Attachment) Ext() string {
	return strings.ToLower(filepath.Ext(a.Filename))
}

// AllowedExtSet 把逗号分隔的白名单（如 ".pdf,.docx"）解析为集合，
// 每一项统一为小写并补齐前导点，便于与 Attachment.Ext() 直接比较。
func AllowedExtSet(csv string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, raw := range strings.Split(csv, ",") {
		ext := strings.ToLower(strings.TrimSpace(raw))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		set[ext] = struct{}{}
	}
	return set
}

// FilterAttachments 按扩展名白名单过滤附件。allowed 为空集时返回空切片（不放行任何附件）。
func FilterAttachments(attachments []Attachment, allowed map[string]struct{}) []Attachment {
	filtered := make([]Attachment, 0, len(attachments))
	for _, att := range attachments {
		if _, ok := allowed[att.Ext()]; ok {
			filtered = append(filtered, att)
		}
	}
	return filtered
}
