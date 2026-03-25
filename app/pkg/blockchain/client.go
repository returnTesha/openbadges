// Package blockchain provides Go clients for the Polygon smart contracts
// used by The Badge Project: KeyRegistry (public key anchoring) and
// BadgeRegistry (badge hash anchoring + revocation).
package blockchain

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ── Embedded ABI JSON ──────────────────────────────────────────────────────

//go:embed abi/KeyRegistry.json
var keyRegistryABIJSON []byte

//go:embed abi/BadgeRegistry.json
var badgeRegistryABIJSON []byte

// ── Sentinel errors ────────────────────────────────────────────────────────

var (
	ErrNoActiveKey  = errors.New("blockchain: no active key in registry")
	ErrKeyNotFound  = errors.New("blockchain: no key found for the given timestamp")
	ErrNotIssued    = errors.New("blockchain: credential not recorded on-chain")
	ErrNoPrivateKey = errors.New("blockchain: deployer private key required for write operations")
)

// ── Domain types ───────────────────────────────────────────────────────────

// KeyEntry represents a single key rotation entry from KeyRegistry.
type KeyEntry struct {
	PublicKey    [32]byte
	ActivatedAt time.Time
	RevokedAt   time.Time // zero value = still active
	RevokeReason string
}

// BadgeStatus represents the full on-chain status of a badge credential.
type BadgeStatus struct {
	Issued    bool
	Revoked   bool
	BadgeHash [32]byte
	IssuedAt  time.Time // zero value if not issued
	RevokedAt time.Time // zero value if not revoked
}

// HashVerification holds the result of an on-chain hash comparison.
type HashVerification struct {
	Matches  bool
	IssuedAt time.Time
}

// ── Interfaces ─────────────────────────────────────────────────────────────

// KeyRegistryReader provides read-only access to the KeyRegistry contract.
type KeyRegistryReader interface {
	GetActiveKey(ctx context.Context) ([32]byte, time.Time, error)
	GetKeyAtTime(ctx context.Context, t time.Time) ([32]byte, time.Time, error)
	GetKeyHistory(ctx context.Context) ([]KeyEntry, error)
}

// BadgeRegistryReader provides read-only access to the BadgeRegistry contract.
type BadgeRegistryReader interface {
	VerifyHash(ctx context.Context, credentialID string, badgeHash [32]byte) (*HashVerification, error)
	GetBadgeStatus(ctx context.Context, credentialID string) (*BadgeStatus, error)
	IsRevoked(ctx context.Context, credentialID string) (bool, error)
}

// BadgeRegistryWriter provides write access to the BadgeRegistry contract.
// Write methods return the transaction hash on success.
type BadgeRegistryWriter interface {
	RecordIssuance(ctx context.Context, credentialID string, badgeHash [32]byte) (txHash string, err error)
	RevokeBadge(ctx context.Context, credentialID string, reason string) (txHash string, err error)
}

// ── Compile-time interface checks ──────────────────────────────────────────

var (
	_ KeyRegistryReader   = (*Client)(nil)
	_ BadgeRegistryReader = (*Client)(nil)
	_ BadgeRegistryWriter = (*Client)(nil)
)

// ── Client ─────────────────────────────────────────────────────────────────

// Client is the concrete implementation of all blockchain interfaces.
type Client struct {
	ethClient *ethclient.Client
	chainID   *big.Int
	txOpts    *bind.TransactOpts // nil for read-only clients

	keyRegistryAddr   common.Address
	keyRegistryABI    abi.ABI
	keyRegistryBound  *bind.BoundContract

	badgeRegistryAddr  common.Address
	badgeRegistryABI   abi.ABI
	badgeRegistryBound *bind.BoundContract
}

