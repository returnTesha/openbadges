// Package model defines the core domain types for the badge system.
// Structs map directly to PostgreSQL tables defined in migrations/001_initial_schema.sql.
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Database entities
// ---------------------------------------------------------------------------

// Issuer represents a badge-issuing organisation (issuers table).
type Issuer struct {
	ID         uuid.UUID `json:"id"         db:"id"`
	DID        string    `json:"did"        db:"did"`
	Name       string    `json:"name"       db:"name"`
	URL        string    `json:"url"        db:"url"`
	LogoBase64 string    `json:"logo_base64,omitempty" db:"logo_base64"`
	PublicKey  []byte    `json:"-"          db:"public_key"` // never expose raw key in JSON
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// Achievement defines a badge template / badge class (achievements table).
type Achievement struct {
	ID                uuid.UUID `json:"id"                 db:"id"`
	IssuerID          uuid.UUID `json:"issuer_id"          db:"issuer_id"`
	Name              string    `json:"name"               db:"name"`
	Description       string    `json:"description"        db:"description"`
	CriteriaNarrative string    `json:"criteria_narrative" db:"criteria_narrative"`
	ImageBase64       string    `json:"image_base64,omitempty" db:"image_base64"`
	ImageURL          string    `json:"image_url,omitempty"    db:"image_url"`
	Tags              []string  `json:"tags,omitempty"     db:"tags"`
	CreatedAt         time.Time `json:"created_at"         db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"         db:"updated_at"`
}

// Badge represents an issued Open Badges 3.0 credential (badges table).
type Badge struct {
	ID               uuid.UUID       `json:"id"                db:"id"`
	AchievementID    *uuid.UUID      `json:"achievement_id,omitempty" db:"achievement_id"`
	IssuerID         uuid.UUID       `json:"issuer_id"         db:"issuer_id"`
	RecipientName    string          `json:"recipient_name"    db:"recipient_name"`
	RecipientEmail   string          `json:"recipient_email"   db:"recipient_email"`
	RecipientDID     string          `json:"recipient_did,omitempty" db:"recipient_did"`
	CredentialID     string          `json:"credential_id,omitempty" db:"credential_id"`
	CredentialJSON   json.RawMessage `json:"credential_json"   db:"credential_json"`
	ProofValue       string          `json:"proof_value"       db:"proof_value"`
	MinIOKey         string          `json:"minio_key,omitempty" db:"minio_key"`
	BlockchainTxHash string          `json:"blockchain_tx_hash,omitempty" db:"blockchain_tx_hash"`
	BlockchainHash   []byte          `json:"-"                 db:"blockchain_hash"`
	UniversityCode   string          `json:"university_code,omitempty" db:"university_code"`
	ProgramID        string          `json:"program_id,omitempty"      db:"program_id"`
	StudentID        string          `json:"student_id,omitempty"      db:"student_id"`
	Status           string          `json:"status"            db:"status"`
	IssuedAt         time.Time       `json:"issued_at"         db:"issued_at"`
	ExpiresAt        *time.Time      `json:"expires_at,omitempty"  db:"expires_at"`
	RevokedAt        *time.Time      `json:"revoked_at,omitempty"  db:"revoked_at"`
	RevocationReason string          `json:"revocation_reason,omitempty" db:"revocation_reason"`
}

// VerificationLog records a single verification attempt (verification_logs table).
type VerificationLog struct {
	ID            uuid.UUID        `json:"id"             db:"id"`
	BadgeID       *uuid.UUID       `json:"badge_id,omitempty" db:"badge_id"`
	CredentialID  string           `json:"credential_id,omitempty" db:"credential_id"`
	IssuerDID     string           `json:"issuer_did,omitempty" db:"issuer_did"`
	VerifierIP    string           `json:"verifier_ip"    db:"verifier_ip"`
	Result        string           `json:"result"         db:"result"`
	FailureReason string           `json:"failure_reason,omitempty" db:"failure_reason"`
	Detail        json.RawMessage  `json:"detail,omitempty" db:"detail"`
	VerifiedAt    time.Time        `json:"verified_at"    db:"verified_at"`
}

// KeyHistory tracks Ed25519 key rotations for an issuer (key_history table).
type KeyHistory struct {
	ID          uuid.UUID  `json:"id"           db:"id"`
	IssuerID    uuid.UUID  `json:"issuer_id"    db:"issuer_id"`
	PublicKey   []byte     `json:"-"            db:"public_key"`
	ActivatedAt time.Time  `json:"activated_at" db:"activated_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	TxHash      string     `json:"tx_hash,omitempty"    db:"tx_hash"`
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// IssueBadgeRequest is the payload for POST /api/v1/badges.
type IssueBadgeRequest struct {
	AchievementID  string `json:"achievement_id"`
	RecipientName  string `json:"recipient_name"`
	RecipientEmail string `json:"recipient_email"`
}

// VerifyBadgeRequest is the payload for POST /api/v1/badges/verify.
// The caller provides either a raw credential JSON or a badge ID to look up.
type VerifyBadgeRequest struct {
	CredentialJSON json.RawMessage `json:"credential_json,omitempty"`
	BadgeID        string          `json:"badge_id,omitempty"`
}

// ---------------------------------------------------------------------------
// Response DTOs
// ---------------------------------------------------------------------------

// BadgeResponse is the public representation returned by badge endpoints.
type BadgeResponse struct {
	ID               uuid.UUID       `json:"id"`
	AchievementID    *uuid.UUID      `json:"achievement_id,omitempty"`
	IssuerID         uuid.UUID       `json:"issuer_id"`
	RecipientName    string          `json:"recipient_name"`
	RecipientEmail   string          `json:"recipient_email"`
	RecipientDID     string          `json:"recipient_did,omitempty"`
	CredentialJSON   json.RawMessage `json:"credential_json"`
	Status           string          `json:"status"`
	IssuedAt         time.Time       `json:"issued_at"`
	ExpiresAt        *time.Time      `json:"expires_at,omitempty"`
	RevokedAt        *time.Time      `json:"revoked_at,omitempty"`
	RevocationReason string          `json:"revocation_reason,omitempty"`
}

// BadgeResponseFromBadge converts a Badge entity to its API representation.
func BadgeResponseFromBadge(b *Badge) BadgeResponse {
	return BadgeResponse{
		ID:               b.ID,
		AchievementID:    b.AchievementID,
		IssuerID:         b.IssuerID,
		RecipientName:    b.RecipientName,
		RecipientEmail:   b.RecipientEmail,
		RecipientDID:     b.RecipientDID,
		CredentialJSON:   b.CredentialJSON,
		Status:           b.Status,
		IssuedAt:         b.IssuedAt,
		ExpiresAt:        b.ExpiresAt,
		RevokedAt:        b.RevokedAt,
		RevocationReason: b.RevocationReason,
	}
}

// HistoryResponse wraps a paginated list of items with total count metadata.
type HistoryResponse struct {
	Items      interface{} `json:"items"`
	TotalCount int64       `json:"total_count"`
	Page       int         `json:"page"`
	PerPage    int         `json:"per_page"`
}

// ---------------------------------------------------------------------------
// Pagination helpers
// ---------------------------------------------------------------------------

const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// PaginationParams holds validated pagination query parameters.
type PaginationParams struct {
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	// Optional filters — handlers set these before passing to the repository.
	Status         string `json:"status,omitempty"`
	RecipientEmail string `json:"recipient_email,omitempty"`
}

// Offset returns the SQL OFFSET derived from page and per_page.
func (p PaginationParams) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// Normalize clamps page/per_page to safe bounds.
func (p *PaginationParams) Normalize() {
	if p.Page < 1 {
		p.Page = DefaultPage
	}
	if p.PerPage < 1 {
		p.PerPage = DefaultPerPage
	}
	if p.PerPage > MaxPerPage {
		p.PerPage = MaxPerPage
	}
}
