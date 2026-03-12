package test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"
)

// ============================================================
// 비교과 수료 배지 발급 시뮬레이션
//
// 시나리오: "한국폴리텍대학 광명융합기술교육원"이
//           "AI·SW 비교과 프로그램 수료" 배지를 학생에게 발급
//
// 전체 흐름:
//   1. 발급자 Ed25519 키 쌍 생성
//   2. 배지 이미지를 Base64 Data URI로 임베딩
//   3. 비교과 수료 Credential JSON 생성
//   4. 정규화(간이) → SHA-256 해시 → Ed25519 서명
//   5. proof 블록 추가 → 최종 Signed Credential 출력
//   6. 검증: 서명 유효성 확인
//   7. 위변조 테스트: 내용 변조 시 검증 실패 확인
// ============================================================

// --------------- OB 3.0 타입 정의 ---------------

type Image struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type Criteria struct {
	Narrative string `json:"narrative"`
}

type Achievement struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Criteria    Criteria `json:"criteria"`
	Image       Image    `json:"image"`
}

type IdentityObject struct {
	Type         string `json:"type"`
	Hashed       bool   `json:"hashed"`
	IdentityHash string `json:"identityHash"`
	IdentityType string `json:"identityType"`
	Salt         string `json:"salt"`
}

type CredentialSubject struct {
	Type        string           `json:"type"`
	Identifier  []IdentityObject `json:"identifier"`
	Achievement Achievement      `json:"achievement"`
}

type Profile struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	URL   string `json:"url,omitempty"`
	Image Image  `json:"image,omitempty"`
}

type Proof struct {
	Type               string `json:"type"`
	Created            string `json:"created"`
	VerificationMethod string `json:"verificationMethod"`
	Cryptosuite        string `json:"cryptosuite"`
	ProofPurpose       string `json:"proofPurpose"`
	ProofValue         string `json:"proofValue"`
}

type AchievementCredential struct {
	Context           []string          `json:"@context"`
	ID                string            `json:"id"`
	Type              []string          `json:"type"`
	Issuer            Profile           `json:"issuer"`
	IssuanceDate      string            `json:"issuanceDate"`
	ExpirationDate    string            `json:"expirationDate,omitempty"`
	Name              string            `json:"name"`
	CredentialSubject CredentialSubject  `json:"credentialSubject"`
	Proof             *Proof            `json:"proof,omitempty"`
}

// --------------- 헬퍼 함수 ---------------

// hashEmail: 수령자 이메일을 SHA-256 해시 (프라이버시 보호)
func hashEmail(email, salt string) string {
	h := sha256.Sum256([]byte(email + salt))
	return fmt.Sprintf("sha256$%x", h)
}

