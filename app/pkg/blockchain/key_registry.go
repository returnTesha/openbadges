package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"time"
)

// ── KeyRegistryReader implementation ───────────────────────────────────────

// GetActiveKey returns the currently active Ed25519 public key and its activation time.
func (c *Client) GetActiveKey(ctx context.Context) ([32]byte, time.Time, error) {
	var result []interface{}
	err := c.keyRegistryBound.Call(callOpts(ctx), &result, "getActiveKey")
	if err != nil {
		return [32]byte{}, time.Time{}, fmt.Errorf("blockchain: GetActiveKey failed: %w", err)
	}

	if len(result) < 2 {
		return [32]byte{}, time.Time{}, ErrNoActiveKey
	}

	pubKey, ok := result[0].([32]byte)
	if !ok {
		return [32]byte{}, time.Time{}, fmt.Errorf("blockchain: GetActiveKey unexpected publicKey type %T", result[0])
	}

	activatedAt, ok := result[1].(*big.Int)
	if !ok {
		return [32]byte{}, time.Time{}, fmt.Errorf("blockchain: GetActiveKey unexpected activatedAt type %T", result[1])
	}

	return pubKey, bigIntToTime(activatedAt), nil
}

// GetKeyAtTime returns the public key that was active at a specific point in time.
func (c *Client) GetKeyAtTime(ctx context.Context, t time.Time) ([32]byte, time.Time, error) {
	timestamp := big.NewInt(t.Unix())

	var result []interface{}
	err := c.keyRegistryBound.Call(callOpts(ctx), &result, "getKeyAtTime", timestamp)
	if err != nil {
		return [32]byte{}, time.Time{}, fmt.Errorf("blockchain: GetKeyAtTime failed: %w", err)
	}

	if len(result) < 2 {
		return [32]byte{}, time.Time{}, ErrKeyNotFound
	}

	pubKey, ok := result[0].([32]byte)
	if !ok {
		return [32]byte{}, time.Time{}, fmt.Errorf("blockchain: GetKeyAtTime unexpected publicKey type %T", result[0])
	}

	activatedAt, ok := result[1].(*big.Int)
	if !ok {
		return [32]byte{}, time.Time{}, fmt.Errorf("blockchain: GetKeyAtTime unexpected activatedAt type %T", result[1])
	}

	return pubKey, bigIntToTime(activatedAt), nil
}

// keyHistoryRaw is the anonymous struct matching the Solidity KeyEntry tuple.
// Field order and types must match the ABI exactly.
type keyHistoryRaw struct {
	PublicKey    [32]byte
	ActivatedAt *big.Int
	RevokedAt   *big.Int
	RevokeReason string
}

// GetKeyHistory returns the full key rotation history.
func (c *Client) GetKeyHistory(ctx context.Context) ([]KeyEntry, error) {
	var result []interface{}
	err := c.keyRegistryBound.Call(callOpts(ctx), &result, "getKeyHistory")
	if err != nil {
		return nil, fmt.Errorf("blockchain: GetKeyHistory failed: %w", err)
	}

	if len(result) < 1 {
		return nil, nil
	}

	rawEntries, ok := result[0].([]struct {
		PublicKey    [32]byte
		ActivatedAt *big.Int
		RevokedAt   *big.Int
		RevokeReason string
	})
	if !ok {
		return nil, fmt.Errorf("blockchain: GetKeyHistory unexpected result type %T", result[0])
	}

	entries := make([]KeyEntry, len(rawEntries))
	for i, raw := range rawEntries {
		entries[i] = KeyEntry{
			PublicKey:    raw.PublicKey,
			ActivatedAt: bigIntToTime(raw.ActivatedAt),
			RevokedAt:   bigIntToTime(raw.RevokedAt),
			RevokeReason: raw.RevokeReason,
		}
	}

	return entries, nil
}
