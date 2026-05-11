package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/config"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUsernameExists    = errors.New("用户名已存在")
	ErrEmailExists       = errors.New("邮箱已存在")
	ErrInvalidCredential = errors.New("账号或密码错误")
	ErrUserDisabled      = errors.New("账号已被禁用")
)

// 记得加上事务！！
type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error)
}

type authService struct {
	repo repository.UserRepository
	cfg  *config.Config
}

func NewAuthService(repo repository.UserRepository, cfg *config.Config) AuthService {
	return &authService{
		repo: repo,
		cfg:  cfg,
	}
}

func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = trimOptionalString(req.Email)
	req.Phone = trimOptionalString(req.Phone)
	req.RealName = trimOptionalString(req.RealName)

	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	if req.Email != nil {
		exists, err = s.repo.ExistsByEmail(ctx, *req.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEmailExists
		}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(passwordHash),
		RealName:     req.RealName,
		Status:       "active",
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return s.buildAuthResponse(user)
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	account := strings.TrimSpace(req.Account)

	user, err := s.repo.FindByUsername(ctx, account)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user, err = s.repo.FindByEmail(ctx, account)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidCredential
	}
	if err != nil {
		return nil, err
	}

	if user.Status != "active" {
		return nil, ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredential
	}

	now := time.Now()
	if err := s.repo.UpdateLastLoginAt(ctx, user.ID, now); err != nil {
		return nil, err
	}
	user.LastLoginAt = &now

	return s.buildAuthResponse(user)
}

func (s *authService) buildAuthResponse(user *model.User) (*dto.AuthResponse, error) {
	roles := []string{"user"}

	token, err := auth.GenerateToken(user.ID, user.Username, roles, s.cfg.JWTSecret, s.cfg.JWTExpireHours)
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		Token:     token,
		TokenType: "Bearer",
		User:      toUserResponse(user),
		Roles:     roles,
	}, nil
}

func toUserResponse(user *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Phone:     user.Phone,
		RealName:  user.RealName,
		AvatarURL: user.AvatarURL,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
	}
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
