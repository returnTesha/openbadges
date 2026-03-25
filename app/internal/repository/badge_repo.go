// Package repository provides data-access interfaces and their implementations.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"the-badge/internal/model"
)

// BadgeRepository defines all data-access operations needed by the badge system.
type BadgeRepository interface {
	// ── Badges ──────────────────────────────────────────────────────────
	CreateBadge(ctx context.Context, badge *model.Badge) error
	GetBadge(ctx context.Context, id uuid.UUID) (*model.Badge, error)
	GetBadgeByCredentialID(ctx context.Context, credentialID string) (*model.Badge, error)
	ListBadges(ctx context.Context, params model.PaginationParams) ([]model.Badge, int64, error)
	UpdateBadgeStatus(ctx context.Context, id uuid.UUID, status string, reason string) error

	// ── Achievements ────────────────────────────────────────────────────
	GetAchievement(ctx context.Context, id uuid.UUID) (*model.Achievement, error)
	ListAchievements(ctx context.Context, issuerID uuid.UUID) ([]model.Achievement, error)

	// ── Issuers ─────────────────────────────────────────────────────────
	GetIssuer(ctx context.Context, id uuid.UUID) (*model.Issuer, error)
	GetIssuerByDID(ctx context.Context, did string) (*model.Issuer, error)

	// ── Verification logs ───────────────────────────────────────────────
	CreateVerificationLog(ctx context.Context, log *model.VerificationLog) error
	ListVerificationLogs(ctx context.Context, params model.PaginationParams) ([]model.VerificationLog, int64, error)

	// ── Key history ─────────────────────────────────────────────────────
	GetActiveKey(ctx context.Context, issuerID uuid.UUID) (*model.KeyHistory, error)
	GetKeyAtTime(ctx context.Context, issuerID uuid.UUID, at time.Time) (*model.KeyHistory, error)

	// ── Sequences ───────────────────────────────────────────────────────
	NextCredentialSequence(ctx context.Context) (int64, error)
}

// ---------------------------------------------------------------------------
// PostgresRepository
// ---------------------------------------------------------------------------

// PostgresRepository implements BadgeRepository against PostgreSQL using pgx.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository returns a new repository instance.
// The caller is responsible for passing a live connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ── Badges ──────────────────────────────────────────────────────────────────

// CreateBadge inserts a new badge row and populates badge.ID and badge.IssuedAt.
func (r *PostgresRepository) CreateBadge(ctx context.Context, badge *model.Badge) error {
	query := `
		INSERT INTO badges (
			achievement_id, issuer_id, recipient_name, recipient_email,
			recipient_did, credential_id, credential_json, proof_value, minio_key,
			blockchain_tx_hash, blockchain_hash,
			university_code, program_id, student_id,
			status, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, issued_at`

	return r.pool.QueryRow(ctx, query,
		badge.AchievementID,
		badge.IssuerID,
		badge.RecipientName,
		badge.RecipientEmail,
		badge.RecipientDID,
		badge.CredentialID,
		badge.CredentialJSON,
		badge.ProofValue,
		badge.MinIOKey,
		badge.BlockchainTxHash,
		badge.BlockchainHash,
		badge.UniversityCode,
		badge.ProgramID,
		badge.StudentID,
		badge.Status,
		badge.ExpiresAt,
	).Scan(&badge.ID, &badge.IssuedAt)
}

