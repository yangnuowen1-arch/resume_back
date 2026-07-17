package db

import (
	"fmt"
	"log"

	"github.com/yangnuowen1-arch/resume_back/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ConnectDB opens the configured database connection. Database schema and data
// migrations must be applied explicitly from the migrations directory before
// deploying the backend; application startup never changes the schema.
func ConnectDB(cfg *config.Config) *gorm.DB {
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

	log.Println("数据库连接成功")
	return database
}
