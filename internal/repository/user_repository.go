package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/query"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	// CreateWithRoles 以事务方式创建用户并绑定角色。
	// 任一步失败都会回滚，避免出现“用户建了但角色没绑”的脏数据。
	CreateWithRoles(ctx context.Context, user *model.User, roleCodes []string) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	// FindRoleCodesByUserID 返回用户的角色 code（如 user/admin）。
	FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateLastLoginAt(ctx context.Context, userID int64, lastLoginAt time.Time) error
}

type userRepository struct {
	q  *query.Query
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		q:  query.Use(db),
		db: db,
	}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.q.User.WithContext(ctx).Create(user)
}

func (r *userRepository) CreateWithRoles(ctx context.Context, user *model.User, roleCodes []string) error {
	// 事务边界：回调返回 error -> 回滚；返回 nil -> 提交。
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) 先创建用户，拿到 user.ID，后续 user_roles 需要它。
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		if len(roleCodes) == 0 {
			return nil
		}

		var roles []model.Role
		// 2) 根据角色 code 批量查询角色表，准备建立 code -> role_id 映射。
		if err := tx.Table(model.TableNameRole).
			Where("code IN ?", roleCodes).
			Find(&roles).Error; err != nil {
			return err
		}

		roleIDByCode := make(map[string]int64, len(roles))
		for _, role := range roles {
			roleIDByCode[role.Code] = role.ID
		}

		// 3) 强校验：入参中的每个角色码都必须存在。
		// 少一个就返回错误并回滚，避免只绑定部分角色。
		for _, code := range roleCodes {
			if _, ok := roleIDByCode[code]; !ok {
				return fmt.Errorf("角色不存在: %s", code)
			}
		}

		userRoles := make([]*model.UserRole, 0, len(roleCodes))
		for _, code := range roleCodes {
			userRoles = append(userRoles, &model.UserRole{
				UserID: user.ID,
				RoleID: roleIDByCode[code],
			})
		}

		// 4) 统一写入 user_roles。
		return tx.Create(userRoles).Error
	})
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	u := r.q.User

	return u.WithContext(ctx).
		Where(u.Username.Eq(username)).
		First()
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	u := r.q.User

	return u.WithContext(ctx).
		Where(u.Email.Eq(email)).
		First()
}

// 读真实角色 → 签 token
func (r *userRepository) FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error) {
	codes := make([]string, 0)

	joinClause := "JOIN " + model.TableNameRole + " ON " + model.TableNameRole + ".id = " + model.TableNameUserRole + ".role_id"
	whereClause := model.TableNameUserRole + ".user_id = ?"
	orderClause := model.TableNameRole + ".id ASC"

	// 从 user_roles 关联 roles，只拿 code。
	// 这里不使用 DISTINCT，避免 PostgreSQL 在 DISTINCT + ORDER BY 非选择列时的约束报错。
	err := r.db.WithContext(ctx).
		Table(model.TableNameUserRole).
		Joins(joinClause).
		Where(whereClause, userID).
		Order(orderClause).
		Pluck(model.TableNameRole+".code", &codes).Error
	if err != nil {
		return nil, err
	}

	return codes, nil
}

func (r *userRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	u := r.q.User

	count, err := u.WithContext(ctx).
		Where(u.Username.Eq(username)).
		Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	u := r.q.User

	count, err := u.WithContext(ctx).
		Where(u.Email.Eq(email)).
		Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *userRepository) UpdateLastLoginAt(ctx context.Context, userID int64, lastLoginAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"last_login_at": lastLoginAt,
			"updated_at":    lastLoginAt,
		}).Error
}
