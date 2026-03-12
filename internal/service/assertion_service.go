package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"openbadge/internal/database"
	"openbadge/internal/model"
)

type AssertionService struct {
	db     *database.DB
	logger *slog.Logger
}

func NewAssertionService(db *database.DB, logger *slog.Logger) *AssertionService {
	return &AssertionService{
		db:     db,
		logger: logger.With("component", "assertion-service"),
	}
}

// Issue 배지 발급
func (s *AssertionService) Issue(ctx context.Context, a *model.Assertion) error {
	// BadgeClass 존재 확인
	badge, err := s.db.GetBadgeClass(ctx, a.BadgeClassID)
	if err != nil {
		return fmt.Errorf("check badge class: %w", err)
	}
	if badge == nil {
		return fmt.Errorf("badge class not found: %d", a.BadgeClassID)
	}

	// Recipient 존재 확인
	recipient, err := s.db.GetRecipient(ctx, a.RecipientID)
	if err != nil {
		return fmt.Errorf("check recipient: %w", err)
	}
	if recipient == nil {
		return fmt.Errorf("recipient not found: %d", a.RecipientID)
	}

	a.IssuerID = badge.IssuerID
	if a.IssuedOn.IsZero() {
		a.IssuedOn = time.Now()
	}

	return s.db.CreateAssertion(ctx, a)
}

// Get 발급 내역 조회
func (s *AssertionService) Get(ctx context.Context, id int64) (*model.Assertion, error) {
	return s.db.GetAssertion(ctx, id)
}

// List 발급 내역 목록
func (s *AssertionService) List(ctx context.Context, badgeClassID, recipientID int64, limit, offset int) ([]*model.Assertion, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.db.ListAssertions(ctx, badgeClassID, recipientID, limit, offset)
}

// Revoke 배지 취소
func (s *AssertionService) Revoke(ctx context.Context, id int64, reason string) error {
	return s.db.RevokeAssertion(ctx, id, reason)
}

// Verify 배지 검증
func (s *AssertionService) Verify(ctx context.Context, id int64) (*VerifyResult, error) {
	a, err := s.db.GetAssertion(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get assertion: %w", err)
	}
	if a == nil {
		return &VerifyResult{Valid: false, Reason: "assertion not found"}, nil
	}

	if a.Revoked {
		return &VerifyResult{Valid: false, Reason: "assertion has been revoked"}, nil
	}

	if a.ExpiresAt != nil && a.ExpiresAt.Before(time.Now()) {
		return &VerifyResult{Valid: false, Reason: "assertion has expired"}, nil
	}

	return &VerifyResult{Valid: true, Assertion: a}, nil
}

// VerifyResult 검증 결과
type VerifyResult struct {
	Valid     bool             `json:"valid"`
	Reason   string           `json:"reason,omitempty"`
	Assertion *model.Assertion `json:"assertion,omitempty"`
}
