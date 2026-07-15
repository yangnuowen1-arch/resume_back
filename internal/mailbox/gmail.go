package mailbox

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// ProviderGoogle 是 Gmail 平台标识，与 mailbox_accounts.provider 对应。
const ProviderGoogle = "google"

// GmailProvider 基于 Gmail API 实现 Provider 接口。
type GmailProvider struct {
	oauthConfig *oauth2.Config
}

// NewGmailProvider 构造 Gmail Provider，scope 固定为只读。
func NewGmailProvider(clientID, clientSecret, redirectURL string) *GmailProvider {
	return &GmailProvider{
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes: []string{
				gmailapi.GmailReadonlyScope,
				gmailapi.GmailModifyScope, // MarkRead 需要写权限（移除 UNREAD 标签）
			},
		},
	}
}

func (p *GmailProvider) Provider() string { return ProviderGoogle }

func (p *GmailProvider) AuthURL(state string) string {
	// AccessTypeOffline + prompt=consent 确保拿到 refresh_token。
	return p.oauthConfig.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

func (p *GmailProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.oauthConfig.Exchange(ctx, code)
}

func (p *GmailProvider) RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error) {
	// TokenSource 会在 token 过期时自动用 refresh_token 换新。
	return p.oauthConfig.TokenSource(ctx, token).Token()
}

func (p *GmailProvider) GetUserEmail(ctx context.Context, token *oauth2.Token) (string, error) {
	svc, err := p.service(ctx, token)
	if err != nil {
		return "", err
	}

	profile, err := svc.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gmail get profile: %w", err)
	}

	return strings.ToLower(profile.EmailAddress), nil
}

// service 用给定 token 构造 Gmail API 客户端。
func (p *GmailProvider) service(ctx context.Context, token *oauth2.Token) (*gmailapi.Service, error) {
	httpClient := p.oauthConfig.Client(ctx, token)
	return gmailapi.NewService(ctx, option.WithHTTPClient(httpClient))
}

func (p *GmailProvider) ListUnread(ctx context.Context, token *oauth2.Token) ([]Message, error) {
	svc, err := p.service(ctx, token)
	if err != nil {
		return nil, err
	}

	// 只拉未读且带附件的邮件，减少无关邮件的元信息拉取。
	listResp, err := svc.Users.Messages.List("me").
		Q("is:unread has:attachment").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("gmail list unread: %w", err)
	}

	messages := make([]Message, 0, len(listResp.Messages))
	for _, ref := range listResp.Messages {
		// metadata 格式只取信头，避免拉全文正文。它不会稳定地返回 MIME 子分段，
		// 因此不能据此判断附件；查询本身已用 has:attachment 限定了结果集。
		full, err := svc.Users.Messages.Get("me", ref.Id).
			Format("metadata").
			MetadataHeaders("From", "Subject").
			Context(ctx).
			Do()
		if err != nil {
			return nil, fmt.Errorf("gmail get message %s: %w", ref.Id, err)
		}
		messages = append(messages, parseGmailAttachmentQueryMessage(full))
	}
	return messages, nil
}

func (p *GmailProvider) FetchAttachments(ctx context.Context, token *oauth2.Token, messageID string) ([]Attachment, error) {
	svc, err := p.service(ctx, token)
	if err != nil {
		return nil, err
	}

	full, err := svc.Users.Messages.Get("me", messageID).
		Format("full").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("gmail get message %s: %w", messageID, err)
	}
	if full.Payload == nil {
		return nil, nil
	}

	var attachments []Attachment
	// 收集所有带文件名的叶子 part。
	for _, part := range flattenGmailParts(full.Payload) {
		if part.Filename == "" || part.Body == nil {
			continue
		}

		data := part.Body.Data
		// 大附件的字节需要单独一次 attachments.get 拉取。
		if data == "" && part.Body.AttachmentId != "" {
			body, err := svc.Users.Messages.Attachments.
				Get("me", messageID, part.Body.AttachmentId).
				Context(ctx).
				Do()
			if err != nil {
				return nil, fmt.Errorf("gmail get attachment %s: %w", part.Body.AttachmentId, err)
			}
			data = body.Data
		}
		if data == "" {
			continue
		}

		decoded, err := base64.URLEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("gmail decode attachment %s: %w", part.Filename, err)
		}

		attachments = append(attachments, Attachment{
			Filename:    part.Filename,
			ContentType: part.MimeType,
			Data:        decoded,
		})
	}
	return attachments, nil
}

func (p *GmailProvider) MarkRead(ctx context.Context, token *oauth2.Token, messageID string) error {
	svc, err := p.service(ctx, token)
	if err != nil {
		return err
	}
	_, err = svc.Users.Messages.Modify("me", messageID, &gmailapi.ModifyMessageRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("gmail mark read %s: %w", messageID, err)
	}
	return nil
}

// parseGmailMessage 从 Gmail 邮件里提取统一的 Message 元信息。
func parseGmailMessage(m *gmailapi.Message) Message {
	msg := Message{ID: m.Id}
	if m.Payload == nil {
		return msg
	}
	for _, h := range m.Payload.Headers {
		switch h.Name {
		case "From":
			msg.FromEmail, msg.FromName = parseFromHeader(h.Value)
		case "Subject":
			msg.Subject = h.Value
		}
	}
	for _, part := range flattenGmailParts(m.Payload) {
		if part.Filename != "" {
			msg.HasAttachments = true
			break
		}
	}
	return msg
}

// parseGmailAttachmentQueryMessage 将 Gmail has:attachment 查询结果转为 Message。
// metadata 响应通常没有 MIME parts，附件实际在 FetchAttachments 的 full 响应中读取；
// 这里直接标记为有附件，确保扫描任务会下载并按白名单过滤真实附件。
func parseGmailAttachmentQueryMessage(m *gmailapi.Message) Message {
	msg := parseGmailMessage(m)
	msg.HasAttachments = true
	return msg
}

// flattenGmailParts 递归展平 MIME 树，返回所有 part（含容器与叶子）。
func flattenGmailParts(part *gmailapi.MessagePart) []*gmailapi.MessagePart {
	if part == nil {
		return nil
	}
	parts := []*gmailapi.MessagePart{part}
	for _, child := range part.Parts {
		parts = append(parts, flattenGmailParts(child)...)
	}
	return parts
}

// parseFromHeader 解析 "Display Name <addr@x.com>" 形式的发件人信头，
// 返回小写规范化的邮箱和显示名。解析失败时退化为原始值。
func parseFromHeader(raw string) (email string, name string) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(raw)), ""
	}
	return strings.ToLower(addr.Address), addr.Name
}