// NewClient creates a blockchain client connected to the given RPC endpoint.
// For read-only usage, pass an empty string for deployerPrivateKey.
// For write operations, pass the hex-encoded Ethereum private key (without 0x prefix).
func NewClient(ctx context.Context, rpcURL, keyRegistryAddr, badgeRegistryAddr, deployerPrivateKey string) (*Client, error) {
	eth, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("blockchain: failed to connect to RPC %s: %w", rpcURL, err)
	}

	chainID, err := eth.ChainID(ctx)
	if err != nil {
		eth.Close()
		return nil, fmt.Errorf("blockchain: failed to get chain ID: %w", err)
	}

	keyABI, err := abi.JSON(bytes.NewReader(keyRegistryABIJSON))
	if err != nil {
		eth.Close()
		return nil, fmt.Errorf("blockchain: failed to parse KeyRegistry ABI: %w", err)
	}

	badgeABI, err := abi.JSON(bytes.NewReader(badgeRegistryABIJSON))
	if err != nil {
		eth.Close()
		return nil, fmt.Errorf("blockchain: failed to parse BadgeRegistry ABI: %w", err)
	}

	keyAddr := common.HexToAddress(keyRegistryAddr)
	badgeAddr := common.HexToAddress(badgeRegistryAddr)

	c := &Client{
		ethClient:         eth,
		chainID:           chainID,
		keyRegistryAddr:   keyAddr,
		keyRegistryABI:    keyABI,
		keyRegistryBound:  bind.NewBoundContract(keyAddr, keyABI, eth, eth, eth),
		badgeRegistryAddr: badgeAddr,
		badgeRegistryABI:  badgeABI,
		badgeRegistryBound: bind.NewBoundContract(badgeAddr, badgeABI, eth, eth, eth),
	}

	if deployerPrivateKey != "" {
		if err := c.initTransactor(deployerPrivateKey); err != nil {
			eth.Close()
			return nil, err
		}
	}

	return c, nil
}

// Close releases the underlying RPC connection.
func (c *Client) Close() {
	c.ethClient.Close()
}

// initTransactor sets up the TransactOpts from a hex-encoded ECDSA private key.
func (c *Client) initTransactor(hexKey string) error {
	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return fmt.Errorf("blockchain: invalid deployer private key: %w", err)
	}

	opts, err := bind.NewKeyedTransactorWithChainID(key, c.chainID)
	if err != nil {
		return fmt.Errorf("blockchain: failed to create transactor: %w", err)
	}

	c.txOpts = opts
	return nil
}

// callOpts returns CallOpts with the given context.
func callOpts(ctx context.Context) *bind.CallOpts {
	return &bind.CallOpts{Context: ctx}
}

// bigIntToTime converts a *big.Int unix timestamp to time.Time.
// Returns zero time if the value is 0.
func bigIntToTime(v *big.Int) time.Time {
	if v == nil || v.Sign() == 0 {
		return time.Time{}
	}
	return time.Unix(v.Int64(), 0)
}

// ensureTxOpts returns a copy of the stored TransactOpts with the given context,
// or ErrNoPrivateKey if no key was configured.
func (c *Client) ensureTxOpts(ctx context.Context) (*bind.TransactOpts, error) {
	if c.txOpts == nil {
		return nil, ErrNoPrivateKey
	}
	// Copy so concurrent callers don't race on the Context field.
	cpy := *c.txOpts
	cpy.Context = ctx
	// Derive public address from the stored key for nonce management.
	cpy.From = c.txOpts.From
	return &cpy, nil
}

// PublicAddressFromKey extracts the Ethereum address from the deployer private key.
// This is exported for diagnostics only; callers should not need it in normal flow.
func PublicAddressFromKey(hexKey string) (common.Address, error) {
	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return common.Address{}, fmt.Errorf("blockchain: invalid private key: %w", err)
	}
	pub, ok := key.Public().(*ecdsa.PublicKey)
	if !ok {
		return common.Address{}, errors.New("blockchain: failed to derive public key")
	}
	return crypto.PubkeyToAddress(*pub), nil
}
