// did_resolver.go — DID resolution service.
// Uses did:web as the primary method with Polygon smart-contract fallback.
// Resolves a DID to its DID Document and extracts the Ed25519 public key.
package service

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DIDDocument is a minimal representation of a DID Document (W3C DID Core).
type DIDDocument struct {
	Context            []string             `json:"@context"`
	ID                 string               `json:"id"`
	VerificationMethod []VerificationMethod `json:"verificationMethod,omitempty"`
	AssertionMethod    []string             `json:"assertionMethod,omitempty"`
	Service            []DIDService         `json:"service,omitempty"`
}

// DIDService represents a service endpoint in a DID Document.
type DIDService struct {
	ID              string      `json:"id"`
	Type            string      `json:"type"`
	ServiceEndpoint interface{} `json:"serviceEndpoint"`
}

// VerificationMethod holds a single public key entry.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultibase string `json:"publicKeyMultibase,omitempty"`
}

// DIDResolver resolves a DID string to its DID Document.
type DIDResolver interface {
	Resolve(ctx context.Context, did string) (*DIDDocument, error)
}

// KeyRegistryReader provides read-only access to the KeyRegistry contract
// for DID fallback resolution. This interface is satisfied by
// pkg/blockchain.Client.
type KeyRegistryReader interface {
	GetActiveKey(ctx context.Context) ([32]byte, time.Time, error)
	GetKeyAtTime(ctx context.Context, t time.Time) ([32]byte, time.Time, error)
}

// DIDResolverService implements DID resolution via did:web with Polygon fallback.
type DIDResolverService struct {
	httpClient   *http.Client
	polygonRPC   string
	contractAddr string
	keyRegistry  KeyRegistryReader // nil = fallback disabled
	localDoc     *DIDDocument      // if set, returned for matching DID without HTTP
}

// NewDIDResolver creates a new DIDResolverService. The keyRegistry parameter
// is optional — pass nil to disable Polygon fallback resolution.
func NewDIDResolver(polygonRPC, contractAddr string, keyRegistry KeyRegistryReader) *DIDResolverService {
	return &DIDResolverService{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		polygonRPC:   polygonRPC,
		contractAddr: contractAddr,
		keyRegistry:  keyRegistry,
	}
}

// SetLocalDIDDocument configures a local DID document so the resolver can
// resolve the server's own DID without making an HTTP request. This is
// essential for local development where did:web DNS resolution would fail.
func (r *DIDResolverService) SetLocalDIDDocument(doc *DIDDocument) {
	r.localDoc = doc
}

// Resolve resolves a did:web DID to its DID Document.
//
// Resolution steps:
//  1. Parse DID method — only did:web is supported.
//  2. Convert did:web identifier to an HTTPS URL for the DID document.
//  3. HTTP GET the DID document from the resolved URL.
//  4. Parse and validate the JSON response.
//  5. Extract public key from verificationMethod.
//  6. Return the DIDDocument.
func (r *DIDResolverService) Resolve(ctx context.Context, did string) (*DIDDocument, error) {
	// Step 0: If a local DID document is configured and matches, return it
	// directly without making an HTTP request. This enables local development
	// where did:web DNS resolution would fail.
	if r.localDoc != nil && r.localDoc.ID == did {
		return r.localDoc, nil
	}

	// Step 1: Parse DID method — only did:web is supported.
	if !strings.HasPrefix(did, "did:web:") {
		return nil, fmt.Errorf("unsupported DID method: only did:web is supported, got %q", did)
	}

	// Step 2: Convert did:web to HTTPS URL.
	// did:web:example.com → https://example.com/.well-known/did.json
	// did:web:example.com:path:to → https://example.com/path/to/did.json
	url, err := didWebToURL(did)
	if err != nil {
		return nil, fmt.Errorf("failed to convert DID to URL: %w", err)
	}

	// Step 3: HTTP GET the DID document.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Accept", "application/did+json, application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch DID document from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DID document fetch returned HTTP %d from %s", resp.StatusCode, url)
	}

	// Step 4: Parse and validate the response.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read DID document body: %w", err)
	}

	var doc DIDDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse DID document JSON: %w", err)
	}

	// Validate that the document ID matches the requested DID.
	if doc.ID != did {
		return nil, fmt.Errorf("DID document ID mismatch: expected %q, got %q", did, doc.ID)
	}

	// Step 5: Verify at least one verificationMethod is present.
	if len(doc.VerificationMethod) == 0 {
		return nil, fmt.Errorf("DID document has no verificationMethod entries")
	}

	// Step 6: Return the parsed document.
	return &doc, nil
}

// ResolveFallback attempts to resolve a DID document via the Polygon smart
// contract. This serves as a fallback when the did:web endpoint is unavailable
// (e.g., the issuer's domain is no longer operational).
//
// It queries the KeyRegistry contract for the public key that was active at
// the badge's issuance time and constructs a synthetic DID Document.
func (r *DIDResolverService) ResolveFallback(ctx context.Context, did string, contractAddr string, issuedAt time.Time) (*DIDDocument, error) {
	if r.keyRegistry == nil {
		return nil, fmt.Errorf("polygon fallback not available: no KeyRegistry client configured")
	}

	// Query the public key that was active at issuedAt from the KeyRegistry contract.
	pubKeyBytes, _, err := r.keyRegistry.GetKeyAtTime(ctx, issuedAt)
	if err != nil {
		return nil, fmt.Errorf("polygon fallback: failed to get key at time %v: %w", issuedAt, err)
	}

	// Encode the raw 32-byte Ed25519 public key as multibase (z + base58btc)
	// with the multicodec prefix 0xed01 for Ed25519.
	prefixed := append([]byte{0xed, 0x01}, pubKeyBytes[:]...)
	multibase := "z" + base58Encode(prefixed)

	// Construct a synthetic DID Document that matches the original DID and key
	// ID format so the verifier can find the key when matching proof.verificationMethod.
	keyID := did + "#key-1"

	doc := &DIDDocument{
		Context: []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/multikey/v1",
		},
		ID: did,
		VerificationMethod: []VerificationMethod{
			{
				ID:                 keyID,
				Type:               "Multikey",
				Controller:         did,
				PublicKeyMultibase: multibase,
			},
		},
		AssertionMethod: []string{keyID},
	}

	return doc, nil
}

