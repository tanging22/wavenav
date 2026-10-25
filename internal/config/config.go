package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config 全部来自环境变量，NAS 上部署最省事。
type Config struct {
	Port          int
	DataDir       string
	Mode          string // dev | prod
	AdminUser     string
	AdminPassword string
	SessionDays   int
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func Load() *Config {
	cfg := &Config{
		Port:          envInt("WAVENAV_PORT", 8018),
		DataDir:       env("WAVENAV_DATA", "./data"),
		Mode:          env("WAVENAV_MODE", "prod"),
		AdminUser:     env("WAVENAV_ADMIN", "admin"),
		AdminPassword: env("WAVENAV_ADMIN_PASSWORD", "admin123"),
		SessionDays:   envInt("WAVENAV_SESSION_DAYS", 30),
	}
	if abs, err := filepath.Abs(cfg.DataDir); err == nil {
		cfg.DataDir = abs
	}
	return cfg
}

// UploadDir 素材目录与静态根分离，避免上传目录被当静态根遍历。
func (c *Config) UploadDir() string { return filepath.Join(c.DataDir, "uploads") }

// DBPath SQLite 文件
func (c *Config) DBPath() string { return filepath.Join(c.DataDir, "wavenav.db") }

func (c *Config) IsDev() bool { return c.Mode == "dev" }
