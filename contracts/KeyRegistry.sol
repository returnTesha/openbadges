// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title KeyRegistry
 * @notice On-chain public key registry for The Badge Service (did:web).
 *         Serves as a fallback when the issuer's domain is unreachable,
 *         enabling permanent badge verification via Polygon.
 * @dev Stores Ed25519 public keys (32 bytes) with full rotation history
 *      so verifiers can retrieve the key that was active at any point in time.
 *      Initial DID and public key are registered at deployment via constructor.
 */
contract KeyRegistry {

    // ──────────────────────────────────────────────
    // Data Structures
    // ──────────────────────────────────────────────

    struct KeyEntry {
        bytes32 publicKey;      // Ed25519 public key (32 bytes)
        uint256 activatedAt;    // timestamp when the key became active
        uint256 revokedAt;      // 0 = still active, >0 = revocation timestamp
        string  revokeReason;   // reason for revocation (empty if active)
    }

    // ──────────────────────────────────────────────
    // State
    // ──────────────────────────────────────────────

    /// @dev The DID registered at deployment (e.g. "did:web:thebadge.kr")
    string public registeredDid;

    /// @dev Ordered list of key entries (append-only)
    KeyEntry[] private _keyHistory;

    /// @dev Ethereum address that manages this registry (deployer)
    address public owner;

    // ──────────────────────────────────────────────
    // Events
    // ──────────────────────────────────────────────

    event KeyRegistered(string indexed did, bytes32 publicKey, uint256 timestamp);
    event KeyRotated(string indexed did, bytes32 oldKey, bytes32 newKey, uint256 timestamp);
    event KeyRevoked(string indexed did, bytes32 publicKey, string reason, uint256 timestamp);
    event OwnershipTransferred(address oldOwner, address newOwner);

    // ──────────────────────────────────────────────
    // Modifiers
    // ──────────────────────────────────────────────

    modifier onlyOwner() {
        require(msg.sender == owner, "KeyRegistry: caller is not the owner");
        _;
    }

    // ──────────────────────────────────────────────
    // Constructor — 배포 시 초기 공개키 등록
    // ──────────────────────────────────────────────

    /**
     * @notice Deploy the registry with the initial DID and Ed25519 public key.
     * @param did       The DID string (e.g. "did:web:thebadge.kr")
     * @param publicKey The initial Ed25519 public key (32 bytes)
     */
    constructor(string memory did, bytes32 publicKey) {
        require(bytes(did).length > 0, "KeyRegistry: empty DID");
        require(publicKey != bytes32(0), "KeyRegistry: zero key not allowed");

        registeredDid = did;
        owner = msg.sender;

        _keyHistory.push(KeyEntry({
            publicKey: publicKey,
            activatedAt: block.timestamp,
            revokedAt: 0,
            revokeReason: ""
        }));

        emit KeyRegistered(did, publicKey, block.timestamp);
    }

    // ──────────────────────────────────────────────
    // Key Management
    // ──────────────────────────────────────────────

    /**
     * @notice Rotate the active key: revoke the current key and activate a new one.
     * @param newPublicKey The new Ed25519 public key
     */
    function rotateKey(bytes32 newPublicKey) external onlyOwner {
        require(newPublicKey != bytes32(0), "KeyRegistry: zero key not allowed");

        uint256 len = _keyHistory.length;
        require(len > 0, "KeyRegistry: no key history");

        KeyEntry storage current = _keyHistory[len - 1];
        require(current.revokedAt == 0, "KeyRegistry: no active key to rotate");

        bytes32 oldKey = current.publicKey;
        current.revokedAt = block.timestamp;

        _keyHistory.push(KeyEntry({
            publicKey: newPublicKey,
            activatedAt: block.timestamp,
            revokedAt: 0,
            revokeReason: ""
        }));

        emit KeyRotated(registeredDid, oldKey, newPublicKey, block.timestamp);
    }

    /**
     * @notice Emergency-revoke the current active key without providing a replacement.
     * @dev After this call there is no active key. Call `addKey` to register a new one.
     * @param reason Human-readable reason for revocation
     */
    function revokeCurrentKey(string calldata reason) external onlyOwner {
        uint256 len = _keyHistory.length;
        require(len > 0, "KeyRegistry: no key history");

        KeyEntry storage current = _keyHistory[len - 1];
        require(current.revokedAt == 0, "KeyRegistry: no active key to revoke");

        current.revokedAt = block.timestamp;
        current.revokeReason = reason;

        emit KeyRevoked(registeredDid, current.publicKey, reason, block.timestamp);
    }

    /**
     * @notice Add a new key after emergency revocation (when no active key exists).
     * @param newPublicKey The new Ed25519 public key
     */
    function addKey(bytes32 newPublicKey) external onlyOwner {
        require(newPublicKey != bytes32(0), "KeyRegistry: zero key not allowed");

        uint256 len = _keyHistory.length;
        if (len > 0) {
            require(_keyHistory[len - 1].revokedAt != 0, "KeyRegistry: active key exists, use rotateKey");
        }

        _keyHistory.push(KeyEntry({
            publicKey: newPublicKey,
            activatedAt: block.timestamp,
            revokedAt: 0,
            revokeReason: ""
        }));

        emit KeyRegistered(registeredDid, newPublicKey, block.timestamp);
    }

    // ──────────────────────────────────────────────
    // Query Functions (for verifiers)
    // ──────────────────────────────────────────────

    /**
     * @notice Get the currently active public key.
     * @return publicKey   The active Ed25519 public key
     * @return activatedAt Timestamp when the key was activated
     */
    function getActiveKey()
        external
        view
        returns (bytes32 publicKey, uint256 activatedAt)
    {
        uint256 len = _keyHistory.length;
        require(len > 0, "KeyRegistry: no key history");

        KeyEntry storage latest = _keyHistory[len - 1];
        require(latest.revokedAt == 0, "KeyRegistry: no active key");

        return (latest.publicKey, latest.activatedAt);
    }

    /**
     * @notice Get the key that was active at a specific timestamp.
     * @dev Critical for verifying badges that were issued in the past.
     * @param timestamp The point in time to query
     * @return publicKey   The Ed25519 public key that was active at `timestamp`
     * @return activatedAt Timestamp when that key was activated
     */
    function getKeyAtTime(uint256 timestamp)
        external
        view
        returns (bytes32 publicKey, uint256 activatedAt)
    {
        uint256 len = _keyHistory.length;

        for (uint256 i = len; i > 0; ) {
            unchecked { --i; }
            KeyEntry storage entry = _keyHistory[i];

            if (entry.activatedAt <= timestamp) {
                if (entry.revokedAt == 0 || entry.revokedAt > timestamp) {
                    return (entry.publicKey, entry.activatedAt);
                }
            }
        }

        revert("KeyRegistry: no key found for the given timestamp");
    }

    /**
     * @notice Get the full key rotation history.
     * @return An array of all KeyEntry records in chronological order
     */
    function getKeyHistory() external view returns (KeyEntry[] memory) {
        return _keyHistory;
    }

    /**
     * @notice Get the number of keys in the history.
     * @return The count of key entries (including revoked ones)
     */
    function getKeyCount() external view returns (uint256) {
        return _keyHistory.length;
    }

    // ──────────────────────────────────────────────
    // Ownership
    // ──────────────────────────────────────────────

    /**
     * @notice Transfer ownership to a new Ethereum address.
     * @param newOwner The address of the new owner
     */
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "KeyRegistry: new owner is zero address");

        address oldOwner = owner;
        owner = newOwner;

        emit OwnershipTransferred(oldOwner, newOwner);
    }
}
