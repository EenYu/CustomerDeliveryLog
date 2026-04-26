package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName            string
	ListenAddr         string
	BaseURL            string
	TokenSecret        string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	MySQLDSN           string
	StorageBackend     string
	UploadDir          string
	WebDir             string
	SeedAdminUsername  string
	SeedAdminPassword  string
	SeedAdminRealName  string
	MaxUploadSizeBytes int64
}

func Load() Config {
	wd, _ := os.Getwd()

	cfg := Config{
		AppName:            getEnv("APP_NAME", "现场交付档案中心"),
		ListenAddr:         getEnv("LISTEN_ADDR", ":8080"),
		BaseURL:            strings.TrimRight(getEnv("BASE_URL", "http://127.0.0.1:8080"), "/"),
		TokenSecret:        getEnv("TOKEN_SECRET", "change-me-in-production"),
		AccessTokenTTL:     getDurationEnv("ACCESS_TOKEN_TTL", 2*time.Hour),
		RefreshTokenTTL:    getDurationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		MySQLDSN:           getEnv("MYSQL_DSN", ""),
		StorageBackend:     getEnv("STORAGE_BACKEND", ""),
		UploadDir:          getEnv("UPLOAD_DIR", filepath.Join(wd, "uploads")),
		WebDir:             getEnv("WEB_DIR", filepath.Join(wd, "web")),
		SeedAdminUsername:  getEnv("SEED_ADMIN_USERNAME", "admin"),
		SeedAdminPassword:  getEnv("SEED_ADMIN_PASSWORD", "Admin@123456"),
		SeedAdminRealName:  getEnv("SEED_ADMIN_REAL_NAME", "系统管理员"),
		MaxUploadSizeBytes: getInt64Env("MAX_UPLOAD_SIZE_BYTES", 50*1024*1024),
	}

	if cfg.StorageBackend == "" {
		if cfg.MySQLDSN != "" {
			cfg.StorageBackend = "mysql"
		} else {
			cfg.StorageBackend = "memory"
		}
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return fallback
}

func getInt64Env(key string, fallback int64) int64 {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}
