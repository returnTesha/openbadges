// Package config provides application configuration loaded from environment variables.
// All settings fall back to sensible defaults for local development.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds every tuneable knob the application exposes.
type Config struct {
	// Server
	ServerPort string
	ServerURL  string // public base URL, e.g. "https://badge.example.com"

	// Issuer identity
	IssuerDID  string // e.g. "did:web:badge.example.com"
	IssuerName string // human-readable issuer name

	// Ed25519 signing key (hex-encoded 64-byte private key for local dev;
	// in production this would come from Vault).
	Ed25519PrivateKeyHex string

	// PostgreSQL (DSN or individual params)
	DatabaseURL  string
	PostgresHost string
	PostgresPort string
	PostgresDB   string
	PostgresUser string
	PostgresPass string

	// MinIO / S3
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string

	// HashiCorp Vault (key management)
	VaultAddr  string
	VaultToken string

	// Polygon (DID smart-contract fallback)
	PolygonRPC           string
	PolygonNetwork       string // e.g. "polygon-mainnet", "polygon-amoy"
	ContractAddress      string // legacy: used in badge JSON didFallback field
	KeyRegistryAddress   string // KeyRegistry contract address
	BadgeRegistryAddress string // BadgeRegistry contract address
	DeployerPrivateKey   string // Ethereum ECDSA private key for tx signing

	// Worker pool
	WorkerCount  int
	JobQueueSize int
}

// Load reads configuration from environment variables.
// Every field has a local-dev default so the binary can start without any env setup.
func Load() *Config {
	return &Config{
		ServerPort: envOrDefault("SERVER_PORT", "3000"),
		ServerURL:  envOrDefault("SERVER_URL", "http://localhost:3000"),

		IssuerDID:  envOrDefault("ISSUER_DID", "did:web:thebadge.kr"),
		IssuerName: envOrDefault("ISSUER_NAME", "다인리더스 The Badge Service"),

		Ed25519PrivateKeyHex: envOrDefault("ED25519_PRIVATE_KEY_HEX", ""),

		DatabaseURL:  envOrDefault("DATABASE_URL", ""),
		PostgresHost: envOrDefault("POSTGRES_HOST", "localhost"),
		PostgresPort: envOrDefault("POSTGRES_PORT", "5432"),
		PostgresDB:   envOrDefault("POSTGRES_DB", "openbadge"),
		PostgresUser: envOrDefault("POSTGRES_USER", "thebadge"),
		PostgresPass: envOrDefault("POSTGRES_PASSWORD", ""),

		MinIOEndpoint:  envOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: envOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: envOrDefault("MINIO_SECRET_KEY", "minioadmin"),

		VaultAddr:  envOrDefault("VAULT_ADDR", "http://localhost:8200"),
		VaultToken: envOrDefault("VAULT_TOKEN", "dev-only-token"),

		PolygonRPC:           envOrDefault("POLYGON_RPC", "http://localhost:8545"),
		PolygonNetwork:       envOrDefault("POLYGON_NETWORK", "polygon-mainnet"),
		ContractAddress:      envOrDefault("CONTRACT_ADDRESS", "0x0000000000000000000000000000000000000000"),
		KeyRegistryAddress:   envOrDefault("KEY_REGISTRY_ADDRESS", ""),
		BadgeRegistryAddress: envOrDefault("BADGE_REGISTRY_ADDRESS", ""),
		DeployerPrivateKey:   envOrDefault("DEPLOYER_PRIVATE_KEY", ""),

		WorkerCount:  envOrDefaultInt("WORKER_COUNT", 0), // 0 means runtime.NumCPU()*2 (resolved by worker pool)
		JobQueueSize: envOrDefaultInt("JOB_QUEUE_SIZE", 1000),
	}
}

// DSN returns the PostgreSQL connection string. If DATABASE_URL is set it is
// used directly; otherwise the DSN is built from the individual POSTGRES_* vars.
func (c *Config) DSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	sslmode := "disable"
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.PostgresUser, c.PostgresPass, c.PostgresHost, c.PostgresPort, c.PostgresDB, sslmode)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
