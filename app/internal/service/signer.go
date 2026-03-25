// Package service contains domain services for badge signing, DID resolution, etc.

// signer.go — Ed25519 signing service (eddsa-rdfc-2022).
// Implements key management, signing, and verification using Go's crypto/ed25519.
package service

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"math/big"
)

// Signer signs and verifies badge credential payloads using Ed25519.
type Signer interface {
	// Sign canonicalises the payload and returns a detached Ed25519 signature.
	Sign(payload []byte) (signature []byte, err error)
	// Verify checks that signature is valid for the given payload.
	Verify(payload, signature []byte) (bool, error)
}

// SignerService holds an Ed25519 key pair and provides signing/verification.
type SignerService struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	keyID      string // verification method ID for proof, e.g. "did:web:example.com#key-1"
}

// NewSignerService creates a SignerService from a raw Ed25519 private key (64 bytes)
// and a key ID string used as the verificationMethod in DataIntegrityProofs.
func NewSignerService(privateKeyBytes []byte, keyID string) (*SignerService, error) {
	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid Ed25519 private key length: got %d, want %d", len(privateKeyBytes), ed25519.PrivateKeySize)
	}

	priv := ed25519.PrivateKey(privateKeyBytes)
	pub := priv.Public().(ed25519.PublicKey)

	return &SignerService{
		privateKey: priv,
		publicKey:  pub,
		keyID:      keyID,
	}, nil
}

// Sign produces an Ed25519 signature over the given data.
// The caller is responsible for canonicalization and hashing before calling this.
func (s *SignerService) Sign(data []byte) ([]byte, error) {
	if s.privateKey == nil {
		return nil, errors.New("signer: private key not initialized")
	}
	sig := ed25519.Sign(s.privateKey, data)
	return sig, nil
}

// Verify checks an Ed25519 signature against the public key.
func (s *SignerService) Verify(data []byte, signature []byte) bool {
	if s.publicKey == nil {
		return false
	}
	return ed25519.Verify(s.publicKey, data, signature)
}

// PublicKeyBytes returns the raw 32-byte Ed25519 public key.
func (s *SignerService) PublicKeyBytes() []byte {
	return []byte(s.publicKey)
}

// PublicKeyMultibase returns the public key encoded in multibase format
// (z + base58btc), as required by the eddsa-rdfc-2022 cryptosuite.
// The multicodec prefix 0xed01 is prepended to identify Ed25519 public keys.
func (s *SignerService) PublicKeyMultibase() string {
	// Multicodec prefix for Ed25519 public key: 0xed 0x01
	prefixed := append([]byte{0xed, 0x01}, s.publicKey...)
	return "z" + base58Encode(prefixed)
}

// KeyID returns the verification method ID (e.g. "did:web:example.com#key-1").
func (s *SignerService) KeyID() string {
	return s.keyID
}

// GenerateKeyPair creates a new Ed25519 key pair for development/testing.
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}
	return pub, priv, nil
}

// --- base58btc encoder (Bitcoin alphabet) ---
// Minimal implementation sufficient for encoding Ed25519 public keys and signatures.

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58Encode encodes a byte slice to base58btc (Bitcoin alphabet).
func base58Encode(input []byte) string {
	if len(input) == 0 {
		return ""
	}

	// Count leading zero bytes — each maps to a '1' in base58.
	leadingZeros := 0
	for _, b := range input {
		if b != 0 {
			break
		}
		leadingZeros++
	}

	// Convert byte slice to a big integer and repeatedly divide by 58.
	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	mod := new(big.Int)
	var encoded []byte

	for x.Sign() > 0 {
		x.DivMod(x, base, mod)
		encoded = append(encoded, base58Alphabet[mod.Int64()])
	}

	// Append leading '1' characters.
	for i := 0; i < leadingZeros; i++ {
		encoded = append(encoded, base58Alphabet[0])
	}

	// Reverse the result (big-endian).
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}

	return string(encoded)
}
