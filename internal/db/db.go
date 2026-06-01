package db

import (
	"fmt"
	"log"

	"github.com/yangnuowen1-arch/resume_back/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectDB(cfg *config.Config) *gorm.DB { //传入配置对象，返回一个 GORM 数据库连接对象
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Shanghai",
		cfg.DBHost,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBPort,
		cfg.DBSSLMode,
	)

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}

	if err := ensureJobDynamicFieldsColumn(database); err != nil {
		log.Fatalf("岗位动态字段初始化失败: %v", err)
	}
	if err := ensureCandidateStatusColumn(database); err != nil {
		log.Fatalf("候选人状态初始化失败: %v", err)
	}
	if err := ensureCandidateCurrentPositionCategoryColumn(database); err != nil {
		log.Fatalf("候选人职位类别初始化失败: %v", err)
	}
	if err := ensureCandidatePositionRelationColumns(database); err != nil {
		log.Fatalf("候选人岗位关联初始化失败: %v", err)
	}

	log.Println("数据库连接成功")
	return database
}

func ensureJobDynamicFieldsColumn(database *gorm.DB) error {
	return database.Exec(
		`DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_name = 'jobs'
			AND column_name = 'custom_fields'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_name = 'jobs'
			AND column_name = 'dynamic_fields'
	) THEN
		ALTER TABLE jobs RENAME COLUMN custom_fields TO dynamic_fields;
	END IF;
END $$;

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS dynamic_fields JSONB NOT NULL DEFAULT '{}'::jsonb`,
	).Error
}

func ensureCandidateStatusColumn(database *gorm.DB) error {
	return database.Exec(
		`ALTER TABLE candidates ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'new'`,
	).Error
}

func ensureCandidateCurrentPositionCategoryColumn(database *gorm.DB) error {
	return database.Exec(
		`ALTER TABLE candidates ADD COLUMN IF NOT EXISTS current_position_category VARCHAR(100)`,
	).Error
}

func ensureCandidatePositionRelationColumns(database *gorm.DB) error {
	return database.Exec(
		`ALTER TABLE candidates ADD COLUMN IF NOT EXISTS position_category_id BIGINT;
ALTER TABLE candidates ADD COLUMN IF NOT EXISTS current_job_id BIGINT`,
	).Error
}
