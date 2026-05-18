package repository

import (
	"context"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type TagRepository interface {
	Create(ctx context.Context, tag *model.Tag) error
	Update(ctx context.Context, tag *model.Tag) error
	FindByID(ctx context.Context, id int64) (*model.Tag, error)
	ExistsByNameInGroup(ctx context.Context, name string, groupID *int64) (bool, error)
	ExistsByNameInGroupExceptID(ctx context.Context, name string, groupID *int64, id int64) (bool, error)
	List(ctx context.Context, keyword string, groupID *int64, status string, page int, pageSize int) ([]*model.Tag, int64, error)
}

type tagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) TagRepository {
	return &tagRepository{
		db: db,
	}
}

func (r *tagRepository) Create(ctx context.Context, tag *model.Tag) error {
	return r.db.WithContext(ctx).Create(tag).Error
}

func (r *tagRepository) Update(ctx context.Context, tag *model.Tag) error {
	return r.db.WithContext(ctx).
		Model(&model.Tag{}).
		Where("id = ?", tag.ID).
		Updates(map[string]interface{}{
			"group_id": tag.GroupID,
			"name":     tag.Name,
			"color":    tag.Color,
			"status":   tag.Status,
		}).Error
}

func (r *tagRepository) FindByID(ctx context.Context, id int64) (*model.Tag, error) {
	tag := &model.Tag{}
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(tag).Error
	if err != nil {
		return nil, err
	}

	return tag, nil
}

func (r *tagRepository) ExistsByNameInGroup(ctx context.Context, name string, groupID *int64) (bool, error) {
	queryBuilder := r.db.WithContext(ctx).
		Model(&model.Tag{}).
		Where("name = ?", name)

	if groupID == nil {
		queryBuilder = queryBuilder.Where("group_id IS NULL")
	} else {
		queryBuilder = queryBuilder.Where("group_id = ?", *groupID)
	}

	var count int64
	if err := queryBuilder.Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *tagRepository) ExistsByNameInGroupExceptID(ctx context.Context, name string, groupID *int64, id int64) (bool, error) {
	queryBuilder := r.db.WithContext(ctx).
		Model(&model.Tag{}).
		Where("name = ?", name).
		Where("id <> ?", id)

	if groupID == nil {
		queryBuilder = queryBuilder.Where("group_id IS NULL")
	} else {
		queryBuilder = queryBuilder.Where("group_id = ?", *groupID)
	}

	var count int64
	if err := queryBuilder.Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *tagRepository) List(
	ctx context.Context,
	keyword string,
	groupID *int64,
	status string,
	page int,
	pageSize int,
) ([]*model.Tag, int64, error) {
	queryBuilder := r.db.WithContext(ctx).Model(&model.Tag{})

	if keyword != "" {
		queryBuilder = queryBuilder.Where("name LIKE ?", "%"+keyword+"%")
	}

	if groupID != nil {
		queryBuilder = queryBuilder.Where("group_id = ?", *groupID)
	}

	if status != "" {
		queryBuilder = queryBuilder.Where("status = ?", status)
	}

	var total int64
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*model.Tag, 0)
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
