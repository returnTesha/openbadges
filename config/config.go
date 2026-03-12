package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Database DatabaseConfig `toml:"database"`
	MinIO    MinIOConfig    `toml:"minio"`
	Auth     AuthConfig     `toml:"auth"`
}

// Server 설정
type ServerConfig struct {
	Port string `toml:"port"`
}

// Database 설정 (PostgreSQL)
type DatabaseConfig struct {
	Enabled  bool   `toml:"enabled"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DBName   string `toml:"dbname"`
	SSLMode  string `toml:"sslmode"`
}

// MinIO 설정 (배지 이미지 등 파일 저장)
type MinIOConfig struct {
	Enabled        bool   `toml:"enabled"`
	Endpoint       string `toml:"endpoint"`
	AccessKeyID    string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	UseSSL         bool   `toml:"use_ssl"`
	BucketName     string `toml:"bucket_name"`
}

// Auth 설정 (API 인증)
type AuthConfig struct {
	Enabled   bool   `toml:"enabled"`
	SecretKey string `toml:"secret_key"`
	Issuer    string `toml:"issuer"`
}

// Load TOML 파일 로드
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// 환경 변수 치환
	content := os.Expand(string(data), func(key string) string {
		key = strings.Trim(key, "{}")
		return os.Getenv(key)
	})

	var cfg Config
	if _, err := toml.Decode(content, &cfg); err != nil {
		return nil, fmt.Errorf("parse toml: %w", err)
	}

	return &cfg, nil
}
