package router

import (
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/yangnuowen1-arch/resume_back/internal/config"
	"github.com/yangnuowen1-arch/resume_back/internal/handler"
	"github.com/yangnuowen1-arch/resume_back/internal/middleware"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/service"

	"github.com/gin-gonic/gin"
	_ "github.com/yangnuowen1-arch/resume_back/docs"
	"gorm.io/gorm"
)

func SetupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	r.Use(middleware.RequestIDMiddleware())

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api/v1")

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, cfg)
	authHandler := handler.NewAuthHandler(authService)

	jobCategoryRepo := repository.NewJobCategoryRepository(db)
	jobCategoryService := service.NewJobCategoryService(jobCategoryRepo)
	jobCategoryHandler := handler.NewJobCategoryHandler(jobCategoryService)

	tagGroupRepo := repository.NewTagGroupRepository(db)
	tagGroupService := service.NewTagGroupService(tagGroupRepo)
	tagGroupHandler := handler.NewTagGroupHandler(tagGroupService)

	tagRepo := repository.NewTagRepository(db)
	tagService := service.NewTagService(tagRepo, tagGroupRepo)
	tagHandler := handler.NewTagHandler(tagService)

	jobRepo := repository.NewJobRepository(db)
	jobService := service.NewJobService(jobRepo)
	jobHandler := handler.NewJobHandler(jobService)

	resumeRepo := repository.NewResumeRepository(db)
	resumeService := service.NewResumeService(resumeRepo)
	resumeHandler := handler.NewResumeHandler(resumeService)

	applicationRepo := repository.NewApplicationRepository(db)
	applicationService := service.NewApplicationService(applicationRepo)
	applicationHandler := handler.NewApplicationHandler(applicationService)

	//public 路由
	authRouter := api.Group("/auth")
	{
		authRouter.POST("/register", authHandler.Register)
		authRouter.POST("/login", authHandler.Login)
	}

	//private 路由 private 这个 group 挂了鉴权中间件，所以这两个接口需要带 token
	private := api.Group("")
	private.Use(middleware.AuthMiddleware(cfg))
	{
		private.GET("/job-categories", jobCategoryHandler.List)
		private.POST("/job-categories", jobCategoryHandler.Create)
		private.PUT("/job-categories/:id", jobCategoryHandler.Update)

		private.GET("/tag-groups", tagGroupHandler.List)
		private.POST("/tag-groups", tagGroupHandler.Create)

		private.GET("/tags", tagHandler.List)
		private.POST("/tags", tagHandler.Create)
		private.PUT("/tags/:id", tagHandler.Update)

		private.GET("/jobs", jobHandler.List)
		private.POST("/jobs", jobHandler.Create)
		private.PUT("/jobs/:id", jobHandler.Update)
		private.GET("/jobs/:id/tags", jobHandler.ListTags)
		private.PUT("/jobs/:id/tags", jobHandler.BindTags)
		private.GET("/jobs/:id/members", jobHandler.ListMembers)
		private.POST("/jobs/:id/members", jobHandler.AssignMember)

		private.POST("/resumes/upload", resumeHandler.Upload)

		private.POST("/applications", applicationHandler.Create)
	}

	return r
}
