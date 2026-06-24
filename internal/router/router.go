package router

import (
	"context"
	"log"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/yangnuowen1-arch/resume_back/internal/config"
	"github.com/yangnuowen1-arch/resume_back/internal/dify"
	"github.com/yangnuowen1-arch/resume_back/internal/handler"
	"github.com/yangnuowen1-arch/resume_back/internal/middleware"
	"github.com/yangnuowen1-arch/resume_back/internal/parser"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"

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
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

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

	candidateRepo := repository.NewCandidateRepository(db)

	resumeRepo := repository.NewResumeRepository(db)
	resumeUploader := storage.Uploader(storage.NewLocalUploader("uploads"))
	if cfg.R2Enabled() {
		r2Uploader, err := storage.NewR2Uploader(context.Background(), storage.R2Config{
			Endpoint:        cfg.R2Endpoint,
			Bucket:          cfg.R2Bucket,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			PublicBaseURL:   cfg.R2PublicBaseURL,
		})
		if err != nil {
			log.Fatalf("R2 初始化失败: %v", err)
		}
		resumeUploader = r2Uploader
	}

	var difyClient service.DifyResumeScreeningClient
	if cfg.DifyEnabled() {
		difyClient = dify.NewClient(dify.Config{
			BaseURL:                 cfg.DifyBaseURL,
			APIKey:                  cfg.DifyAPIKey,
			User:                    cfg.DifyUser,
			ResumeFileInputName:     cfg.DifyResumeFileInputName,
			JobContextInputName:     cfg.DifyJobContextInputName,
			OutputLanguageInputName: cfg.DifyOutputLanguageInputName,
			ResultOutputName:        cfg.DifyResultOutputName,
		})
	}

	applicationRepo := repository.NewApplicationRepository(db)
	screeningTaskRepo := repository.NewScreeningTaskRepository(db)
	screeningTaskService := service.NewScreeningTaskService(screeningTaskRepo, service.ScreeningTaskDependencies{
		JobRepo:         jobRepo,
		ResumeRepo:      resumeRepo,
		ApplicationRepo: applicationRepo,
		Uploader:        resumeUploader,
		DifyClient:      difyClient,
		DifyUser:        cfg.DifyUser,
		WorkerCount:     cfg.DifyScreeningWorkerCount,
	})
	resumeParser := parser.NewPlainTextParser()
	candidateService := service.NewCandidateService(candidateRepo, service.CandidateServiceDependencies{
		ScreeningTaskEnqueuer: screeningTaskService,
	})
	resumeService := service.NewResumeService(resumeRepo, candidateRepo, resumeUploader, resumeParser)
	candidateHandler := handler.NewCandidateHandler(candidateService, resumeUploader)
	screeningTaskHandler := handler.NewScreeningTaskHandler(screeningTaskService)
	resumeHandler := handler.NewResumeHandler(resumeService, resumeUploader)

	applicationService := service.NewApplicationService(applicationRepo, jobRepo, resumeRepo, candidateRepo)
	applicationHandler := handler.NewApplicationHandler(applicationService)

	operationLogRepo := repository.NewOperationLogRepository(db)
	operationLogService := service.NewOperationLogService(operationLogRepo)
	operationLogHandler := handler.NewOperationLogHandler(operationLogService)

	//public 路由
	authRouter := api.Group("/auth")
	{
		authRouter.POST("/register", authHandler.Register)
		authRouter.POST("/login", authHandler.Login)
	}

	//private 路由 private 这个 group 挂了鉴权中间件，所以这两个接口需要带 token
	private := api.Group("")
	private.Use(middleware.AuthMiddleware(cfg), middleware.OperationLogMiddleware(operationLogService))
	{
		private.GET("/job-categories", jobCategoryHandler.List)
		private.POST("/job-categories", jobCategoryHandler.Create)
		private.PUT("/job-categories/:id", jobCategoryHandler.Update)

		private.GET("/tag-groups", tagGroupHandler.List)
		private.POST("/tag-groups", tagGroupHandler.Create)
		private.PUT("/tag-groups/:id", tagGroupHandler.Update)

		private.GET("/tags/grouped", tagHandler.ListGrouped)
		private.GET("/tags", tagHandler.List)
		private.POST("/tags", tagHandler.Create)
		private.PUT("/tags/:id", tagHandler.Update)

		private.GET("/roles", userHandler.ListRoles)
		private.GET("/users", userHandler.List)
		private.POST("/users", userHandler.Create)
		private.GET("/users/:id", userHandler.Get)
		private.PUT("/users/:id", userHandler.Update)
		private.DELETE("/users/:id", userHandler.Delete)
		private.PUT("/users/:id/roles", userHandler.AssignRoles)

		private.GET("/jobs", jobHandler.List)
		private.POST("/jobs", jobHandler.Create)
		private.GET("/jobs/:id", jobHandler.Get)
		private.GET("/jobs/:id/screening-context", jobHandler.GetScreeningContext)
		private.PUT("/jobs/:id", jobHandler.Update)
		private.DELETE("/jobs/:id", jobHandler.Delete)
		private.GET("/jobs/:id/tags", jobHandler.ListTags)
		private.PUT("/jobs/:id/tags", jobHandler.BindTags)
		private.GET("/jobs/:id/members", jobHandler.ListMembers)
		private.POST("/jobs/:id/members", jobHandler.AssignMember)

		private.GET("/candidates", candidateHandler.List)
		private.POST("/candidates", candidateHandler.Create)
		private.POST("/candidates/batch-analyze", candidateHandler.BatchAnalyze)
		private.POST("/candidates/:id/resume", candidateHandler.UploadResume)
		private.PUT("/candidates/:id", candidateHandler.Update)
		private.GET("/candidate-statuses", candidateHandler.ListStatuses)
		private.POST("/screening-tasks/run", screeningTaskHandler.RunResumeScreening)
		private.GET("/screening-tasks", screeningTaskHandler.List)
		private.GET("/screening-tasks/:id", screeningTaskHandler.Detail)

		private.GET("/resumes", resumeHandler.List)
		private.POST("/resumes/upload", resumeHandler.Upload)
		private.POST("/resumes/:id/parse", resumeHandler.Parse)

		private.GET("/applications", applicationHandler.List)
		private.POST("/applications", applicationHandler.Create)

		private.GET("/operation-logs", operationLogHandler.List)
	}

	return r
}
