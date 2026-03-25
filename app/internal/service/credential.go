// credential.go — OB 3.0 OpenBadgeCredential builder.
//
// Constructs a W3C Verifiable Credential in the Open Badges 3.0 format,
// signs it using the eddsa-rdfc-2022 cryptosuite (DataIntegrityProof), and
// returns the complete credential as a JSON map.
//
// Canonicalization note:
// Full RDFC-1.0 (RDF Dataset Canonicalization) requires an N-Quads serialiser
// and the URDNA2015/RDFC-1.0 algorithm. No mature, dependency-free Go library
// exists yet. This implementation uses a simplified canonical form:
//   1. Build the credential JSON WITHOUT the proof field.
//   2. Marshal with deterministic (sorted-key) JSON serialization.
//   3. SHA-256 hash the canonical bytes.
//   4. Ed25519-sign the hash.
//   5. Encode the signature as multibase (z + base58btc).
//
// Production deployments should replace step 2 with full RDFC-1.0 once a
// suitable Go library is available.
package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CredentialBuilder constructs and signs OB 3.0 OpenBadgeCredentials.
type CredentialBuilder struct {
	signer *SignerService
}

// NewCredentialBuilder creates a builder that signs credentials using the
// provided SignerService.
func NewCredentialBuilder(signer *SignerService) *CredentialBuilder {
	return &CredentialBuilder{signer: signer}
}

// CredentialParams holds the input parameters for building a badge credential.
type CredentialParams struct {
	IssuerDID       string // e.g. "did:web:badge.example.com"
	IssuerName      string
	ServerURL       string // e.g. "https://badge.example.com"
	ContractAddress string // Polygon contract address for didFallback

	CredentialID    string // e.g. "SNU-LEADERSHIP-20261"
	AchievementID   string
	AchievementName string
	AchievementDesc string
	Criteria        string
	ImageBase64     string // base64-encoded PNG/SVG badge image (without data URI prefix)

	// CPS integration fields
	UniversityCode  string // e.g. "SNU"
	ProgramID       string // e.g. "NPI00001"
	ProgramCategory string // e.g. "LEADERSHIP"
	StudentID       string // e.g. "2021-12345"

	RecipientName  string
	RecipientEmail string
	RecipientDID   string // optional; if provided, used as credentialSubject.id
}

// BuildCredential constructs a signed OpenBadgeCredential JSON-LD document.
//
// Signing flow:
//   1. Assemble the credential body (everything except the proof).
//   2. Canonicalize: serialize to deterministic JSON with sorted keys.
//   3. Hash: SHA-256 of the canonical bytes.
//   4. Sign: Ed25519 over the hash.
//   5. Encode: multibase (z + base58btc) the raw signature.
//   6. Attach the proof object to the credential.
func (cb *CredentialBuilder) BuildCredential(params CredentialParams) (map[string]interface{}, error) {
	now := time.Now().UTC()

	// Use the server-assigned credential ID if provided, otherwise fall back to UUID.
	credentialID := params.CredentialID
	if credentialID == "" {
		credentialID = "urn:uuid:" + uuid.New().String()
	}
	badgeID := credentialID // use the credential ID as the badge ID for URL construction

	// If no explicit achievement ID is provided, generate one.
	achievementURN := params.AchievementID
	if achievementURN == "" {
		achievementURN = "urn:uuid:" + uuid.New().String()
	}

	// --- Build the credential body (without proof) ---

	// Normalize image: ensure data URI prefix is present exactly once.
	imageURI := ensureImageDataURI(params.ImageBase64)

	// Build achievement object with optional CPS fields.
	achievementPairs := []keyValue{
		kv("id", achievementURN),
		kv("type", []string{"Achievement"}),
		kv("name", params.AchievementName),
		kv("description", params.AchievementDesc),
		kv("criteria", orderedMap(
			kv("narrative", params.Criteria),
		)),
	}
	if params.ProgramCategory != "" {
		achievementPairs = append(achievementPairs, kv("category", params.ProgramCategory))
	}
	if params.ProgramID != "" {
		achievementPairs = append(achievementPairs, kv("programId", params.ProgramID))
	}
	achievementPairs = append(achievementPairs,
		kv("image", orderedMap(
			kv("id", imageURI),
			kv("type", "Image"),
		)),
	)

	// Build credentialSubject with optional CPS identity fields.
	subjectPairs := []keyValue{}
	if params.RecipientDID != "" {
		subjectPairs = append(subjectPairs, kv("id", params.RecipientDID))
	}
	subjectPairs = append(subjectPairs,
		kv("type", []string{"AchievementSubject"}),
		kv("name", params.RecipientName),
	)
	if params.StudentID != "" {
		subjectPairs = append(subjectPairs, kv("identifier", orderedMap(
			kv("type", "StudentId"),
			kv("identityValue", params.StudentID),
		)))
	}
	if params.UniversityCode != "" {
		subjectPairs = append(subjectPairs, kv("source", orderedMap(
			kv("code", params.UniversityCode),
		)))
	}
	subjectPairs = append(subjectPairs, kv("achievement", orderedMapSlice(achievementPairs)))
	subject := orderedMapSlice(subjectPairs)

	// Build the credential URL from the credential ID.
	// For {year}{sequence} IDs: https://thebadge.kr/credentials/20261
	// For legacy URN UUIDs: extract the UUID portion.
	shortID := credentialID
	if strings.HasPrefix(credentialID, "urn:uuid:") {
		shortID = credentialID[len("urn:uuid:"):]
	}
	credentialURL := fmt.Sprintf("%s/credentials/%s", params.ServerURL, shortID)

	// Assemble the full credential (without proof).
	credential := orderedMap(
		kv("@context", []string{
			"https://www.w3.org/ns/credentials/v2",
			"https://purl.imsglobal.org/spec/ob/v3p0/context-3.0.3.json",
		}),
		kv("type", []string{"VerifiableCredential", "OpenBadgeCredential"}),
		kv("id", credentialURL),
		kv("issuer", orderedMap(
			kv("id", params.IssuerDID),
			kv("type", []string{"Profile"}),
			kv("name", params.IssuerName),
		)),
		kv("validFrom", now.Format(time.RFC3339)),
		kv("credentialSubject", subject),
		kv("didFallback", orderedMap(
			kv("type", "PolygonSmartContract"),
			kv("contractAddress", params.ContractAddress),
			kv("description", "Fallback: query this Polygon contract for the issuer public key if did:web resolution fails."),
		)),
	)

	// --- Signing flow ---

	// Step 1-2: Serialize without proof using deterministic JSON (sorted keys).
	canonicalBytes, err := canonicalJSON(credential)
	if err != nil {
		return nil, fmt.Errorf("credential: canonical JSON serialization failed: %w", err)
	}

	// Step 3: SHA-256 hash of the canonical form.
	hash := sha256.Sum256(canonicalBytes)

	// Step 4: Ed25519 sign the hash.
	signature, err := cb.signer.Sign(hash[:])
	if err != nil {
		return nil, fmt.Errorf("credential: signing failed: %w", err)
	}

	// Step 5: Encode signature as multibase (z + base58btc).
	proofValue := "z" + base58Encode(signature)

	// Step 6: Attach the proof.
	proof := orderedMap(
		kv("type", "DataIntegrityProof"),
		kv("cryptosuite", "eddsa-rdfc-2022"),
		kv("verificationMethod", cb.signer.KeyID()),
		kv("proofPurpose", "assertionMethod"),
		kv("created", now.Format(time.RFC3339)),
		kv("proofValue", proofValue),
	)

	// Add the proof to the credential.
	credential = append(credential, keyValue{Key: "proof", Value: proof})

	// Convert to a plain map for JSON serialization by the caller.
	// NOTE: badgeID (== credentialID) is available for callers that need to
	// persist the badge. The short ID can be extracted by trimming "urn:uuid:".
	_ = badgeID
	return orderedMapToMap(credential), nil
}

