// verifier.go — Badge credential verification service.
// Verifies Open Badges 3.0 credentials by resolving the issuer's DID,
// extracting the public key, and validating the Ed25519 signature.
package service

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RevocationChecker checks whether a credential has been revoked on-chain.
// This interface is satisfied by pkg/blockchain.Client.
type RevocationChecker interface {
	IsRevoked(ctx context.Context, credentialID string) (bool, error)
}

// VerificationService verifies OB 3.0 badge credentials.
type VerificationService struct {
	resolver    *DIDResolverService
	revocation  RevocationChecker // nil = revocation check skipped
}

// VerificationResult contains the outcome of a badge verification attempt.
type VerificationResult struct {
	Valid           bool                   `json:"valid"`
	Credential      map[string]interface{} `json:"credential,omitempty"`
	IssuerDID       string                 `json:"issuer_did"`
	IssuerName      string                 `json:"issuer_name"`
	RecipientName   string                 `json:"recipient_name,omitempty"`
	AchievementName string                 `json:"achievement_name,omitempty"`
	IssuedAt        string                 `json:"issued_at,omitempty"`
	ExpiresAt       string                 `json:"expires_at,omitempty"`
	Errors          []string               `json:"errors,omitempty"`
}

// NewVerificationService creates a new VerificationService. The revocation
// parameter is optional — pass nil to skip on-chain revocation checks.
func NewVerificationService(resolver *DIDResolverService, revocation RevocationChecker) *VerificationService {
	return &VerificationService{
		resolver:   resolver,
		revocation: revocation,
	}
}