// ResolveWithFallback tries did:web resolution first, and falls back to the
// Polygon smart contract if the primary method fails.
func (r *DIDResolverService) ResolveWithFallback(ctx context.Context, did string, fallbackAddr string, issuedAt time.Time) (*DIDDocument, error) {
	// Step 1: Try primary did:web resolution.
	doc, err := r.Resolve(ctx, did)
	if err == nil {
		return doc, nil
	}

	// Step 2: Primary failed — attempt Polygon fallback if a contract address
	// is provided.
	if fallbackAddr == "" {
		return nil, fmt.Errorf("did:web resolution failed and no fallback address provided: %w", err)
	}

	fallbackDoc, fallbackErr := r.ResolveFallback(ctx, did, fallbackAddr, issuedAt)
	if fallbackErr != nil {
		return nil, fmt.Errorf("both did:web and polygon fallback failed: web=%v, polygon=%v", err, fallbackErr)
	}

	return fallbackDoc, nil
}

// didWebToURL converts a did:web identifier to the corresponding HTTPS URL
// where the DID document can be fetched.
//
// Examples:
//
//	did:web:example.com            → https://example.com/.well-known/did.json
//	did:web:example.com:path:to    → https://example.com/path/to/did.json
//	did:web:example.com%3A8080     → https://example.com:8080/.well-known/did.json
func didWebToURL(did string) (string, error) {
	// Remove the "did:web:" prefix.
	identifier := strings.TrimPrefix(did, "did:web:")
	if identifier == "" {
		return "", fmt.Errorf("empty did:web identifier")
	}

	// Split by ":" to separate the domain from optional path segments.
	parts := strings.Split(identifier, ":")

	// The first part is the domain (with percent-encoded port if present).
	// Decode %3A back to : for the domain portion.
	domain := strings.ReplaceAll(parts[0], "%3A", ":")

	if len(parts) == 1 {
		// No path segments: use .well-known location.
		return "https://" + domain + "/.well-known/did.json", nil
	}

	// With path segments: join remaining parts with "/" and append did.json.
	path := strings.Join(parts[1:], "/")
	return "https://" + domain + "/" + path + "/did.json", nil
}

// DecodePublicKeyMultibase decodes a multibase-encoded public key string
// into an Ed25519 public key. The expected format is "z" prefix followed by
// base58btc-encoded bytes (the standard multibase encoding for Ed25519 keys).
func DecodePublicKeyMultibase(multibase string) (ed25519.PublicKey, error) {
	if len(multibase) == 0 {
		return nil, errors.New("empty multibase string")
	}

	// Check for the "z" prefix which indicates base58btc encoding.
	if multibase[0] != 'z' {
		return nil, fmt.Errorf("unsupported multibase prefix %q: expected 'z' (base58btc)", string(multibase[0]))
	}

	// Decode the base58btc payload (everything after the "z" prefix).
	decoded, err := base58Decode(multibase[1:])
	if err != nil {
		return nil, fmt.Errorf("base58btc decode failed: %w", err)
	}

	// The decoded bytes may include a multicodec prefix for Ed25519 public key:
	// 0xed 0x01 (two bytes). If present, strip it.
	if len(decoded) == ed25519.PublicKeySize+2 && decoded[0] == 0xed && decoded[1] == 0x01 {
		decoded = decoded[2:]
	}

	// Validate the key length.
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 public key length: got %d bytes, expected %d", len(decoded), ed25519.PublicKeySize)
	}

	return ed25519.PublicKey(decoded), nil
}

// base58Decode decodes a base58btc-encoded string (Bitcoin alphabet).
// This is a self-contained implementation to avoid external dependencies.
func base58Decode(input string) ([]byte, error) {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// Build reverse lookup table.
	var lookup [256]int
	for i := range lookup {
		lookup[i] = -1
	}
	for i, c := range alphabet {
		lookup[c] = i
	}

	// Count leading '1's (which map to zero bytes).
	leadingZeros := 0
	for _, c := range input {
		if c == '1' {
			leadingZeros++
		} else {
			break
		}
	}

	// Decode the base58 string into a big-endian byte slice.
	// Work with a sufficiently large intermediate buffer.
	size := len(input) * 733 / 1000 // log(58) / log(256), rounded up
	buf := make([]byte, size+1)

	for _, c := range input {
		carry := lookup[c]
		if carry < 0 {
			return nil, fmt.Errorf("invalid base58 character: %c", c)
		}
		for j := len(buf) - 1; j >= 0; j-- {
			carry += 58 * int(buf[j])
			buf[j] = byte(carry % 256)
			carry /= 256
		}
		if carry != 0 {
			return nil, errors.New("base58 decode: value overflow")
		}
	}

	// Skip leading zeros in the buffer.
	start := 0
	for start < len(buf) && buf[start] == 0 {
		start++
	}

	// Prepend the leading zero bytes and return.
	result := make([]byte, leadingZeros+len(buf)-start)
	copy(result[leadingZeros:], buf[start:])
	return result, nil
}
