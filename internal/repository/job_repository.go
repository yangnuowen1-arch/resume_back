package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type JobTagWithTag struct {
	JobID     int64
	TagID     int64
	GroupID   *int64
	Name      string
	Color     *string
	Status    string
	CreatedAt time.Time
}

type JobMemberWithUser struct {
	ID         int64
	JobID      int64
	UserID     int64
	Username   string
	RealName   *string
	Email      *string
	UserStatus string
	MemberRole string
	CreatedBy  *int64
	CreatedAt  time.Time
}

type JobRepository interface {
	Create(ctx context.Context, job *model.Job) error
	Update(ctx context.Context, job *model.Job) error
	FindByID(ctx context.Context, id int64) (*model.Job, error)
	List(ctx context.Context, keyword string, categoryID *int64, status string, page int, pageSize int) ([]*model.Job, int64, error)

	// CategoryExists 判断岗位分类是否存在，用于创建或更新岗位时校验 category_id 是否合法。
	CategoryExists(ctx context.Context, id int64) (bool, error)

	// UserExists 判断用户是否存在，用于校验岗位负责人或岗位成员是否是有效用户。
	UserExists(ctx context.Context, id int64) (bool, error)

	// CountTagsByIDs 统计传入的标签 ID 中真实存在多少个，用于判断是否有无效标签 ID。
	CountTagsByIDs(ctx context.Context, ids []int64) (int64, error)

	// BindTags 给岗位重新绑定标签，会先清空旧关联，再写入新的岗位-标签关系。
	BindTags(ctx context.Context, jobID int64, tagIDs []int64) error
	ListTags(ctx context.Context, jobID int64) ([]JobTagWithTag, error)

	// AssignMember 给岗位分配成员；如果该用户已是岗位成员，则更新成员角色。
	AssignMember(ctx context.Context, member *model.JobMember) error
	ListMembers(ctx context.Context, jobID int64) ([]JobMemberWithUser, error)
}

type jobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{
		db: db,
	}
}

func (r *jobRepository) Create(ctx context.Context, job *model.Job) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *jobRepository) Update(ctx context.Context, job *model.Job) error {
	return r.db.WithContext(ctx).
		Model(&model.Job{}).
		Where("id = ?", job.ID).
		Updates(map[string]interface{}{
			"category_id":      job.CategoryID,
			"title":            job.Title,
			"department":       job.Department,
			"headcount":        job.Headcount,
			"work_location":    job.WorkLocation,
			"work_type":        job.WorkType,
			"employment_type":  job.EmploymentType,
			"salary_min":       job.SalaryMin,
			"salary_max":       job.SalaryMax,
			"salary_months":    job.SalaryMonths,
			"experience_min":   job.ExperienceMin,
			"experience_max":   job.ExperienceMax,
			"education_level":  job.EducationLevel,
			"description":      job.Description,
			"responsibilities": job.Responsibilities,
			"requirements":     job.Requirements,
			"bonus_points":     job.BonusPoints,
			"status":           job.Status,
			"priority":         job.Priority,
			"owner_user_id":    job.OwnerUserID,
		}).Error
}

func (r *jobRepository) FindByID(ctx context.Context, id int64) (*model.Job, error) {
	job := &model.Job{}
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(job).Error
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (r *jobRepository) List(
	ctx context.Context,
	keyword string,
	categoryID *int64,
	status string,
	page int,
	pageSize int,
) ([]*model.Job, int64, error) {
	queryBuilder := r.db.WithContext(ctx).Model(&model.Job{})

	if keyword != "" {
		queryBuilder = queryBuilder.Where("title LIKE ?", "%"+keyword+"%")
	}

	if categoryID != nil {
		queryBuilder = queryBuilder.Where("category_id = ?", *categoryID)
	}

	if status != "" {
		queryBuilder = queryBuilder.Where("status = ?", status)
	}

	var total int64
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*model.Job, 0)
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

func (r *jobRepository) CategoryExists(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.JobCategory{}).
		Where("id = ?", id).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *jobRepository) UserExists(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *jobRepository) CountTagsByIDs(ctx context.Context, ids []int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Tag{}).
		Where("id IN ?", ids).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *jobRepository) BindTags(ctx context.Context, jobID int64, tagIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", jobID).Delete(&model.JobTag{}).Error; err != nil {
			return err
		}

		if len(tagIDs) == 0 {
			return nil
		}

		jobTags := make([]*model.JobTag, 0, len(tagIDs))
		for _, tagID := range tagIDs {
			jobTags = append(jobTags, &model.JobTag{
				JobID: jobID,
				TagID: tagID,
			})
		}

		return tx.Create(jobTags).Error
	})
}

func (r *jobRepository) ListTags(ctx context.Context, jobID int64) ([]JobTagWithTag, error) {
	items := make([]JobTagWithTag, 0)
	err := r.db.WithContext(ctx).
		Table(model.TableNameJobTag).
		Select("job_tags.job_id, job_tags.tag_id, tags.group_id, tags.name, tags.color, tags.status, job_tags.created_at").
		Joins("JOIN "+model.TableNameTag+" ON tags.id = job_tags.tag_id").
		Where("job_tags.job_id = ?", jobID).
		Order("tags.id ASC").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (r *jobRepository) AssignMember(ctx context.Context, member *model.JobMember) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing := &model.JobMember{}
		err := tx.Where("job_id = ? AND user_id = ?", member.JobID, member.UserID).First(existing).Error
		if err == nil {
			member.ID = existing.ID
			return tx.Model(existing).
				Updates(map[string]interface{}{
					"member_role": member.MemberRole,
					"created_by":  member.CreatedBy,
				}).Error
		}

		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if err := tx.Create(member).Error; err != nil {
			return fmt.Errorf("分配岗位成员失败: %w", err)
		}

		return nil
	})
}

func (r *jobRepository) ListMembers(ctx context.Context, jobID int64) ([]JobMemberWithUser, error) {
	items := make([]JobMemberWithUser, 0)
	err := r.db.WithContext(ctx).
		Table(model.TableNameJobMember).
		Select("job_members.id, job_members.job_id, job_members.user_id, users.username, users.real_name, users.email, users.status AS user_status, job_members.member_role, job_members.created_by, job_members.created_at").
		Joins("JOIN "+model.TableNameUser+" ON users.id = job_members.user_id").
		Where("job_members.job_id = ?", jobID).
		Order("job_members.id ASC").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}
