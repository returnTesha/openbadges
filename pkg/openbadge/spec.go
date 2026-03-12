package openbadge

import "time"

// OB 3.0 JSON-LD Context
const (
	ContextURL = "https://purl.imsglobal.org/spec/ob/v3p0/context-3.0.3.json"
	TypeAchievementCredential = "AchievementCredential"
	TypeAchievement           = "Achievement"
	TypeProfile               = "Profile"
)

// AchievementCredential OB 3.0 배지 발급 증명 (Verifiable Credential)
type AchievementCredential struct {
	Context      []string    `json:"@context"`
	ID           string      `json:"id"`
	Type         []string    `json:"type"`
	Issuer       Profile     `json:"issuer"`
	IssuanceDate string      `json:"issuanceDate"`
	ExpirationDate string   `json:"expirationDate,omitempty"`
	Name         string      `json:"name"`
	CredentialSubject CredentialSubject `json:"credentialSubject"`
}

// Profile OB 3.0 발급자 프로필
type Profile struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Email       string `json:"email,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
}

// CredentialSubject OB 3.0 수령자 + 달성 정보
type CredentialSubject struct {
	ID          string      `json:"id,omitempty"`
	Type        string      `json:"type"`
	Achievement Achievement `json:"achievement"`
}

// Achievement OB 3.0 배지 정의
type Achievement struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Criteria    Criteria `json:"criteria"`
	Image       string   `json:"image,omitempty"`
}

// Criteria 배지 획득 기준
type Criteria struct {
	Narrative string `json:"narrative,omitempty"`
	ID        string `json:"id,omitempty"`
}

// NewAchievementCredential OB 3.0 AchievementCredential 생성 헬퍼
func NewAchievementCredential(id string, issuer Profile, subject CredentialSubject, issuedOn time.Time) *AchievementCredential {
	return &AchievementCredential{
		Context: []string{
			"https://www.w3.org/ns/credentials/v2",
			ContextURL,
		},
		ID:           id,
		Type:         []string{"VerifiableCredential", TypeAchievementCredential},
		Issuer:       issuer,
		IssuanceDate: issuedOn.Format(time.RFC3339),
		CredentialSubject: subject,
	}
}
