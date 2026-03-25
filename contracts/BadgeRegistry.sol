// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title BadgeRegistry
 * @notice On-chain registry for Open Badges 3.0 credentials.
 *         Handles two responsibilities:
 *         1. Badge issuance recording (credential hash anchoring)
 *         2. Badge revocation management
 * @dev Designed for Polygon deployment. The issuer DID is registered at
 *      deployment via constructor. Deployer becomes the owner.
 */
contract BadgeRegistry {

    // ──────────────────────────────────────────────
    // Data Structures
    // ──────────────────────────────────────────────

    struct IssuanceEntry {
        string  credentialId;   // e.g. "snu-0001"
        bytes32 badgeHash;      // SHA-256 hash of the signed badge JSON
        uint256 issuedAt;       // block.timestamp of recording
    }

    struct RevocationEntry {
        string  credentialId;   // e.g. "snu-0001"
        string  reason;         // human-readable revocation reason
        uint256 revokedAt;      // block.timestamp of revocation
    }

    // ──────────────────────────────────────────────
    // State
    // ──────────────────────────────────────────────

    /// @dev The issuer DID registered at deployment (e.g. "did:web:thebadge.kr")
    string public issuerDid;

    /// @dev Ethereum address that manages this registry (deployer)
    address public owner;

    /// @dev credentialId -> issuance details
    mapping(string => IssuanceEntry) private _issuances;

    /// @dev credentialId -> whether it has been recorded
    mapping(string => bool) private _isIssued;

    /// @dev list of all issued credential IDs
    string[] private _issuedBadges;

    /// @dev credentialId -> revocation details
    mapping(string => RevocationEntry) private _revocations;

    /// @dev credentialId -> whether it has been revoked
    mapping(string => bool) private _isRevoked;

    /// @dev list of all revoked credential IDs
    string[] private _revokedBadges;

    // ──────────────────────────────────────────────
    // Events
    // ──────────────────────────────────────────────

    event IssuerRegistered(string indexed issuerDid, address owner);
    event BadgeIssued(string indexed credentialId, bytes32 badgeHash, uint256 timestamp);
    event BatchIssued(uint256 count, uint256 timestamp);
    event BadgeRevoked(string indexed credentialId, string reason, uint256 timestamp);
    event BatchRevoked(uint256 count, uint256 timestamp);
    event OwnershipTransferred(address oldOwner, address newOwner);

    // ──────────────────────────────────────────────
    // Modifiers
    // ──────────────────────────────────────────────

    modifier onlyOwner() {
        require(msg.sender == owner, "BadgeRegistry: caller is not the owner");
        _;
    }

    // ──────────────────────────────────────────────
    // Constructor — 배포 시 issuer DID 등록
    // ──────────────────────────────────────────────

    /**
     * @notice Deploy the registry with the issuer DID.
     * @param _issuerDid The issuer's DID string (e.g. "did:web:thebadge.kr")
     */
    constructor(string memory _issuerDid) {
        require(bytes(_issuerDid).length > 0, "BadgeRegistry: empty DID");

        issuerDid = _issuerDid;
        owner = msg.sender;

        emit IssuerRegistered(_issuerDid, msg.sender);
    }

    // ──────────────────────────────────────────────
    // Badge Issuance Recording
    // ──────────────────────────────────────────────

    /**
     * @notice Record a badge issuance on-chain by storing its hash.
     * @param credentialId The credential identifier (e.g. "snu-0001")
     * @param badgeHash    SHA-256 hash of the complete signed badge JSON
     */
    function recordIssuance(string calldata credentialId, bytes32 badgeHash)
        external
        onlyOwner
    {
        require(!_isIssued[credentialId], "BadgeRegistry: credential already recorded");
        require(badgeHash != bytes32(0), "BadgeRegistry: zero hash not allowed");

        _isIssued[credentialId] = true;
        _issuances[credentialId] = IssuanceEntry({
            credentialId: credentialId,
            badgeHash: badgeHash,
            issuedAt: block.timestamp
        });
        _issuedBadges.push(credentialId);

        emit BadgeIssued(credentialId, badgeHash, block.timestamp);
    }

    /**
     * @notice Batch-record multiple badge issuances in a single transaction.
     * @param credentialIds Array of credential identifiers
     * @param badgeHashes   Array of SHA-256 hashes (must match credentialIds length)
     */
    function batchRecordIssuance(
        string[] calldata credentialIds,
        bytes32[] calldata badgeHashes
    )
        external
        onlyOwner
    {
        uint256 count = credentialIds.length;
        require(count > 0, "BadgeRegistry: empty batch");
        require(count == badgeHashes.length, "BadgeRegistry: array length mismatch");

        for (uint256 i = 0; i < count; ) {
            require(!_isIssued[credentialIds[i]], "BadgeRegistry: credential already recorded");
            require(badgeHashes[i] != bytes32(0), "BadgeRegistry: zero hash not allowed");

            _isIssued[credentialIds[i]] = true;
            _issuances[credentialIds[i]] = IssuanceEntry({
                credentialId: credentialIds[i],
                badgeHash: badgeHashes[i],
                issuedAt: block.timestamp
            });
            _issuedBadges.push(credentialIds[i]);

            emit BadgeIssued(credentialIds[i], badgeHashes[i], block.timestamp);

            unchecked { ++i; }
        }

        emit BatchIssued(count, block.timestamp);
    }

    // ──────────────────────────────────────────────
    // Badge Revocation
    // ──────────────────────────────────────────────

    /**
     * @notice Revoke a single badge credential.
     * @param credentialId The credential identifier
     * @param reason       Human-readable reason for revocation
     */
    function revokeBadge(string calldata credentialId, string calldata reason)
        external
        onlyOwner
    {
        require(_isIssued[credentialId], "BadgeRegistry: credential not recorded");
        _revokeSingle(credentialId, reason);

        emit BadgeRevoked(credentialId, reason, block.timestamp);
    }

    /**
     * @notice Batch-revoke multiple badge credentials in a single transaction.
     * @param credentialIds Array of credential identifiers to revoke
     * @param reason        Human-readable reason for revocation
     */
    function batchRevoke(string[] calldata credentialIds, string calldata reason)
        external
        onlyOwner
    {
        uint256 count = credentialIds.length;
        require(count > 0, "BadgeRegistry: empty batch");

        for (uint256 i = 0; i < count; ) {
            require(_isIssued[credentialIds[i]], "BadgeRegistry: credential not recorded");
            _revokeSingle(credentialIds[i], reason);

            emit BadgeRevoked(credentialIds[i], reason, block.timestamp);

            unchecked { ++i; }
        }

        emit BatchRevoked(count, block.timestamp);
    }

    // ──────────────────────────────────────────────
    // Query — Issuance
    // ──────────────────────────────────────────────

    /**
     * @notice Check whether a badge has been recorded on-chain.
     */
    function isIssued(string calldata credentialId) external view returns (bool) {
        return _isIssued[credentialId];
    }

    /**
     * @notice Get issuance details for a recorded badge.
     */
    function getIssuance(string calldata credentialId)
        external
        view
        returns (IssuanceEntry memory entry)
    {
        require(_isIssued[credentialId], "BadgeRegistry: credential not recorded");
        return _issuances[credentialId];
    }

    /**
     * @notice Verify a badge file's hash against the on-chain record.
     * @param credentialId The credential identifier
     * @param badgeHash    SHA-256 hash of the badge file to verify
     * @return matches True if the hash matches the on-chain record
     * @return issuedAt Timestamp of the original issuance (0 if not recorded)
     */
    function verifyHash(string calldata credentialId, bytes32 badgeHash)
        external
        view
        returns (bool matches, uint256 issuedAt)
    {
        if (!_isIssued[credentialId]) {
            return (false, 0);
        }
        IssuanceEntry storage entry = _issuances[credentialId];
        return (entry.badgeHash == badgeHash, entry.issuedAt);
    }

    /**
     * @notice Get all issued badge IDs.
     */
    function getIssuedBadges() external view returns (string[] memory) {
        return _issuedBadges;
    }

    /**
     * @notice Get the total number of badges issued.
     */
    function getIssuanceCount() external view returns (uint256) {
        return _issuedBadges.length;
    }

    // ──────────────────────────────────────────────
    // Query — Revocation
    // ──────────────────────────────────────────────

    /**
     * @notice Check whether a badge credential has been revoked.
     */
    function isRevoked(string calldata credentialId) external view returns (bool) {
        return _isRevoked[credentialId];
    }

    /**
     * @notice Get full revocation details for a credential.
     */
    function getRevocation(string calldata credentialId)
        external
        view
        returns (RevocationEntry memory entry)
    {
        require(_isRevoked[credentialId], "BadgeRegistry: credential not revoked");
        return _revocations[credentialId];
    }

    /**
     * @notice Get all revoked badge IDs.
     */
    function getRevokedBadges() external view returns (string[] memory) {
        return _revokedBadges;
    }

    /**
     * @notice Get the number of revoked badges.
     */
    function getRevocationCount() external view returns (uint256) {
        return _revokedBadges.length;
    }

    /**
     * @notice Get full status of a badge (issuance + revocation).
     * @param credentialId The credential identifier
     * @return issued   Whether the badge was recorded on-chain
     * @return revoked  Whether the badge has been revoked
     * @return badgeHash The SHA-256 hash (bytes32(0) if not issued)
     * @return issuedAt  Issuance timestamp (0 if not issued)
     * @return revokedAt Revocation timestamp (0 if not revoked)
     */
    function getBadgeStatus(string calldata credentialId)
        external
        view
        returns (
            bool issued,
            bool revoked,
            bytes32 badgeHash,
            uint256 issuedAt,
            uint256 revokedAt
        )
    {
        issued = _isIssued[credentialId];
        revoked = _isRevoked[credentialId];

        if (issued) {
            badgeHash = _issuances[credentialId].badgeHash;
            issuedAt = _issuances[credentialId].issuedAt;
        }
        if (revoked) {
            revokedAt = _revocations[credentialId].revokedAt;
        }
    }

    // ──────────────────────────────────────────────
    // Ownership
    // ──────────────────────────────────────────────

    /**
     * @notice Transfer ownership to a new Ethereum address.
     * @param newOwner The address of the new owner
     */
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "BadgeRegistry: new owner is zero address");

        address oldOwner = owner;
        owner = newOwner;

        emit OwnershipTransferred(oldOwner, newOwner);
    }

    // ──────────────────────────────────────────────
    // Internal
    // ──────────────────────────────────────────────

    function _revokeSingle(string calldata credentialId, string calldata reason) private {
        require(!_isRevoked[credentialId], "BadgeRegistry: credential already revoked");

        _isRevoked[credentialId] = true;
        _revocations[credentialId] = RevocationEntry({
            credentialId: credentialId,
            reason: reason,
            revokedAt: block.timestamp
        });
        _revokedBadges.push(credentialId);
    }
}
