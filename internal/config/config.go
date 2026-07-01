package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                    string
	DatabaseURL             string
	DBMaxOpenConns          int
	DBMaxIdleConns          int
	DBConnMaxLifetime       time.Duration
	JWTSecret               string
	AccessTokenTTL          time.Duration
	RefreshTokenTTL         time.Duration
	CORSOrigins             []string
	APIRateLimitPerMinute   int
	LoginRateLimitPerMinute int
	RegisterRateLimitMinute int
	WSMaxConnectionsPerUser int
	ShutdownTimeout         time.Duration
	LogLevel                string
}

func Load() Config {
	return Config{
		Port:                    getEnv("PORT", "8080"),
		DatabaseURL:             getEnv("DATABASE_URL", defaultDatabaseURL()),
		DBMaxOpenConns:          getEnvInt("DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConns:          getEnvInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime:       getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		JWTSecret:               getEnv("JWT_SECRET", "change-me-in-production"),
		AccessTokenTTL:          getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTokenTTL:         getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		CORSOrigins:             getEnvCSV("CORS_ORIGINS", []string{"http://localhost:3000"}),
		APIRateLimitPerMinute:   getEnvInt("API_RATE_LIMIT_PER_MINUTE", 100),
		LoginRateLimitPerMinute: getEnvInt("LOGIN_RATE_LIMIT_PER_MINUTE", 5),
		RegisterRateLimitMinute: getEnvInt("REGISTER_RATE_LIMIT_PER_MINUTE", 3),
		WSMaxConnectionsPerUser: getEnvInt("WS_MAX_CONNECTIONS_PER_USER", 5),
		ShutdownTimeout:         getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		LogLevel:                getEnv("LOG_LEVEL", "INFO"),
	}
}

func defaultDatabaseURL() string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	name := getEnv("DB_NAME", "fifa_wc_2026")
	sslMode := getEnv("DB_SSLMODE", "disable")
	return "postgres://" + user + ":" + password + "@" + host + ":" + port + "/" + name + "?sslmode=" + sslMode
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvCSV(key string, fallback []string) []string {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
