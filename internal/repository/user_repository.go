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
	Update(ctx context.Context, user *model.User) error
	Disable(ctx context.Context, userID int64) error
	FindByID(ctx context.Context, userID int64) (*model.User, error)
	List(ctx context.Context, keyword string, status string, page int, pageSize int) ([]*model.User, int64, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	// FindRoleCodesByUserID 返回用户的角色 code（如 user/admin）。
	FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error)
	FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]string, error)
	ListRoles(ctx context.Context) ([]*model.Role, error)
	ReplaceRoles(ctx context.Context, userID int64, roleCodes []string) error
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	ExistsByUsernameExceptID(ctx context.Context, username string, userID int64) (bool, error)
	ExistsByEmailExceptID(ctx context.Context, email string, userID int64) (bool, error)
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

		roleIDByCode, err := roleIDByCodeInTx(tx, roleCodes)
		if err != nil {
			return err
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

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", user.ID).
		Updates(map[string]interface{}{
			"username":  user.Username,
			"email":     user.Email,
			"phone":     user.Phone,
			"real_name": user.RealName,
			"status":    user.Status,
		}).Error
}

func (r *userRepository) Disable(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("status", "disabled").Error
}

func (r *userRepository) FindByID(ctx context.Context, userID int64) (*model.User, error) {
	user := &model.User{}
	err := r.db.WithContext(ctx).
		Where("id = ?", userID).
		First(user).Error
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepository) List(
	ctx context.Context,
	keyword string,
	status string,
	page int,
	pageSize int,
) ([]*model.User, int64, error) {
	queryBuilder := r.db.WithContext(ctx).Model(&model.User{})

	if keyword != "" {
		like := "%" + keyword + "%"
		queryBuilder = queryBuilder.Where("(username LIKE ? OR real_name LIKE ? OR email LIKE ? OR phone LIKE ?)", like, like, like, like)
	}

	if status != "" {
		queryBuilder = queryBuilder.Where("status = ?", status)
	}

	var total int64
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*model.User, 0)
	err := queryBuilder.
		Order("id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
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

func (r *userRepository) FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]string, error) {
	result := make(map[int64][]string, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	type userRoleCode struct {
		UserID int64
		Code   string
	}

	rows := make([]userRoleCode, 0)
	err := r.db.WithContext(ctx).
		Table(model.TableNameUserRole).
		Select(model.TableNameUserRole+".user_id, "+model.TableNameRole+".code").
		Joins("JOIN "+model.TableNameRole+" ON "+model.TableNameRole+".id = "+model.TableNameUserRole+".role_id").
		Where(model.TableNameUserRole+".user_id IN ?", userIDs).
		Order(model.TableNameUserRole + ".user_id ASC, " + model.TableNameRole + ".id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.Code)
	}

	return result, nil
}

func (r *userRepository) ListRoles(ctx context.Context) ([]*model.Role, error) {
	roles := make([]*model.Role, 0)
	err := r.db.WithContext(ctx).
		Model(&model.Role{}).
		Order("id ASC").
		Find(&roles).Error
	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (r *userRepository) ReplaceRoles(ctx context.Context, userID int64, roleCodes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roleIDByCode, err := roleIDByCodeInTx(tx, roleCodes)
		if err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}

		if len(roleCodes) == 0 {
			return nil
		}

		userRoles := make([]*model.UserRole, 0, len(roleCodes))
		for _, code := range roleCodes {
			userRoles = append(userRoles, &model.UserRole{
				UserID: userID,
				RoleID: roleIDByCode[code],
			})
		}

		return tx.Create(userRoles).Error
	})
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

func (r *userRepository) ExistsByUsernameExceptID(ctx context.Context, username string, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("username = ? AND id <> ?", username, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *userRepository) ExistsByEmailExceptID(ctx context.Context, email string, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("email = ? AND id <> ?", email, userID).
		Count(&count).Error
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

func roleIDByCodeInTx(tx *gorm.DB, roleCodes []string) (map[string]int64, error) {
	roleIDByCode := make(map[string]int64, len(roleCodes))
	if len(roleCodes) == 0 {
		return roleIDByCode, nil
	}

	var roles []model.Role
	// 根据角色 code 批量查询角色表，准备建立 code -> role_id 映射。
	if err := tx.Table(model.TableNameRole).
		Where("code IN ?", roleCodes).
		Find(&roles).Error; err != nil {
		return nil, err
	}

	for _, role := range roles {
		roleIDByCode[role.Code] = role.ID
	}

	// 强校验：入参中的每个角色码都必须存在。
	// 少一个就返回错误并回滚，避免只绑定部分角色。
	for _, code := range roleCodes {
		if _, ok := roleIDByCode[code]; !ok {
			return nil, fmt.Errorf("角色不存在: %s", code)
		}
	}

	return roleIDByCode, nil
}
