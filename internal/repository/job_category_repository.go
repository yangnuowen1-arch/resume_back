package repository

import (
	"context"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/query"

	"gorm.io/gorm"
)

// 返回 *model.JobCategory的话是因为，需要把数据拿回来
type JobCategoryRepository interface {
	Create(ctx context.Context, category *model.JobCategory) error                                                        //插入一条记录
	FindByID(ctx context.Context, id int64) (*model.JobCategory, error)                                                   //按 ID 查一条
	ExistsByName(ctx context.Context, name string) (bool, error)                                                          //检查名字是否已存在（用于判断重名）
	List(ctx context.Context, keyword string, status string, page int, pageSize int) ([]*model.JobCategory, int64, error) //分页+筛选查列表
}

// 私有 struct，外部看不到，只能通过 interface 使用
type jobCategoryRepository struct {
	q *query.Query
}

// 传入数据库连接，返回一个可用的 repository 实例
func NewJobCategoryRepository(db *gorm.DB) JobCategoryRepository {
	return &jobCategoryRepository{
		q: query.Use(db),
	}
}

func (r *jobCategoryRepository) Create(ctx context.Context, category *model.JobCategory) error {
	return r.q.JobCategory.WithContext(ctx).Create(category)
}

func (r *jobCategoryRepository) FindByID(ctx context.Context, id int64) (*model.JobCategory, error) {
	jc := r.q.JobCategory

	return jc.WithContext(ctx).
		Where(jc.ID.Eq(id)).
		First()
}

func (r *jobCategoryRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	jc := r.q.JobCategory

	count, err := jc.WithContext(ctx).
		Where(jc.Name.Eq(name)).
		Count()

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *jobCategoryRepository) List(
	ctx context.Context,
	keyword string,
	status string,
	page int,
	pageSize int,
) ([]*model.JobCategory, int64, error) {
	jc := r.q.JobCategory

	queryBuilder := jc.WithContext(ctx)

	if keyword != "" {
		queryBuilder = queryBuilder.Where(jc.Name.Like("%" + keyword + "%"))
	}

	if status != "" {
		queryBuilder = queryBuilder.Where(jc.Status.Eq(status))
	}

	total, err := queryBuilder.Count()
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	items, err := queryBuilder.
		Order(jc.SortOrder, jc.ID.Desc()). // 排序
		Limit(pageSize).                   // 每页取几条
		Offset(offset).                    // 从第几条开始取
		Find()                             // 执行查询，返回结果

	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