// VerifyCredential verifies an OB 3.0 OpenBadgeCredential JSON.
//
// Verification steps:
//  1. Parse the credential JSON into a generic map.
//  2. Extract the proof section.
//  3. Extract the issuer DID (issuer.id).
//  4. Extract didFallback.contractAddress if present.
//  5. Resolve the DID to get the public key (with fallback).
//  6. Reconstruct canonical form (credential without proof, sorted keys, SHA-256).
//  7. Decode the proofValue from multibase.
//  8. Verify the Ed25519 signature.
//  9. Check revocation status on-chain via BadgeRegistry contract.
//  10. Check expirationDate if present.
//  11. Return VerificationResult.
func (v *VerificationService) VerifyCredential(ctx context.Context, credentialJSON []byte) (*VerificationResult, error) {
	result := &VerificationResult{
		Valid:  false,
		Errors: []string{},
	}

	// Step 1: Parse the credential JSON.
	var credential map[string]interface{}
	if err := json.Unmarshal(credentialJSON, &credential); err != nil {
		result.Errors = append(result.Errors, "invalid credential JSON: "+err.Error())
		return result, nil
	}
	result.Credential = credential

	// Step 2: Extract the proof section.
	proofRaw, ok := credential["proof"]
	if !ok {
		result.Errors = append(result.Errors, "credential has no proof section")
		return result, nil
	}
	proof, ok := proofRaw.(map[string]interface{})
	if !ok {
		result.Errors = append(result.Errors, "proof section is not a valid object")
		return result, nil
	}

	// Step 3: Extract the issuer DID.
	issuerDID, issuerName := extractIssuerInfo(credential)
	if issuerDID == "" {
		result.Errors = append(result.Errors, "could not extract issuer DID from credential")
		return result, nil
	}
	result.IssuerDID = issuerDID
	result.IssuerName = issuerName

	// Extract additional metadata for the result.
	result.RecipientName = extractRecipientName(credential)
	result.AchievementName = extractAchievementName(credential)
	// W3C VC v2 uses validFrom; fall back to v1's issuanceDate.
	result.IssuedAt = extractStringField(credential, "validFrom")
	if result.IssuedAt == "" {
		result.IssuedAt = extractStringField(credential, "issuanceDate")
	}
	// W3C VC v2 uses validUntil; fall back to v1's expirationDate.
	result.ExpiresAt = extractStringField(credential, "validUntil")
	if result.ExpiresAt == "" {
		result.ExpiresAt = extractStringField(credential, "expirationDate")
	}

	// Step 4: Extract didFallback.contractAddress if present.
	fallbackAddr := extractFallbackAddress(credential)

	// Determine issuedAt time for fallback resolution.
	issuedAt := time.Now()
	if result.IssuedAt != "" {
		if t, err := time.Parse(time.RFC3339, result.IssuedAt); err == nil {
			issuedAt = t
		}
	}

	// Step 5: Resolve the DID to get the public key.
	doc, err := v.resolver.ResolveWithFallback(ctx, issuerDID, fallbackAddr, issuedAt)
	if err != nil {
		result.Errors = append(result.Errors, "DID resolution failed: "+err.Error())
		return result, nil
	}

	// Find the verification method referenced in the proof, or use the first one.
	pubKey, err := extractPublicKey(doc, proof)
	if err != nil {
		result.Errors = append(result.Errors, "failed to extract public key: "+err.Error())
		return result, nil
	}

	// Step 6: Reconstruct canonical form — credential without proof, sorted keys, SHA-256.
	canonicalHash, err := canonicalize(credential)
	if err != nil {
		result.Errors = append(result.Errors, "failed to canonicalize credential: "+err.Error())
		return result, nil
	}

	// Step 7: Decode the proofValue from multibase.
	proofValueStr, ok := proof["proofValue"].(string)
	if !ok || proofValueStr == "" {
		result.Errors = append(result.Errors, "proof has no proofValue")
		return result, nil
	}

	signature, err := decodeMultibaseSignature(proofValueStr)
	if err != nil {
		result.Errors = append(result.Errors, "failed to decode proofValue: "+err.Error())
		return result, nil
	}

	// Step 8: Verify the Ed25519 signature against the canonical hash.
	if !ed25519.Verify(pubKey, canonicalHash, signature) {
		result.Errors = append(result.Errors, "Ed25519 signature verification failed")
		return result, nil
	}

	// Step 9: Check revocation status on-chain.
	if v.revocation != nil {
		credentialID := extractStringField(credential, "id")
		// URL에서 credential_id 추출: "https://.../credentials/SNU-LEADERSHIP-20261" → "SNU-LEADERSHIP-20261"
		if idx := strings.LastIndex(credentialID, "/"); idx >= 0 {
			credentialID = credentialID[idx+1:]
		}
		if credentialID != "" {
			revoked, revErr := v.revocation.IsRevoked(ctx, credentialID)
			if revErr != nil {
				result.Errors = append(result.Errors, "revocation check failed: "+revErr.Error())
				return result, nil
			}
			if revoked {
				result.Errors = append(result.Errors, "credential has been revoked")
				return result, nil
			}
		}
	}

	// Step 10: Check expirationDate if present.
	if result.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, result.ExpiresAt)
		if err != nil {
			result.Errors = append(result.Errors, "invalid expirationDate format: "+err.Error())
			return result, nil
		}
		if time.Now().After(expiresAt) {
			result.Errors = append(result.Errors, "credential has expired at "+result.ExpiresAt)
			return result, nil
		}
	}

	// Step 11: All checks passed.
	result.Valid = true
	return result, nil
}

// extractIssuerInfo extracts the issuer DID and name from a credential.
// The issuer field can be a string (just the DID) or an object with id and name.
func extractIssuerInfo(credential map[string]interface{}) (did string, name string) {
	issuer, ok := credential["issuer"]
	if !ok {
		return "", ""
	}

	switch v := issuer.(type) {
	case string:
		return v, ""
	case map[string]interface{}:
		id, _ := v["id"].(string)
		n, _ := v["name"].(string)
		return id, n
	}
	return "", ""
}

// extractRecipientName extracts the recipient name from credentialSubject.
func extractRecipientName(credential map[string]interface{}) string {
	subject, ok := credential["credentialSubject"].(map[string]interface{})
	if !ok {
		return ""
	}
	if name, ok := subject["name"].(string); ok {
		return name
	}
	return ""
}

