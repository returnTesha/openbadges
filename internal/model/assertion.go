package model

import "time"

// Assertion Open Badges 발급 내역 (OB 3.0 Credential)
type Assertion struct {
	ID           int64      `db:"id"            json:"id"`
	BadgeClassID int64      `db:"badge_class_id" json:"badgeClassId"`
	IssuerID     int64      `db:"issuer_id"     json:"issuerId"`
	RecipientID  int64      `db:"recipient_id"  json:"recipientId"`
	IssuedOn     time.Time  `db:"issued_on"     json:"issuedOn"`
	ExpiresAt    *time.Time `db:"expires_at"    json:"expiresAt,omitempty"`
	Revoked      bool       `db:"revoked"       json:"revoked"`
	RevokedAt    *time.Time `db:"revoked_at"    json:"revokedAt,omitempty"`
	RevocationReason string `db:"revocation_reason" json:"revocationReason,omitempty"`
	Evidence     string     `db:"evidence"      json:"evidence,omitempty"`
	CreatedAt    time.Time  `db:"created_at"    json:"createdAt"`
}
