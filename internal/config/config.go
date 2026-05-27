package config

import (
	"os"

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
	}
}

func (c *Config) R2Enabled() bool {
	return c.R2Endpoint != "" &&
		c.R2Bucket != "" &&
		c.R2AccessKeyID != "" &&
		c.R2SecretAccessKey != ""
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