// createSampleBadgeImage: 테스트용 SVG 배지 이미지를 Base64 Data URI로 생성
func createSampleBadgeImage() string {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200">
  <defs>
    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:#1a237e"/>
      <stop offset="100%" style="stop-color:#0d47a1"/>
    </linearGradient>
  </defs>
  <circle cx="100" cy="100" r="95" fill="url(#bg)" stroke="#ffd600" stroke-width="4"/>
  <text x="100" y="75" text-anchor="middle" fill="#ffffff" font-size="16" font-weight="bold">AI·SW</text>
  <text x="100" y="100" text-anchor="middle" fill="#ffd600" font-size="13" font-weight="bold">비교과 프로그램</text>
  <text x="100" y="125" text-anchor="middle" fill="#ffffff" font-size="14">수료</text>
  <text x="100" y="165" text-anchor="middle" fill="#bbdefb" font-size="9">한국폴리텍대학</text>
  <text x="100" y="180" text-anchor="middle" fill="#bbdefb" font-size="8">광명융합기술교육원</text>
</svg>`
	encoded := base64.StdEncoding.EncodeToString([]byte(svg))
	return "data:image/svg+xml;base64," + encoded
}

// signCredential: credential JSON을 Ed25519로 서명
func signCredential(credJSON []byte, privateKey ed25519.PrivateKey) []byte {
	// Step 1: 문서 해시 (실제로는 RDFC-1.0 정규화 후 해시해야 하지만, 데모에서는 JSON 바이트 직접 해시)
	docHash := sha256.Sum256(credJSON)

	// Step 2: proof options 해시
	proofOptions := fmt.Sprintf(`{"type":"DataIntegrityProof","cryptosuite":"eddsa-rdfc-2022","proofPurpose":"assertionMethod","created":"%s"}`,
		time.Now().UTC().Format(time.RFC3339))
	proofHash := sha256.Sum256([]byte(proofOptions))

	// Step 3: 두 해시 결합 (64 bytes)
	hashData := append(proofHash[:], docHash[:]...)

	// Step 4: Ed25519 서명
	signature := ed25519.Sign(privateKey, hashData)

	return signature
}

// verifyCredential: credential JSON의 서명을 Ed25519로 검증
func verifyCredential(credJSON []byte, signature []byte, proofCreated string, publicKey ed25519.PublicKey) bool {
	docHash := sha256.Sum256(credJSON)

	proofOptions := fmt.Sprintf(`{"type":"DataIntegrityProof","cryptosuite":"eddsa-rdfc-2022","proofPurpose":"assertionMethod","created":"%s"}`,
		proofCreated)
	proofHash := sha256.Sum256([]byte(proofOptions))

	hashData := append(proofHash[:], docHash[:]...)

	return ed25519.Verify(publicKey, hashData, signature)
}

// --------------- 메인 테스트 ---------------

func TestIssueBadge_비교과수료(t *testing.T) {
	fmt.Println("==========================================================")
	fmt.Println("  비교과 수료 배지 발급 시뮬레이션 (Open Badges 3.0)")
	fmt.Println("==========================================================")

	// ── 1. 발급자 키 쌍 생성 ──
	fmt.Println("\n[1단계] Ed25519 키 쌍 생성")
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("키 생성 실패: %v", err)
	}
	pubKeyMultibase := "z" + base58.Encode(append([]byte{0xed, 0x01}, publicKey...))
	fmt.Printf("  공개키 (Multibase): %s\n", pubKeyMultibase)
	fmt.Printf("  공개키 길이: %d bytes\n", len(publicKey))
	fmt.Printf("  비밀키 길이: %d bytes (발급자만 보관)\n", len(privateKey))

	// ── 2. 배지 이미지 Base64 임베딩 ──
	fmt.Println("\n[2단계] 배지 이미지 Base64 Data URI 생성")
	badgeImageDataURI := createSampleBadgeImage()
	fmt.Printf("  이미지 형식: SVG (Base64 인코딩)\n")
	fmt.Printf("  Data URI 길이: %d bytes\n", len(badgeImageDataURI))
	fmt.Printf("  Data URI 미리보기: %s...\n", badgeImageDataURI[:60])
	fmt.Println("  → 이미지가 JSON 안에 임베딩됨 = 발급사 서버 없어도 이미지 유실 없음")

	// ── 3. 수령자 이메일 해싱 ──
	fmt.Println("\n[3단계] 수령자 이메일 해싱 (프라이버시 보호)")
	recipientEmail := "hong.gildong@kopo.ac.kr"
	salt := "kopo-2025-badge-salt"
	emailHash := hashEmail(recipientEmail, salt)
	fmt.Printf("  원본 이메일: %s\n", recipientEmail)
	fmt.Printf("  Salt: %s\n", salt)
	fmt.Printf("  해시 결과: %s\n", emailHash)

	// ── 4. Unsigned Credential 생성 ──
	fmt.Println("\n[4단계] 비교과 수료 Credential 생성 (서명 전)")

	now := time.Now().UTC()
	expiration := now.AddDate(2, 0, 0) // 2년 유효

	credential := AchievementCredential{
		Context: []string{
			"https://www.w3.org/ns/credentials/v2",
			"https://purl.imsglobal.org/spec/ob/v3p0/context-3.0.3.json",
		},
		ID:             "https://badge.kopo.ac.kr/credentials/2025/NC-001",
		Type:           []string{"VerifiableCredential", "AchievementCredential"},
		Issuer: Profile{
			ID:   "did:web:badge.kopo.ac.kr:issuers:gwangmyeong",
			Type: "Profile",
			Name: "한국폴리텍대학 광명융합기술교육원",
			URL:  "https://www.kopo.ac.kr/gwangmyeong",
			Image: Image{
				ID:   "data:image/png;base64,iVBORw0KGgo=", // 기관 로고 (축약)
				Type: "Image",
			},
		},
		IssuanceDate:   now.Format(time.RFC3339),
		ExpirationDate: expiration.Format(time.RFC3339),
		Name:           "AI·SW 비교과 프로그램 수료",
		CredentialSubject: CredentialSubject{
			Type: "AchievementSubject",
			Identifier: []IdentityObject{
				{
					Type:         "IdentityObject",
					Hashed:       true,
					IdentityHash: emailHash,
					IdentityType: "emailAddress",
					Salt:         salt,
				},
			},
			Achievement: Achievement{
				ID:          "https://badge.kopo.ac.kr/achievements/aisw-noncurricular-2025",
				Type:        "Achievement",
				Name:        "AI·SW 비교과 프로그램 수료",
				Description: "AI·소프트웨어 분야 비교과 활동 프로그램(해커톤, 특강, 멘토링, 프로젝트)을 성실히 이수한 학생에게 수여하는 디지털 배지",
				Criteria: Criteria{
					Narrative: "비교과 활동 4회 이상 참여 및 최종 결과물 제출 완료",
				},
				Image: Image{
					ID:   badgeImageDataURI, // ← Base64 Data URI로 임베딩!
					Type: "Image",
				},
			},
		},
	}

	unsignedJSON, err := json.MarshalIndent(credential, "", "  ")
	if err != nil {
		t.Fatalf("JSON 직렬화 실패: %v", err)
	}
	fmt.Printf("\n--- Unsigned Credential ---\n%s\n", string(unsignedJSON))

	// ── 5. 서명 생성 ──
	fmt.Println("\n[5단계] Ed25519 서명 생성")
	proofCreated := now.Format(time.RFC3339)
	signature := signCredential(unsignedJSON, privateKey)
	proofValue := "z" + base58.Encode(signature)
	fmt.Printf("  서명 알고리즘: Ed25519\n")
	fmt.Printf("  서명값 (raw): %d bytes\n", len(signature))
	fmt.Printf("  proofValue (base58-btc): %s\n", proofValue)

	// ── 6. Signed Credential 완성 ──
	fmt.Println("\n[6단계] proof 블록 추가 → 최종 Signed Credential")
	credential.Proof = &Proof{
		Type:               "DataIntegrityProof",
		Created:            proofCreated,
		VerificationMethod: "did:web:badge.kopo.ac.kr:issuers:gwangmyeong#key-1",
		Cryptosuite:        "eddsa-rdfc-2022",
		ProofPurpose:       "assertionMethod",
		ProofValue:         proofValue,
	}

	signedJSON, err := json.MarshalIndent(credential, "", "  ")
	if err != nil {
		t.Fatalf("서명된 JSON 직렬화 실패: %v", err)
	}
	fmt.Printf("\n--- Signed Credential (최종 발급 결과) ---\n%s\n", string(signedJSON))

	// ── 7. 검증 ──
	fmt.Println("\n[7단계] 배지 검증")

	// 7-1. 정상 검증
	fmt.Println("\n  [검증 1] 원본 배지 → 정상 검증")
	valid := verifyCredential(unsignedJSON, signature, proofCreated, publicKey)
	fmt.Printf("  결과: %v\n", valid)
	if !valid {
		t.Fatal("정상 배지 검증 실패!")
	}
	fmt.Println("  → 이 배지는 한국폴리텍대학 광명융합기술교육원이 발급한 진짜 배지입니다")

	// 7-2. 위변조 테스트
	fmt.Println("\n  [검증 2] 위변조 테스트 — 이름을 '석사 학위'로 변조")
	tampered := AchievementCredential{}
	json.Unmarshal(unsignedJSON, &tampered)
	tampered.Name = "석사 학위" // 위변조!
	tamperedJSON, _ := json.MarshalIndent(tampered, "", "  ")

	validTampered := verifyCredential(tamperedJSON, signature, proofCreated, publicKey)
	fmt.Printf("  결과: %v\n", validTampered)
	if validTampered {
		t.Fatal("위변조된 배지가 검증 통과해버림!")
	}
	fmt.Println("  → 위변조 감지! 서명이 일치하지 않습니다")

	// 7-3. 가짜 서명 테스트
	fmt.Println("\n  [검증 3] 가짜 서명 테스트 — 다른 키로 서명")
	_, fakePrivKey, _ := ed25519.GenerateKey(nil)
	fakeSignature := signCredential(unsignedJSON, fakePrivKey)
	validFake := verifyCredential(unsignedJSON, fakeSignature, proofCreated, publicKey)
	fmt.Printf("  결과: %v\n", validFake)
	if validFake {
		t.Fatal("가짜 서명이 검증 통과해버림!")
	}
	fmt.Println("  → 가짜 서명 감지! 발급자의 키로 서명되지 않았습니다")

	// ── 8. Base64 이미지 검증 포인트 ──
	fmt.Println("\n[8단계] Base64 이미지 임베딩 효과 확인")
	fmt.Println("  ┌─────────────────────────────────────────────────────┐")
	fmt.Println("  │  이미지 저장 방식 비교                                │")
	fmt.Println("  │                                                     │")
	fmt.Println("  │  [기존 URL 방식]                                     │")
	fmt.Println("  │   image.id = \"https://example.com/badge.png\"        │")
	fmt.Println("  │   → 서버 다운 시 이미지 유실 (404)                    │")
	fmt.Println("  │   → 서명은 URL 문자열만 보호                          │")
	fmt.Println("  │                                                     │")
	fmt.Println("  │  [Base64 Data URI 방식] ← 우리가 선택한 방식          │")
	fmt.Println("  │   image.id = \"data:image/svg+xml;base64,PHN2Zy...\"  │")
	fmt.Println("  │   → 이미지 데이터가 JSON 안에 포함                     │")
	fmt.Println("  │   → 서명이 이미지 데이터까지 보호                      │")
	fmt.Println("  │   → 서버 없어도 이미지 표시 가능                       │")
	fmt.Println("  │   → 이미지 위변조 시 서명 검증 실패                    │")
	fmt.Println("  └─────────────────────────────────────────────────────┘")

	// ── 9. DID Document 예시 ──
	fmt.Println("\n[참고] 발급자 DID Document (공개키 공개용)")
	fmt.Println("  URL: https://badge.kopo.ac.kr/issuers/gwangmyeong/did.json")
	didDoc := map[string]interface{}{
		"@context": []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/multikey/v1",
		},
		"id": "did:web:badge.kopo.ac.kr:issuers:gwangmyeong",
		"verificationMethod": []map[string]string{
			{
				"id":                 "did:web:badge.kopo.ac.kr:issuers:gwangmyeong#key-1",
				"type":               "Multikey",
				"controller":         "did:web:badge.kopo.ac.kr:issuers:gwangmyeong",
				"publicKeyMultibase": pubKeyMultibase,
			},
		},
		"assertionMethod": []string{
			"did:web:badge.kopo.ac.kr:issuers:gwangmyeong#key-1",
		},
	}
	didJSON, _ := json.MarshalIndent(didDoc, "  ", "  ")
	fmt.Printf("  %s\n", string(didJSON))

	fmt.Println("\n==========================================================")
	fmt.Println("  시뮬레이션 완료")
	fmt.Println("  ")
	fmt.Println("  발급된 배지 요약:")
	fmt.Printf("    배지명: %s\n", credential.Name)
	fmt.Printf("    발급자: %s\n", credential.Issuer.Name)
	fmt.Printf("    발급일: %s\n", credential.IssuanceDate)
	fmt.Printf("    만료일: %s\n", credential.ExpirationDate)
	fmt.Printf("    이미지: Base64 임베딩 (%d bytes)\n", len(badgeImageDataURI))
	fmt.Printf("    서명: %s\n", proofValue)
	fmt.Println("    검증: 정상 ✓ | 위변조 감지 ✓ | 가짜서명 감지 ✓")
	fmt.Println("==========================================================")
}
