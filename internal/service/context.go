package service

import (
	"context"

	"github.com/yangnuowen1-arch/resume_back/internal/auth"
)

// 本文件用于放 service 层从 context 中读取公共请求信息的辅助函数。
// 例如：当前登录用户 ID、后续可能增加的租户 ID、请求来源等上下文数据。
// 这样各个 service 不需要重复解析 context，也能统一处理未登录等错误。

// currentUserID 从请求上下文中读取当前登录用户 ID。
func currentUserID(ctx context.Context) (int64, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok || claims.UserID <= 0 {
		return 0, ErrUnauthenticated
	}

	return claims.UserID, nil
}
