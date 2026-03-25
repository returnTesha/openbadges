package blockchain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
)

// ── BadgeRegistryReader implementation ─────────────────────────────────────

// VerifyHash checks whether a badge file's SHA-256 hash matches the on-chain record.
func (c *Client) VerifyHash(ctx context.Context, credentialID string, badgeHash [32]byte) (*HashVerification, error) {
	var result []interface{}
	err := c.badgeRegistryBound.Call(callOpts(ctx), &result, "verifyHash", credentialID, badgeHash)
	if err != nil {
		return nil, fmt.Errorf("blockchain: VerifyHash failed: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("blockchain: VerifyHash unexpected result length %d", len(result))
	}

	matches, ok := result[0].(bool)
	if !ok {
		return nil, fmt.Errorf("blockchain: VerifyHash unexpected matches type %T", result[0])
	}

	issuedAt, ok := result[1].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("blockchain: VerifyHash unexpected issuedAt type %T", result[1])
	}

	return &HashVerification{
		Matches:  matches,
		IssuedAt: bigIntToTime(issuedAt),
	}, nil
}

// GetBadgeStatus returns the full issuance and revocation status of a credential.
func (c *Client) GetBadgeStatus(ctx context.Context, credentialID string) (*BadgeStatus, error) {
	var result []interface{}
	err := c.badgeRegistryBound.Call(callOpts(ctx), &result, "getBadgeStatus", credentialID)
	if err != nil {
		return nil, fmt.Errorf("blockchain: GetBadgeStatus failed: %w", err)
	}

	if len(result) < 5 {
		return nil, fmt.Errorf("blockchain: GetBadgeStatus unexpected result length %d", len(result))
	}

	issued, ok := result[0].(bool)
	if !ok {
		return nil, fmt.Errorf("blockchain: GetBadgeStatus unexpected issued type %T", result[0])
	}

	revoked, ok := result[1].(bool)
	if !ok {
		return nil, fmt.Errorf("blockchain: GetBadgeStatus unexpected revoked type %T", result[1])
	}

	badgeHash, ok := result[2].([32]byte)
	if !ok {
		return nil, fmt.Errorf("blockchain: GetBadgeStatus unexpected badgeHash type %T", result[2])
	}

	issuedAt, ok := result[3].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("blockchain: GetBadgeStatus unexpected issuedAt type %T", result[3])
	}

	revokedAt, ok := result[4].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("blockchain: GetBadgeStatus unexpected revokedAt type %T", result[4])
	}

	return &BadgeStatus{
		Issued:    issued,
		Revoked:   revoked,
		BadgeHash: badgeHash,
		IssuedAt:  bigIntToTime(issuedAt),
		RevokedAt: bigIntToTime(revokedAt),
	}, nil
}

// IsRevoked checks whether a credential has been revoked.
func (c *Client) IsRevoked(ctx context.Context, credentialID string) (bool, error) {
	var result []interface{}
	err := c.badgeRegistryBound.Call(callOpts(ctx), &result, "isRevoked", credentialID)
	if err != nil {
		return false, fmt.Errorf("blockchain: IsRevoked failed: %w", err)
	}

	if len(result) < 1 {
		return false, fmt.Errorf("blockchain: IsRevoked unexpected result length %d", len(result))
	}

	revoked, ok := result[0].(bool)
	if !ok {
		return false, fmt.Errorf("blockchain: IsRevoked unexpected type %T", result[0])
	}

	return revoked, nil
}

// ── BadgeRegistryWriter implementation ─────────────────────────────────────

// RecordIssuance anchors a badge's SHA-256 hash on-chain.
// Returns the transaction hash on success.
func (c *Client) RecordIssuance(ctx context.Context, credentialID string, badgeHash [32]byte) (string, error) {
	opts, err := c.ensureTxOpts(ctx)
	if err != nil {
		return "", err
	}

	tx, err := c.badgeRegistryBound.Transact(opts, "recordIssuance", credentialID, badgeHash)
	if err != nil {
		return "", fmt.Errorf("blockchain: RecordIssuance tx failed: %w", err)
	}

	if err := c.waitForReceipt(ctx, tx, "RecordIssuance"); err != nil {
		return tx.Hash().Hex(), err
	}

	return tx.Hash().Hex(), nil
}

// RevokeBadge revokes a previously recorded credential with a reason.
// Returns the transaction hash on success.
func (c *Client) RevokeBadge(ctx context.Context, credentialID string, reason string) (string, error) {
	opts, err := c.ensureTxOpts(ctx)
	if err != nil {
		return "", err
	}

	tx, err := c.badgeRegistryBound.Transact(opts, "revokeBadge", credentialID, reason)
	if err != nil {
		return "", fmt.Errorf("blockchain: RevokeBadge tx failed: %w", err)
	}

	if err := c.waitForReceipt(ctx, tx, "RevokeBadge"); err != nil {
		return tx.Hash().Hex(), err
	}

	return tx.Hash().Hex(), nil
}

// waitForReceipt waits for a transaction to be mined and checks its status.
func (c *Client) waitForReceipt(ctx context.Context, tx *types.Transaction, opName string) error {
	receipt, err := bind.WaitMined(ctx, c.ethClient, tx)
	if err != nil {
		return fmt.Errorf("blockchain: %s wait for receipt failed: %w", opName, err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("blockchain: %s transaction reverted (tx=%s)", opName, tx.Hash().Hex())
	}

	return nil
}
