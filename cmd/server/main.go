// @title AI Resume Screening API
// @version 1.0
// @description AI 智能简历筛选系统后端接口文档
// @host localhost:8081
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

package main

import (
	"log"
	"net/http"

	"github.com/yangnuowen1-arch/resume_back/internal/config"
	"github.com/yangnuowen1-arch/resume_back/internal/db"
	"github.com/yangnuowen1-arch/resume_back/internal/router"
)

func main() {
	cfg := config.LoadConfig()

	database := db.ConnectDB(cfg)

	app := router.SetupRouter(database, cfg)

	log.Println("服务启动成功，端口:", cfg.AppPort)

	if err := app.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}

	err := http.ListenAndServe("0.0.0.0:8081", nil)
	if err != nil {
		log.Fatal(err)
	}
}
