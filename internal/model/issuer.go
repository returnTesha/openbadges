package model

import "time"

// Issuer Open Badges 발급자 (OB 3.0 Profile)
type Issuer struct {
	ID          int64     `db:"id"           json:"id"`
	Name        string    `db:"name"         json:"name"`
	URL         string    `db:"url"          json:"url"`
	Email       string    `db:"email"        json:"email,omitempty"`
	Description string    `db:"description"  json:"description,omitempty"`
	ImageURL    string    `db:"image_url"    json:"image,omitempty"`
	CreatedAt   time.Time `db:"created_at"   json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updatedAt"`
}
