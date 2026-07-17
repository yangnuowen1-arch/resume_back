package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	JWTSecret      string
	JWTExpireHours string

	R2Endpoint        string
	R2Bucket          string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2PublicBaseURL   string

	DifyBaseURL                 string
	DifyAPIKey                  string
	DifyUser                    string
	DifyResumeFileInputName     string
	DifyJobContextInputName     string
	DifyOutputLanguageInputName string
	DifyResultOutputName        string
	DifyScreeningWorkerCount    int

	GoogleOAuthClientID            string
	GoogleOAuthClientSecret        string
	GoogleOAuthRedirectURL         string
	MailboxOAuthSuccessRedirectURL string

	MailboxScanCronHour    int
	MailboxAllowedExt      string
	MailboxScanWorkerCount int
}

func LoadConfig() *Config {
	_ = godotenv.Load() //忽略 godotenv.Load() 返回的错误

	return &Config{
		AppPort: getEnv("APP_PORT", "8081"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "200156"),
		DBName:     getEnv("DB_NAME", "postgres"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:      getEnv("JWT_SECRET", "123456"),
		JWTExpireHours: getEnv("JWT_EXPIRE_HOURS", "2"),

		R2Endpoint:        getEnv("R2_ENDPOINT", ""),
		R2Bucket:          getEnv("R2_BUCKET", ""),
		R2AccessKeyID:     getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2PublicBaseURL:   getEnv("R2_PUBLIC_BASE_URL", ""),

		DifyBaseURL:                 getEnv("DIFY_BASE_URL", "https://api.dify.ai/v1"),
		DifyAPIKey:                  getEnv("DIFY_API_KEY", ""),
		DifyUser:                    getEnv("DIFY_USER", "resume_back"),
		DifyResumeFileInputName:     getEnv("DIFY_RESUME_FILE_INPUT_NAME", "resume_file"),
		DifyJobContextInputName:     getEnv("DIFY_JOB_CONTEXT_INPUT_NAME", "job_context"),
		DifyOutputLanguageInputName: getEnv("DIFY_OUTPUT_LANGUAGE_INPUT_NAME", "output_language"),
		DifyResultOutputName:        getEnv("DIFY_RESULT_OUTPUT_NAME", "screening_result"),
		DifyScreeningWorkerCount:    getEnvInt("DIFY_SCREENING_WORKER_COUNT", 3),

		GoogleOAuthClientID:            getEnv("GOOGLE_OAUTH_CLIENT_ID", ""),
		GoogleOAuthClientSecret:        getEnv("GOOGLE_OAUTH_CLIENT_SECRET", ""),
		GoogleOAuthRedirectURL:         getEnv("GOOGLE_OAUTH_REDIRECT_URL", ""),
		MailboxOAuthSuccessRedirectURL: getEnv("MAILBOX_OAUTH_SUCCESS_REDIRECT_URL", ""),

		MailboxScanCronHour:    getEnvInt("MAILBOX_SCAN_CRON_HOUR", 23),
		MailboxAllowedExt:      getEnv("MAILBOX_ALLOWED_EXT", ".pdf,.docx"),
		MailboxScanWorkerCount: getEnvInt("MAILBOX_SCAN_WORKER_COUNT", 2),
	}
}

func (c *Config) R2Enabled() bool {
	return c.R2Endpoint != "" &&
		c.R2Bucket != "" &&
		c.R2AccessKeyID != "" &&
		c.R2SecretAccessKey != ""
}

func (c *Config) DifyEnabled() bool {
	return c.DifyAPIKey != ""
}

func (c *Config) GoogleOAuthEnabled() bool {
	return c.GoogleOAuthClientID != "" &&
		c.GoogleOAuthClientSecret != "" &&
		c.GoogleOAuthRedirectURL != ""
}

func (c *Config) MailboxEnabled() bool {
	// 邮箱导入必须把原文件写入 R2，并把可直接访问的地址回填候选人列表。
	// 未配置公开域名时 R2Uploader 只能生成 r2:// 地址，浏览器无法打开，
	// 因此不启用邮箱入口，避免悄悄回退到本地 uploads/。
	return c.GoogleOAuthEnabled() && c.R2Enabled() && strings.TrimSpace(c.R2PublicBaseURL) != ""
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}
