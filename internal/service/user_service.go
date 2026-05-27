package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("用户不存在")

type UserService interface {
	Create(ctx context.Context, req dto.CreateUserRequest) (int64, error)
	Update(ctx context.Context, id int64, req dto.UpdateUserRequest) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*dto.ManagedUserResponse, error)
	List(ctx context.Context, query dto.UserQuery) ([]dto.ManagedUserResponse, int64, error)
	AssignRoles(ctx context.Context, id int64, req dto.AssignUserRolesRequest) error
	ListRoles(ctx context.Context) ([]dto.RoleResponse, error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{
		repo: repo,
	}
}

func (s *userService) Create(ctx context.Context, req dto.CreateUserRequest) (int64, error) {
	if _, err := currentUserID(ctx); err != nil {
		return 0, err
	}

	normalizeCreateUserRequest(&req)
	if req.Status == "" {
		req.Status = "active"
	}
	if err := validateUserStatus(req.Status); err != nil {
		return 0, err
	}

	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, ErrUsernameExists
	}

	if req.Email != nil {
		exists, err = s.repo.ExistsByEmail(ctx, *req.Email)
		if err != nil {
			return 0, err
		}
		if exists {
			return 0, ErrEmailExists
		}
	}

	roleCodes := normalizeRoleCodes(req.Roles)
	if len(roleCodes) == 0 {
		roleCodes = []string{auth.DefaultUserRoleCode}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(passwordHash),
		RealName:     req.RealName,
		Status:       req.Status,
	}

	if err := s.repo.CreateWithRoles(ctx, user, roleCodes); err != nil {
		return 0, err
	}

	return user.ID, nil
}

func (s *userService) Update(ctx context.Context, id int64, req dto.UpdateUserRequest) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("用户 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return ErrUserNotFound
	}

	normalizeUpdateUserRequest(&req)
	if err := validateUserStatus(req.Status); err != nil {
		return err
	}

	exists, err := s.repo.ExistsByUsernameExceptID(ctx, req.Username, id)
	if err != nil {
		return err
	}
	if exists {
		return ErrUsernameExists
	}

	if req.Email != nil {
		exists, err = s.repo.ExistsByEmailExceptID(ctx, *req.Email, id)
		if err != nil {
			return err
		}
		if exists {
			return ErrEmailExists
		}
	}

	user := &model.User{
		ID:       id,
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		RealName: req.RealName,
		Status:   req.Status,
	}

	return s.repo.Update(ctx, user)
}

func (s *userService) Delete(ctx context.Context, id int64) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("用户 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return ErrUserNotFound
	}

	return s.repo.Disable(ctx, id)
}

func (s *userService) Get(ctx context.Context, id int64) (*dto.ManagedUserResponse, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, err
	}

	if id <= 0 {
		return nil, errors.New("用户 ID 不合法")
	}

	user, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	roles, err := s.repo.FindRoleCodesByUserID(ctx, id)
	if err != nil {
		return nil, err
	}

	response := toManagedUserResponse(user, roles)
	return &response, nil
}

func (s *userService) List(ctx context.Context, query dto.UserQuery) ([]dto.ManagedUserResponse, int64, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, 0, err
	}

	query = normalizeUserQuery(query)

	items, total, err := s.repo.List(ctx, strings.TrimSpace(query.Keyword), strings.TrimSpace(query.Status), query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]int64, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.ID)
	}

	rolesByUserID, err := s.repo.FindRoleCodesByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.ManagedUserResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toManagedUserResponse(item, rolesByUserID[item.ID]))
	}

	return result, total, nil
}

func (s *userService) AssignRoles(ctx context.Context, id int64, req dto.AssignUserRolesRequest) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("用户 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return ErrUserNotFound
	}

	roleCodes := normalizeRoleCodes(req.Roles)
	if len(roleCodes) == 0 {
		return errors.New("角色不能为空")
	}

	return s.repo.ReplaceRoles(ctx, id, roleCodes)
}

func (s *userService) ListRoles(ctx context.Context) ([]dto.RoleResponse, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, err
	}

	roles, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]dto.RoleResponse, 0, len(roles))
	for _, role := range roles {
		result = append(result, dto.RoleResponse{
			ID:          role.ID,
			Code:        role.Code,
			Name:        role.Name,
			Description: role.Description,
			CreatedAt:   role.CreatedAt,
		})
	}

	return result, nil
}

func normalizeCreateUserRequest(req *dto.CreateUserRequest) {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = trimOptionalString(req.Email)
	req.Phone = trimOptionalString(req.Phone)
	req.RealName = trimOptionalString(req.RealName)
	req.Status = strings.TrimSpace(req.Status)
}

func normalizeUpdateUserRequest(req *dto.UpdateUserRequest) {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = trimOptionalString(req.Email)
	req.Phone = trimOptionalString(req.Phone)
	req.RealName = trimOptionalString(req.RealName)
	req.Status = strings.TrimSpace(req.Status)
}

func normalizeUserQuery(query dto.UserQuery) dto.UserQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}
	query.Status = strings.TrimSpace(query.Status)
	if query.Status == "all" {
		query.Status = ""
	}

	return query
}

func validateUserStatus(status string) error {
	switch status {
	case "active", "disabled":
		return nil
	default:
		return errors.New("用户状态不合法")
	}
}

func toManagedUserResponse(user *model.User, roleCodes []string) dto.ManagedUserResponse {
	roles := normalizeRoleCodes(roleCodes)
	if roles == nil {
		roles = []string{}
	}

	return dto.ManagedUserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Phone:       user.Phone,
		RealName:    user.RealName,
		AvatarURL:   user.AvatarURL,
		Status:      user.Status,
		Roles:       roles,
		LastLoginAt: user.LastLoginAt,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}
