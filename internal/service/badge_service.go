package service

import (
	"context"
	"fmt"
	"log/slog"

	"openbadge/internal/database"
	"openbadge/internal/model"
)

type BadgeService struct {
	db     *database.DB
	logger *slog.Logger
}

func NewBadgeService(db *database.DB, logger *slog.Logger) *BadgeService {
	return &BadgeService{
		db:     db,
		logger: logger.With("component", "badge-service"),
	}
}

func (s *BadgeService) Create(ctx context.Context, badge *model.BadgeClass) error {
	if badge.Name == "" {
		return fmt.Errorf("name is required")
	}
	if badge.IssuerID <= 0 {
		return fmt.Errorf("issuer_id is required")
	}

	// Issuer 존재 확인
	issuer, err := s.db.GetIssuer(ctx, badge.IssuerID)
	if err != nil {
		return fmt.Errorf("check issuer: %w", err)
	}
	if issuer == nil {
		return fmt.Errorf("issuer not found: %d", badge.IssuerID)
	}

	return s.db.CreateBadgeClass(ctx, badge)
}

func (s *BadgeService) Get(ctx context.Context, id int64) (*model.BadgeClass, error) {
	return s.db.GetBadgeClass(ctx, id)
}

func (s *BadgeService) List(ctx context.Context, issuerID int64, limit, offset int) ([]*model.BadgeClass, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.db.ListBadgeClasses(ctx, issuerID, limit, offset)
}

func (s *BadgeService) Update(ctx context.Context, badge *model.BadgeClass) error {
	return s.db.UpdateBadgeClass(ctx, badge)
}

func (s *BadgeService) Delete(ctx context.Context, id int64) error {
	return s.db.DeleteBadgeClass(ctx, id)
}
