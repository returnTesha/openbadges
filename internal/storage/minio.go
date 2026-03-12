package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOConfig MinIO 연결 설정
type MinIOConfig struct {
	Endpoint       string
	AccessKeyID    string
	SecretAccessKey string
	UseSSL         bool
	BucketName     string
}

// MinIOClient MinIO 클라이언트 래퍼
type MinIOClient struct {
	client     *minio.Client
	bucketName string
	logger     *slog.Logger
}

// NewMinIOClient 새 MinIO 클라이언트 생성
func NewMinIOClient(cfg MinIOConfig, logger *slog.Logger) (*MinIOClient, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure:    cfg.UseSSL,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	mc := &MinIOClient{
		client:     client,
		bucketName: cfg.BucketName,
		logger:     logger.With("component", "minio"),
	}

	if err := mc.ensureBucket(context.Background()); err != nil {
		return nil, err
	}

	logger.Info("MinIO client initialized", "endpoint", cfg.Endpoint, "bucket", cfg.BucketName)
	return mc, nil
}

func (m *MinIOClient) ensureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucketName)
	if err != nil {
		return fmt.Errorf("check bucket exists: %w", err)
	}
	if !exists {
		if err := m.client.MakeBucket(ctx, m.bucketName, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		m.logger.Info("Created bucket", "bucket", m.bucketName)
	}
	return nil
}

// UploadFile 로컬 파일을 MinIO에 업로드
func (m *MinIOClient) UploadFile(ctx context.Context, localPath, objectPath string) (*UploadResult, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	contentType := getContentType(localPath)
	start := time.Now()

	info, err := m.client.PutObject(ctx, m.bucketName, objectPath, file, stat.Size(), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload to minio: %w", err)
	}

	return &UploadResult{
		Bucket:   m.bucketName,
		Key:      objectPath,
		Size:     info.Size,
		ETag:     info.ETag,
		Duration: time.Since(start),
	}, nil
}

// UploadReader io.Reader를 MinIO에 업로드
func (m *MinIOClient) UploadReader(ctx context.Context, reader io.Reader, objectPath string, size int64) (*UploadResult, error) {
	contentType := getContentType(objectPath)
	start := time.Now()

	info, err := m.client.PutObject(ctx, m.bucketName, objectPath, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload to minio: %w", err)
	}

	return &UploadResult{
		Bucket:   m.bucketName,
		Key:      objectPath,
		Size:     info.Size,
		ETag:     info.ETag,
		Duration: time.Since(start),
	}, nil
}

// GetObjectURL Pre-signed URL 생성
func (m *MinIOClient) GetObjectURL(ctx context.Context, objectPath string, expiry time.Duration) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, m.bucketName, objectPath, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("generate presigned url: %w", err)
	}
	return url.String(), nil
}

// DeleteObject 객체 삭제
func (m *MinIOClient) DeleteObject(ctx context.Context, objectPath string) error {
	return m.client.RemoveObject(ctx, m.bucketName, objectPath, minio.RemoveObjectOptions{})
}

// UploadResult 업로드 결과
type UploadResult struct {
	Bucket   string
	Key      string
	Size     int64
	ETag     string
	Duration time.Duration
}

func getContentType(path string) string {
	switch filepath.Ext(path) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
