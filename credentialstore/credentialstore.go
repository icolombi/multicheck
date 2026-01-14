package credentialstore

import (
	"os"
	"strconv"
)

// Passwords

var (
	// RedisPassword string = "footop"
	RedisHost     string = getEnv("REDIS_HOST", "127.0.0.1")
	RedisPort     int    = getEnvInt("REDIS_PORT", 6379)
	RedisDatabase int    = 0
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
