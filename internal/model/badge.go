package model

import "time"

// BadgeClass Open Badges 배지 클래스 (OB 3.0 Achievement)
type BadgeClass struct {
	ID          int64     `db:"id"           json:"id"`
	IssuerID    int64     `db:"issuer_id"    json:"issuerId"`
	Name        string    `db:"name"         json:"name"`
	Description string    `db:"description"  json:"description"`
	ImageURL    string    `db:"image_url"    json:"image"`
	CriteriaURL string   `db:"criteria_url" json:"criteria"`
	Tags        []string `db:"-"            json:"tags,omitempty"`
	CreatedAt   time.Time `db:"created_at"   json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updatedAt"`
}
