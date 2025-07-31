package config

import (
	"os"
)

type Config struct {
	ServerPort string
	TargetURL  string
	LogDir     string
}

func Load() *Config {
	return &Config{
		ServerPort: getEnv("SERVER_PORT", ":8080"),
		TargetURL:  getEnv("TARGET_URL", "https://api.aicodewith.com"),
		LogDir:     getEnv("LOG_DIR", "./logs"),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
