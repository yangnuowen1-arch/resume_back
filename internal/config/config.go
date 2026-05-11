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
	}
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
