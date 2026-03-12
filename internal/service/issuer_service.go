package service

import (
	"context"
	"fmt"
	"log/slog"

	"openbadge/internal/database"
	"openbadge/internal/model"
)

type IssuerService struct {
	db     *database.DB
	logger *slog.Logger
}

func NewIssuerService(db *database.DB, logger *slog.Logger) *IssuerService {
	return &IssuerService{
		db:     db,
		logger: logger.With("component", "issuer-service"),
	}
}

func (s *IssuerService) Create(ctx context.Context, issuer *model.Issuer) error {
	if issuer.Name == "" {
		return fmt.Errorf("name is required")
	}
	if issuer.URL == "" {
		return fmt.Errorf("url is required")
	}
	return s.db.CreateIssuer(ctx, issuer)
}

func (s *IssuerService) Get(ctx context.Context, id int64) (*model.Issuer, error) {
	return s.db.GetIssuer(ctx, id)
}

func (s *IssuerService) List(ctx context.Context, limit, offset int) ([]*model.Issuer, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.db.ListIssuers(ctx, limit, offset)
}

func (s *IssuerService) Update(ctx context.Context, issuer *model.Issuer) error {
	return s.db.UpdateIssuer(ctx, issuer)
}

func (s *IssuerService) Delete(ctx context.Context, id int64) error {
	return s.db.DeleteIssuer(ctx, id)
}