// --- Deterministic JSON helpers ---
// These helpers ensure keys are serialized in a stable, sorted order,
// which is critical for the simplified canonicalization approach.

// keyValue is an ordered key-value pair.
type keyValue struct {
	Key   string
	Value interface{}
}

// orderedMapSlice is an ordered list of key-value pairs that serializes
// to JSON with keys in insertion order.
type orderedMapSlice []keyValue

// kv is a convenience constructor for keyValue.
func kv(key string, value interface{}) keyValue {
	return keyValue{Key: key, Value: value}
}

// orderedMap creates an orderedMapSlice from key-value pairs.
func orderedMap(pairs ...keyValue) orderedMapSlice {
	return orderedMapSlice(pairs)
}

// MarshalJSON serializes orderedMapSlice as a JSON object preserving key order.
func (o orderedMapSlice) MarshalJSON() ([]byte, error) {
	buf := []byte{'{'}
	for i, pair := range o {
		if i > 0 {
			buf = append(buf, ',')
		}
		keyBytes, err := json.Marshal(pair.Key)
		if err != nil {
			return nil, err
		}
		valBytes, err := json.Marshal(pair.Value)
		if err != nil {
			return nil, err
		}
		buf = append(buf, keyBytes...)
		buf = append(buf, ':')
		buf = append(buf, valBytes...)
	}
	buf = append(buf, '}')
	return buf, nil
}

// canonicalJSON produces a deterministic JSON serialization with sorted keys.
// It first marshals with our ordered map, then re-parses and re-marshals through
// a generic interface{} to ensure all nested maps are also sorted.
func canonicalJSON(data interface{}) ([]byte, error) {
	// First pass: marshal with our ordered map implementation.
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// Second pass: unmarshal into generic interface and re-marshal with sorted keys.
	var generic interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}

	return marshalSorted(generic)
}

// marshalSorted recursively serializes a value with map keys in sorted order.
func marshalSorted(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		buf := []byte{'{'}
		for i, k := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			keyBytes, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			valBytes, err := marshalSorted(val[k])
			if err != nil {
				return nil, err
			}
			buf = append(buf, keyBytes...)
			buf = append(buf, ':')
			buf = append(buf, valBytes...)
		}
		buf = append(buf, '}')
		return buf, nil

	case []interface{}:
		buf := []byte{'['}
		for i, item := range val {
			if i > 0 {
				buf = append(buf, ',')
			}
			itemBytes, err := marshalSorted(item)
			if err != nil {
				return nil, err
			}
			buf = append(buf, itemBytes...)
		}
		buf = append(buf, ']')
		return buf, nil

	default:
		return json.Marshal(v)
	}
}

// ensureImageDataURI ensures the image string has a data URI prefix.
// If it already starts with "data:image/", it is returned unchanged.
// Otherwise, "data:image/png;base64," is prepended.
func ensureImageDataURI(img string) string {
	if strings.HasPrefix(img, "data:image/") {
		return img
	}
	return "data:image/png;base64," + img
}

// orderedMapToMap converts an orderedMapSlice to a plain map[string]interface{}
// for use with Fiber's c.JSON().
func orderedMapToMap(o orderedMapSlice) map[string]interface{} {
	m := make(map[string]interface{}, len(o))
	for _, pair := range o {
		switch v := pair.Value.(type) {
		case orderedMapSlice:
			m[pair.Key] = orderedMapToMap(v)
		default:
			m[pair.Key] = pair.Value
		}
	}
	return m
}
