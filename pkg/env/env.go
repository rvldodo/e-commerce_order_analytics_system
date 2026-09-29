package env

import (
	"os"
	"strconv"
	"strings"
	"time"

	"e-commerce_order_analytics_system/pkg/lib"

	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		panic("env: failed to load .env: " + err.Error())
	}
}

func GetString(key, fallback string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	return v
}

func GetInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	valInt, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}

	return valInt
}

func GetInt64(key string, fallback int64) int64 {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	valInt, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}

	return valInt
}

func GetBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	value = strings.ToLower(strings.TrimSpace(value))
	return lib.IsTruthy(value)
}

func GetStringArray(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func GetDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}

	return d
}