// extractAchievementName extracts the achievement name from credentialSubject.achievement.
func extractAchievementName(credential map[string]interface{}) string {
	subject, ok := credential["credentialSubject"].(map[string]interface{})
	if !ok {
		return ""
	}
	achievement, ok := subject["achievement"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := achievement["name"].(string)
	return name
}

// extractStringField extracts a top-level string field from the credential.
func extractStringField(credential map[string]interface{}, key string) string {
	v, _ := credential[key].(string)
	return v
}

// extractFallbackAddress extracts the Polygon contract address from
// the didFallback field in the credential.
func extractFallbackAddress(credential map[string]interface{}) string {
	fallback, ok := credential["didFallback"].(map[string]interface{})
	if !ok {
		return ""
	}
	addr, _ := fallback["contractAddress"].(string)
	return addr
}

// extractPublicKey finds the appropriate verification method from the DID document
// and decodes the Ed25519 public key. If the proof specifies a verificationMethod,
// that specific key is used; otherwise the first available key is used.
func extractPublicKey(doc *DIDDocument, proof map[string]interface{}) (ed25519.PublicKey, error) {
	if len(doc.VerificationMethod) == 0 {
		return nil, fmt.Errorf("DID document has no verification methods")
	}

	// Determine which verification method the proof references.
	targetID, _ := proof["verificationMethod"].(string)

	for _, vm := range doc.VerificationMethod {
		// If a specific method is referenced, match it. Otherwise, use the first one.
		if targetID != "" && vm.ID != targetID {
			continue
		}

		if vm.PublicKeyMultibase == "" {
			continue
		}

		pubKey, err := DecodePublicKeyMultibase(vm.PublicKeyMultibase)
		if err != nil {
			return nil, fmt.Errorf("failed to decode public key from verification method %s: %w", vm.ID, err)
		}
		return pubKey, nil
	}

	if targetID != "" {
		return nil, fmt.Errorf("verification method %q not found in DID document", targetID)
	}
	return nil, fmt.Errorf("no usable public key found in DID document")
}

// canonicalize produces a deterministic hash of the credential for signature
// verification. It removes the proof section, serializes with sorted keys,
// and computes a SHA-256 hash.
//
// This is a simplified canonicalization. A full implementation would use
// RDFC-1.0 (RDF Dataset Canonicalization), but for the MVP we use sorted-key
// JSON serialization followed by SHA-256, matching the signing side.
func canonicalize(credential map[string]interface{}) ([]byte, error) {
	// Create a copy without the proof section.
	withoutProof := make(map[string]interface{}, len(credential))
	for k, v := range credential {
		if k == "proof" {
			continue
		}
		withoutProof[k] = v
	}

	// Use the same canonicalJSON function used during signing to ensure
	// identical serialization (marshal → unmarshal → marshalSorted).
	canonical, err := canonicalJSON(withoutProof)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize canonical form: %w", err)
	}

	// Compute SHA-256 hash.
	hash := sha256.Sum256(canonical)
	return hash[:], nil
}

// marshalSorted is defined in credential.go and shared across the package.
// It serializes a map to JSON with keys sorted alphabetically at every
// nesting level for deterministic output.

// decodeMultibaseSignature decodes a multibase-encoded signature value.
// The expected format is "z" prefix followed by base58btc-encoded bytes.
func decodeMultibaseSignature(multibase string) ([]byte, error) {
	if len(multibase) == 0 {
		return nil, fmt.Errorf("empty multibase signature")
	}

	if multibase[0] != 'z' {
		return nil, fmt.Errorf("unsupported multibase prefix %q: expected 'z' (base58btc)", string(multibase[0]))
	}

	decoded, err := base58Decode(multibase[1:])
	if err != nil {
		return nil, fmt.Errorf("base58btc decode failed: %w", err)
	}

	return decoded, nil
}
