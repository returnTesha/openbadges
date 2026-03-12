package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"openbadge/internal/model"

	_ "github.com/lib/pq"
)

// Config DB 연결 설정
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DB 데이터베이스 클라이언트
type DB struct {
	conn   *sql.DB
	logger *slog.Logger
}

// NewDB 새 DB 연결 생성
func NewDB(cfg Config, logger *slog.Logger) (*DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	logger.Info("Database connected", "host", cfg.Host, "dbname", cfg.DBName)

	return &DB{conn: conn, logger: logger.With("component", "database")}, nil
}

// Conn 원시 DB 커넥션 반환
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Close DB 연결 종료
func (db *DB) Close() {
	db.conn.Close()
}

// ==========================================
// Issuer CRUD
// ==========================================

func (db *DB) CreateIssuer(ctx context.Context, issuer *model.Issuer) error {
	query := `INSERT INTO issuers (name, url, email, description, image_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`
	return db.conn.QueryRowContext(ctx, query,
		issuer.Name, issuer.URL, issuer.Email, issuer.Description, issuer.ImageURL,
	).Scan(&issuer.ID, &issuer.CreatedAt, &issuer.UpdatedAt)
}

func (db *DB) GetIssuer(ctx context.Context, id int64) (*model.Issuer, error) {
	query := `SELECT id, name, url, email, description, image_url, created_at, updated_at
		FROM issuers WHERE id = $1`
	var i model.Issuer
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&i.ID, &i.Name, &i.URL, &i.Email, &i.Description, &i.ImageURL,
		&i.CreatedAt, &i.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (db *DB) ListIssuers(ctx context.Context, limit, offset int) ([]*model.Issuer, error) {
	query := `SELECT id, name, url, email, description, image_url, created_at, updated_at
		FROM issuers ORDER BY id DESC LIMIT $1 OFFSET $2`
	rows, err := db.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issuers []*model.Issuer
	for rows.Next() {
		var i model.Issuer
		if err := rows.Scan(&i.ID, &i.Name, &i.URL, &i.Email, &i.Description, &i.ImageURL,
			&i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		issuers = append(issuers, &i)
	}
	return issuers, nil
}

func (db *DB) UpdateIssuer(ctx context.Context, issuer *model.Issuer) error {
	query := `UPDATE issuers SET name=$1, url=$2, email=$3, description=$4, image_url=$5, updated_at=NOW()
		WHERE id=$6 RETURNING updated_at`
	return db.conn.QueryRowContext(ctx, query,
		issuer.Name, issuer.URL, issuer.Email, issuer.Description, issuer.ImageURL, issuer.ID,
	).Scan(&issuer.UpdatedAt)
}

func (db *DB) DeleteIssuer(ctx context.Context, id int64) error {
	result, err := db.conn.ExecContext(ctx, `DELETE FROM issuers WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("issuer not found: %d", id)
	}
	return nil
}

// ==========================================
// BadgeClass CRUD
// ==========================================

func (db *DB) CreateBadgeClass(ctx context.Context, b *model.BadgeClass) error {
	query := `INSERT INTO badge_classes (issuer_id, name, description, image_url, criteria_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`
	return db.conn.QueryRowContext(ctx, query,
		b.IssuerID, b.Name, b.Description, b.ImageURL, b.CriteriaURL,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
}

func (db *DB) GetBadgeClass(ctx context.Context, id int64) (*model.BadgeClass, error) {
	query := `SELECT id, issuer_id, name, description, image_url, criteria_url, created_at, updated_at
		FROM badge_classes WHERE id = $1`
	var b model.BadgeClass
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.IssuerID, &b.Name, &b.Description, &b.ImageURL, &b.CriteriaURL,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (db *DB) ListBadgeClasses(ctx context.Context, issuerID int64, limit, offset int) ([]*model.BadgeClass, error) {
	var query string
	var args []interface{}

	if issuerID > 0 {
		query = `SELECT id, issuer_id, name, description, image_url, criteria_url, created_at, updated_at
			FROM badge_classes WHERE issuer_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3`
		args = []interface{}{issuerID, limit, offset}
	} else {
		query = `SELECT id, issuer_id, name, description, image_url, criteria_url, created_at, updated_at
			FROM badge_classes ORDER BY id DESC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var badges []*model.BadgeClass
	for rows.Next() {
		var b model.BadgeClass
		if err := rows.Scan(&b.ID, &b.IssuerID, &b.Name, &b.Description, &b.ImageURL,
			&b.CriteriaURL, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		badges = append(badges, &b)
	}
	return badges, nil
}

func (db *DB) UpdateBadgeClass(ctx context.Context, b *model.BadgeClass) error {
	query := `UPDATE badge_classes SET name=$1, description=$2, image_url=$3, criteria_url=$4, updated_at=NOW()
		WHERE id=$5 RETURNING updated_at`
	return db.conn.QueryRowContext(ctx, query,
		b.Name, b.Description, b.ImageURL, b.CriteriaURL, b.ID,
	).Scan(&b.UpdatedAt)
}

func (db *DB) DeleteBadgeClass(ctx context.Context, id int64) error {
	result, err := db.conn.ExecContext(ctx, `DELETE FROM badge_classes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("badge class not found: %d", id)
	}
	return nil
}

// ==========================================
// Assertion CRUD
// ==========================================

func (db *DB) CreateAssertion(ctx context.Context, a *model.Assertion) error {
	query := `INSERT INTO assertions (badge_class_id, issuer_id, recipient_id, issued_on, expires_at, evidence)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	return db.conn.QueryRowContext(ctx, query,
		a.BadgeClassID, a.IssuerID, a.RecipientID, a.IssuedOn, a.ExpiresAt, a.Evidence,
	).Scan(&a.ID, &a.CreatedAt)
}

func (db *DB) GetAssertion(ctx context.Context, id int64) (*model.Assertion, error) {
	query := `SELECT id, badge_class_id, issuer_id, recipient_id, issued_on, expires_at,
		revoked, revoked_at, revocation_reason, evidence, created_at
		FROM assertions WHERE id = $1`
	var a model.Assertion
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.BadgeClassID, &a.IssuerID, &a.RecipientID, &a.IssuedOn, &a.ExpiresAt,
		&a.Revoked, &a.RevokedAt, &a.RevocationReason, &a.Evidence, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (db *DB) ListAssertions(ctx context.Context, badgeClassID, recipientID int64, limit, offset int) ([]*model.Assertion, error) {
	query := `SELECT id, badge_class_id, issuer_id, recipient_id, issued_on, expires_at,
		revoked, revoked_at, revocation_reason, evidence, created_at
		FROM assertions WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if badgeClassID > 0 {
		query += fmt.Sprintf(" AND badge_class_id = $%d", argIdx)
		args = append(args, badgeClassID)
		argIdx++
	}
	if recipientID > 0 {
		query += fmt.Sprintf(" AND recipient_id = $%d", argIdx)
		args = append(args, recipientID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assertions []*model.Assertion
	for rows.Next() {
		var a model.Assertion
		if err := rows.Scan(&a.ID, &a.BadgeClassID, &a.IssuerID, &a.RecipientID, &a.IssuedOn,
			&a.ExpiresAt, &a.Revoked, &a.RevokedAt, &a.RevocationReason, &a.Evidence,
			&a.CreatedAt); err != nil {
			return nil, err
		}
		assertions = append(assertions, &a)
	}
	return assertions, nil
}

func (db *DB) RevokeAssertion(ctx context.Context, id int64, reason string) error {
	query := `UPDATE assertions SET revoked=true, revoked_at=NOW(), revocation_reason=$1 WHERE id=$2 AND revoked=false`
	result, err := db.conn.ExecContext(ctx, query, reason, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("assertion not found or already revoked: %d", id)
	}
	return nil
}

// ==========================================
// Recipient CRUD
// ==========================================

func (db *DB) CreateRecipient(ctx context.Context, r *model.Recipient) error {
	query := `INSERT INTO recipients (name, email)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`
	return db.conn.QueryRowContext(ctx, query, r.Name, r.Email).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
}

func (db *DB) GetRecipient(ctx context.Context, id int64) (*model.Recipient, error) {
	query := `SELECT id, name, email, created_at, updated_at FROM recipients WHERE id = $1`
	var r model.Recipient
	err := db.conn.QueryRowContext(ctx, query, id).Scan(&r.ID, &r.Name, &r.Email, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *DB) GetRecipientByEmail(ctx context.Context, email string) (*model.Recipient, error) {
	query := `SELECT id, name, email, created_at, updated_at FROM recipients WHERE email = $1`
	var r model.Recipient
	err := db.conn.QueryRowContext(ctx, query, email).Scan(&r.ID, &r.Name, &r.Email, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *DB) ListRecipients(ctx context.Context, limit, offset int) ([]*model.Recipient, error) {
	query := `SELECT id, name, email, created_at, updated_at FROM recipients ORDER BY id DESC LIMIT $1 OFFSET $2`
	rows, err := db.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipients []*model.Recipient
	for rows.Next() {
		var r model.Recipient
		if err := rows.Scan(&r.ID, &r.Name, &r.Email, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		recipients = append(recipients, &r)
	}
	return recipients, nil
}
