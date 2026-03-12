package main

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"openbadge/config"
	handler "openbadge/internal/adapter/http"
	"openbadge/internal/database"
	"openbadge/internal/middleware"
	"openbadge/internal/service"
	"openbadge/internal/storage"

	"github.com/gofiber/fiber/v2"
)

func main() {
	logger := setupLogger()
	logger.Info("Starting OpenBadge Server")

	if err := loadEnv(".env"); err != nil {
		logger.Warn("No .env file loaded", "error", err)
	}

	cfg, err := config.Load("config/config.toml")
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// MinIO 클라이언트 초기화 (선택적)
	var minioClient *storage.MinIOClient
	if cfg.MinIO.Enabled {
		mc, err := setupMinIO(cfg, logger)
		if err != nil {
			logger.Warn("MinIO setup failed, continuing without storage", "error", err)
		} else {
			minioClient = mc
		}
	}
	_ = minioClient // 추후 이미지 업로드 시 사용

	// 데이터베이스 초기화
	var db *database.DB
	if cfg.Database.Enabled {
		dbConn, err := setupDatabase(cfg, logger)
		if err != nil {
			logger.Error("Database setup failed", "error", err)
			os.Exit(1)
		}
		db = dbConn
		defer db.Close()
	} else {
		logger.Error("Database is required for OpenBadge")
		os.Exit(1)
	}

	// 서비스 초기화
	issuerSvc := service.NewIssuerService(db, logger)
	badgeSvc := service.NewBadgeService(db, logger)
	assertionSvc := service.NewAssertionService(db, logger)

	// 시그널 처리
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Fiber HTTP 서버
	app := fiber.New(fiber.Config{
		AppName: "OpenBadge",
	})

	// 인증 미들웨어 (선택적)
	if cfg.Auth.Enabled {
		app.Use(middleware.NewAuth(middleware.AuthConfig{
			SecretKey: cfg.Auth.SecretKey,
			Skip:      []string{"/health", "/api/v1/assertions/*/verify"},
		}))
	}

	// 라우트 등록
	httpHandler := handler.NewHttpHandler(issuerSvc, badgeSvc, assertionSvc, logger)
	httpHandler.RegisterRoutes(app)

	port := cfg.Server.Port
	if port == "" {
		port = ":8080"
	}

	// 서버 시작 (별도 고루틴)
	go func() {
		logger.Info("HTTP server starting", "port", port)
		if err := app.Listen(port); err != nil {
			logger.Error("Server failed", "error", err)
		}
	}()

	// 시그널 대기
	<-ctx.Done()
	logger.Info("Shutting down...")
	app.Shutdown()
	logger.Info("Application stopped")
}

// --- 헬퍼 함수들 ---

func setupLogger() *slog.Logger {
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h)
}

func loadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func setupMinIO(cfg *config.Config, logger *slog.Logger) (*storage.MinIOClient, error) {
	return storage.NewMinIOClient(storage.MinIOConfig{
		Endpoint:       cfg.MinIO.Endpoint,
		AccessKeyID:    cfg.MinIO.AccessKeyID,
		SecretAccessKey: cfg.MinIO.SecretAccessKey,
		UseSSL:         cfg.MinIO.UseSSL,
		BucketName:     cfg.MinIO.BucketName,
	}, logger)
}

func setupDatabase(cfg *config.Config, logger *slog.Logger) (*database.DB, error) {
	return database.NewDB(database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}, logger)
}