// GetBadge retrieves a single badge by primary key.
func (r *PostgresRepository) GetBadge(ctx context.Context, id uuid.UUID) (*model.Badge, error) {
	query := `
		SELECT id, achievement_id, issuer_id, recipient_name, recipient_email,
		       COALESCE(recipient_did, ''), COALESCE(credential_id, ''),
		       credential_json, proof_value, COALESCE(minio_key, ''),
		       COALESCE(blockchain_tx_hash, ''), blockchain_hash,
		       COALESCE(university_code, ''), COALESCE(program_id, ''), COALESCE(student_id, ''),
		       status, issued_at, expires_at, revoked_at, COALESCE(revocation_reason, '')
		FROM badges WHERE id = $1`

	var b model.Badge
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&b.ID, &b.AchievementID, &b.IssuerID, &b.RecipientName, &b.RecipientEmail,
		&b.RecipientDID, &b.CredentialID,
		&b.CredentialJSON, &b.ProofValue, &b.MinIOKey,
		&b.BlockchainTxHash, &b.BlockchainHash,
		&b.UniversityCode, &b.ProgramID, &b.StudentID,
		&b.Status, &b.IssuedAt, &b.ExpiresAt, &b.RevokedAt, &b.RevocationReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

// GetBadgeByCredentialID retrieves a single badge by credential_id.
func (r *PostgresRepository) GetBadgeByCredentialID(ctx context.Context, credentialID string) (*model.Badge, error) {
	query := `
		SELECT id, achievement_id, issuer_id, recipient_name, recipient_email,
		       COALESCE(recipient_did, ''), COALESCE(credential_id, ''),
		       credential_json, proof_value, COALESCE(minio_key, ''),
		       COALESCE(blockchain_tx_hash, ''), blockchain_hash,
		       COALESCE(university_code, ''), COALESCE(program_id, ''), COALESCE(student_id, ''),
		       status, issued_at, expires_at, revoked_at, COALESCE(revocation_reason, '')
		FROM badges WHERE credential_id = $1`

	var b model.Badge
	err := r.pool.QueryRow(ctx, query, credentialID).Scan(
		&b.ID, &b.AchievementID, &b.IssuerID, &b.RecipientName, &b.RecipientEmail,
		&b.RecipientDID, &b.CredentialID,
		&b.CredentialJSON, &b.ProofValue, &b.MinIOKey,
		&b.BlockchainTxHash, &b.BlockchainHash,
		&b.UniversityCode, &b.ProgramID, &b.StudentID,
		&b.Status, &b.IssuedAt, &b.ExpiresAt, &b.RevokedAt, &b.RevocationReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

// ListBadges returns a paginated, optionally filtered list of badges and the
// total count of matching rows.
func (r *PostgresRepository) ListBadges(ctx context.Context, params model.PaginationParams) ([]model.Badge, int64, error) {
	params.Normalize()

	// Build dynamic WHERE clause.
	var conditions []string
	var args []interface{}
	argIdx := 1

	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, params.Status)
		argIdx++
	}
	if params.RecipientEmail != "" {
		conditions = append(conditions, fmt.Sprintf("recipient_email = $%d", argIdx))
		args = append(args, params.RecipientEmail)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query.
	countQuery := "SELECT count(*) FROM badges " + where
	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data query.
	dataQuery := fmt.Sprintf(`
		SELECT id, achievement_id, issuer_id, recipient_name, recipient_email,
		       COALESCE(recipient_did, ''), COALESCE(credential_id, ''),
		       credential_json, proof_value, COALESCE(minio_key, ''),
		       COALESCE(blockchain_tx_hash, ''), blockchain_hash,
		       COALESCE(university_code, ''), COALESCE(program_id, ''), COALESCE(student_id, ''),
		       status, issued_at, expires_at, revoked_at, COALESCE(revocation_reason, '')
		FROM badges %s
		ORDER BY issued_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, params.PerPage, params.Offset())

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var badges []model.Badge
	for rows.Next() {
		var b model.Badge
		if err := rows.Scan(
			&b.ID, &b.AchievementID, &b.IssuerID, &b.RecipientName, &b.RecipientEmail,
			&b.RecipientDID, &b.CredentialID,
			&b.CredentialJSON, &b.ProofValue, &b.MinIOKey,
			&b.BlockchainTxHash, &b.BlockchainHash,
			&b.UniversityCode, &b.ProgramID, &b.StudentID,
			&b.Status, &b.IssuedAt, &b.ExpiresAt, &b.RevokedAt, &b.RevocationReason,
		); err != nil {
			return nil, 0, err
		}
		badges = append(badges, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return badges, total, nil
}

// UpdateBadgeStatus sets the status of a badge (e.g. 'revoked') and records
// the revocation reason and timestamp.
func (r *PostgresRepository) UpdateBadgeStatus(ctx context.Context, id uuid.UUID, status string, reason string) error {
	var query string
	if status == "revoked" {
		query = `
			UPDATE badges
			SET status = $2, revoked_at = now(), revocation_reason = $3
			WHERE id = $1`
	} else {
		query = `
			UPDATE badges
			SET status = $2, revoked_at = NULL, revocation_reason = ''
			WHERE id = $1`
	}

	tag, err := r.pool.Exec(ctx, query, id, status, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("badge not found: %s", id)
	}
	return nil
}

// ── Achievements ────────────────────────────────────────────────────────────

// GetAchievement retrieves a single achievement by primary key.
func (r *PostgresRepository) GetAchievement(ctx context.Context, id uuid.UUID) (*model.Achievement, error) {
	query := `
		SELECT id, issuer_id, name, description, criteria_narrative,
		       COALESCE(image_base64, ''), COALESCE(image_url, ''),
		       COALESCE(tags, '{}'), created_at, updated_at
		FROM achievements WHERE id = $1`

	var a model.Achievement
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.IssuerID, &a.Name, &a.Description, &a.CriteriaNarrative,
		&a.ImageBase64, &a.ImageURL, &a.Tags, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// ListAchievements returns all achievements belonging to a given issuer.
func (r *PostgresRepository) ListAchievements(ctx context.Context, issuerID uuid.UUID) ([]model.Achievement, error) {
	query := `
		SELECT id, issuer_id, name, description, criteria_narrative,
		       COALESCE(image_base64, ''), COALESCE(image_url, ''),
		       COALESCE(tags, '{}'), created_at, updated_at
		FROM achievements WHERE issuer_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, issuerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var achievements []model.Achievement
	for rows.Next() {
		var a model.Achievement
		if err := rows.Scan(
			&a.ID, &a.IssuerID, &a.Name, &a.Description, &a.CriteriaNarrative,
			&a.ImageBase64, &a.ImageURL, &a.Tags, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		achievements = append(achievements, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return achievements, nil
}

// ── Issuers ─────────────────────────────────────────────────────────────────

// GetIssuer retrieves an issuer by primary key.
func (r *PostgresRepository) GetIssuer(ctx context.Context, id uuid.UUID) (*model.Issuer, error) {
	query := `
		SELECT id, did, name, COALESCE(url, ''), COALESCE(logo_base64, ''),
		       public_key, created_at, updated_at
		FROM issuers WHERE id = $1`

	var i model.Issuer
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&i.ID, &i.DID, &i.Name, &i.URL, &i.LogoBase64, &i.PublicKey,
		&i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

// GetIssuerByDID retrieves an issuer by its DID string (unique).
func (r *PostgresRepository) GetIssuerByDID(ctx context.Context, did string) (*model.Issuer, error) {
	query := `
		SELECT id, did, name, COALESCE(url, ''), COALESCE(logo_base64, ''),
		       public_key, created_at, updated_at
		FROM issuers WHERE did = $1`

	var i model.Issuer
	err := r.pool.QueryRow(ctx, query, did).Scan(
		&i.ID, &i.DID, &i.Name, &i.URL, &i.LogoBase64, &i.PublicKey,
		&i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

// ── Verification logs ───────────────────────────────────────────────────────

// CreateVerificationLog inserts a new verification log entry.
func (r *PostgresRepository) CreateVerificationLog(ctx context.Context, log *model.VerificationLog) error {
	query := `
		INSERT INTO verification_logs (badge_id, credential_id, issuer_did, verifier_ip, result, failure_reason, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, verified_at`

	return r.pool.QueryRow(ctx, query,
		log.BadgeID,
		log.CredentialID,
		log.IssuerDID,
		log.VerifierIP,
		log.Result,
		log.FailureReason,
		log.Detail,
	).Scan(&log.ID, &log.VerifiedAt)
}

// ListVerificationLogs returns a paginated list of verification log entries
// and the total count.
func (r *PostgresRepository) ListVerificationLogs(ctx context.Context, params model.PaginationParams) ([]model.VerificationLog, int64, error) {
	params.Normalize()

	// Count query.
	var total int64
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM verification_logs").Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data query.
	query := `
		SELECT id, badge_id, COALESCE(credential_id, ''), COALESCE(issuer_did, ''),
		       COALESCE(verifier_ip, ''), result, COALESCE(failure_reason, ''), detail, verified_at
		FROM verification_logs
		ORDER BY verified_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, params.PerPage, params.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []model.VerificationLog
	for rows.Next() {
		var l model.VerificationLog
		if err := rows.Scan(
			&l.ID, &l.BadgeID, &l.CredentialID, &l.IssuerDID, &l.VerifierIP, &l.Result, &l.FailureReason, &l.Detail, &l.VerifiedAt,
		); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// ── Key history ─────────────────────────────────────────────────────────────

// GetActiveKey returns the currently active key for an issuer (revoked_at IS NULL).
func (r *PostgresRepository) GetActiveKey(ctx context.Context, issuerID uuid.UUID) (*model.KeyHistory, error) {
	query := `
		SELECT id, issuer_id, public_key, activated_at, revoked_at, COALESCE(tx_hash, '')
		FROM key_history
		WHERE issuer_id = $1 AND revoked_at IS NULL
		ORDER BY activated_at DESC
		LIMIT 1`

	var k model.KeyHistory
	err := r.pool.QueryRow(ctx, query, issuerID).Scan(
		&k.ID, &k.IssuerID, &k.PublicKey, &k.ActivatedAt, &k.RevokedAt, &k.TxHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &k, nil
}

// GetKeyAtTime returns the key that was active for an issuer at a specific
// point in time. This is essential for verifying badges issued with a
// previously-rotated key.
func (r *PostgresRepository) GetKeyAtTime(ctx context.Context, issuerID uuid.UUID, at time.Time) (*model.KeyHistory, error) {
	query := `
		SELECT id, issuer_id, public_key, activated_at, revoked_at, COALESCE(tx_hash, '')
		FROM key_history
		WHERE issuer_id = $1
		  AND activated_at <= $2
		  AND (revoked_at IS NULL OR revoked_at > $2)
		ORDER BY activated_at DESC
		LIMIT 1`

	var k model.KeyHistory
	err := r.pool.QueryRow(ctx, query, issuerID, at).Scan(
		&k.ID, &k.IssuerID, &k.PublicKey, &k.ActivatedAt, &k.RevokedAt, &k.TxHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &k, nil
}

// ── Sequences ───────────────────────────────────────────────────────────────

// NextCredentialSequence returns the next value from badge_credential_seq.
// The caller combines it with the year: fmt.Sprintf("%d%d", year, seq).
func (r *PostgresRepository) NextCredentialSequence(ctx context.Context) (int64, error) {
	var seq int64
	err := r.pool.QueryRow(ctx, "SELECT nextval('badge_credential_seq')").Scan(&seq)
	return seq, err
}
