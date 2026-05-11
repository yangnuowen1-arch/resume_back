package repository

import (
	"context"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/query"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
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
