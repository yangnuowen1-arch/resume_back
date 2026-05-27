package repository

import (
	"context"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/query"
	"gorm.io/gorm"
)

type TagGroupRepository interface {
	Create(ctx context.Context, group *model.TagGroup) error
	Update(ctx context.Context, group *model.TagGroup) error
	FindByID(ctx context.Context, id int64) (*model.TagGroup, error)
	ExistsByName(ctx context.Context, name string) (bool, error)
	ExistsByNameExceptID(ctx context.Context, name string, id int64) (bool, error)
	List(ctx context.Context, keyword string, status string, page int, pageSize int) ([]*model.TagGroup, int64, error)
	ListAll(ctx context.Context, status string) ([]*model.TagGroup, error)
}

type tagGroupRepository struct {
	q  *query.Query
	db *gorm.DB
}

func NewTagGroupRepository(db *gorm.DB) TagGroupRepository {
	return &tagGroupRepository{
		q:  query.Use(db),
		db: db,
	}
}

func (r *tagGroupRepository) Create(ctx context.Context, group *model.TagGroup) error {
	return r.q.TagGroup.WithContext(ctx).Create(group)
}

func (r *tagGroupRepository) Update(ctx context.Context, group *model.TagGroup) error {
	return r.db.WithContext(ctx).
		Model(&model.TagGroup{}).
		Where("id = ?", group.ID).
		Updates(map[string]interface{}{
			"name":        group.Name,
			"description": group.Description,
			"status":      group.Status,
		}).Error
}

func (r *tagGroupRepository) FindByID(ctx context.Context, id int64) (*model.TagGroup, error) {
	tg := r.q.TagGroup

	return tg.WithContext(ctx).
		Where(tg.ID.Eq(id)).
		First()
}

func (r *tagGroupRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	tg := r.q.TagGroup

	count, err := tg.WithContext(ctx).
		Where(tg.Name.Eq(name)).
		Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *tagGroupRepository) ExistsByNameExceptID(ctx context.Context, name string, id int64) (bool, error) {
	tg := r.q.TagGroup

	count, err := tg.WithContext(ctx).
		Where(tg.Name.Eq(name), tg.ID.Neq(id)).
		Count()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *tagGroupRepository) List(
	ctx context.Context,
	keyword string,
	status string,
	page int,
	pageSize int,
) ([]*model.TagGroup, int64, error) {
	tg := r.q.TagGroup
	queryBuilder := tg.WithContext(ctx)

	if keyword != "" {
		queryBuilder = queryBuilder.Where(tg.Name.Like("%" + keyword + "%"))
	}

	if status != "" {
		queryBuilder = queryBuilder.Where(tg.Status.Eq(status))
	}

	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, err
	}

	items, err := queryBuilder.
		Order(tg.ID.Desc()).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find()
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *tagGroupRepository) ListAll(ctx context.Context, status string) ([]*model.TagGroup, error) {
	tg := r.q.TagGroup
	queryBuilder := tg.WithContext(ctx)

	if status != "" {
		queryBuilder = queryBuilder.Where(tg.Status.Eq(status))
	}

	return queryBuilder.Order(tg.ID.Desc()).Find()
}
