package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port    string
	DevMode bool
	DatabaseConfig
	QuestionServiceURL string
}

type DatabaseConfig struct {
	DatabaseURL string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:    getEnv("PORT", "50052"),
		DevMode: getEnvBool("DEV_MODE", false),
		DatabaseConfig: DatabaseConfig{
			DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost/interverse?sslmode=disable"),
		},
		QuestionServiceURL: getEnv("QUESTION_SERVICE_URL", "localhost:50056"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
