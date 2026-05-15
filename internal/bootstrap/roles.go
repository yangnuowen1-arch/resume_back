package bootstrap

import (
	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type roleSeed struct {
	Code string
	Name string
}

var defaultRoles = []roleSeed{
	{Code: auth.DefaultUserRoleCode, Name: "普通用户"},
	{Code: auth.DefaultAdminRoleCode, Name: "系统管理员"},
}

func EnsureDefaultRoles(db *gorm.DB) error {
	// 启动时做幂等初始化：存在就跳过，不存在就创建。
	// 这样空库首次启动也能保证注册默认角色可用。
	return db.Transaction(func(tx *gorm.DB) error {
		for _, seed := range defaultRoles {
			role := model.Role{}
			if err := tx.Where("code = ?", seed.Code).
				Attrs(model.Role{
					Code: seed.Code,
					Name: seed.Name,
				}).
				FirstOrCreate(&role).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
