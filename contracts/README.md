# The Badge — Polygon Smart Contracts

On-chain infrastructure for the Open Badges 3.0 issuance/verification platform.
These contracts are deployed on Polygon and serve as a **permanent fallback** when the issuer's domain (did:web) is unreachable.

## Contracts

### KeyRegistry.sol

Public key anchoring with full rotation history.

- **Purpose:** When The Badge Service domain goes down, verifiers can still retrieve the Ed25519 public key from this contract and verify old badges.
- **Key features:**
  - Register DID with initial public key (`registerKey`)
  - Rotate keys (old key is preserved with `revokedAt` timestamp)
  - Emergency revocation without rotation
  - Time-travel query: retrieve the key that was active at any past timestamp
  - Full key history retrieval

### BadgeRegistry.sol

On-chain registry for badge issuance (hash anchoring) and revocation.

- **Purpose:** Records SHA-256 hashes of issued badges on-chain, providing tamper-evident proof of issuance. Also manages revocation of incorrectly issued badges.
- **Issuance features:**
  - Record badge hash at issuance (single and batch)
  - Verify badge file hash against on-chain record
  - Query all badges by issuer DID
  - Get full badge status (issued/revoked/hash/timestamps)
- **Revocation features:**
  - Single and batch revocation (only recorded badges can be revoked)
  - Per-issuer revocation lists
  - Query by credential ID or issuer DID

## Deployment

### Prerequisites

- Node.js >= 18
- Hardhat or Foundry

### Using Hardhat

```bash
# Install dependencies
npm install --save-dev hardhat @nomicfoundation/hardhat-toolbox

# Compile
npx hardhat compile

# Deploy to Polygon Amoy testnet
npx hardhat run scripts/deploy.js --network amoy

# Deploy to Polygon mainnet
npx hardhat run scripts/deploy.js --network polygon
```

Sample `hardhat.config.js` network entries:

```javascript
module.exports = {
  solidity: "0.8.20",
  networks: {
    amoy: {
      url: "https://rpc-amoy.polygon.technology",
      accounts: [process.env.DEPLOYER_PRIVATE_KEY],
    },
    polygon: {
      url: "https://polygon-rpc.com",
      accounts: [process.env.DEPLOYER_PRIVATE_KEY],
    },
  },
};
```

### Using Foundry

```bash
# Compile
forge build

# Deploy KeyRegistry
forge create --rpc-url https://rpc-amoy.polygon.technology \
  --private-key $DEPLOYER_PRIVATE_KEY \
  src/KeyRegistry.sol:KeyRegistry

# Deploy BadgeRegistry
forge create --rpc-url https://rpc-amoy.polygon.technology \
  --private-key $DEPLOYER_PRIVATE_KEY \
  src/BadgeRegistry.sol:BadgeRegistry
```

## Networks

| Network          | Chain ID | RPC URL                                  | Usage      |
|------------------|----------|------------------------------------------|------------|
| Polygon Mainnet  | 137      | `https://polygon-rpc.com`                | Production |
| Polygon Amoy     | 80002    | `https://rpc-amoy.polygon.technology`    | Testing    |

> Note: Mumbai testnet is deprecated. Use Amoy testnet.

## How Verifiers Should Query the Contracts

### 1. Recording Badge Issuance (Server-side)

When The Badge Service issues a badge, it records the SHA-256 hash on-chain:

```javascript
const badgeRegistry = new ethers.Contract(contractAddress, BadgeRegistryABI, signer);

// Hash the signed badge JSON
const badgeJson = JSON.stringify(signedBadge);
const badgeHash = ethers.sha256(ethers.toUtf8Bytes(badgeJson));

// Record on-chain
await badgeRegistry.recordIssuance("snu-0001", "did:web:thebadge.kr", badgeHash);
```

### 2. Verifying Badge Hash (Anyone)

```javascript
const badgeRegistry = new ethers.Contract(contractAddress, BadgeRegistryABI, provider);

// Hash the badge file you have
const badgeHash = ethers.sha256(ethers.toUtf8Bytes(badgeFileContent));

// Compare with on-chain record
const [matches, issuedAt] = await badgeRegistry.verifyHash("snu-0001", badgeHash);
if (matches) {
  console.log(`Badge is authentic, issued at ${new Date(issuedAt * 1000)}`);
} else {
  console.log("Badge hash does not match on-chain record!");
}
```

### 3. Checking Badge Status (Full)

```javascript
const [issued, revoked, badgeHash, issuedAt, revokedAt] =
  await badgeRegistry.getBadgeStatus("snu-0001");

console.log(`Issued: ${issued}`);
console.log(`Revoked: ${revoked}`);
console.log(`Hash: ${badgeHash}`);
console.log(`Issued at: ${new Date(issuedAt * 1000)}`);
if (revoked) {
  console.log(`Revoked at: ${new Date(revokedAt * 1000)}`);
}
```

### 4. Verifying Badge Signature (Public Key Fallback)

```javascript
const keyRegistry = new ethers.Contract(contractAddress, KeyRegistryABI, provider);

// Get the key that was active when the badge was issued
const [publicKey, activatedAt] = await keyRegistry.getKeyAtTime(
  "did:web:thebadge.kr",
  Math.floor(new Date("2026-06-15T00:00:00Z").getTime() / 1000)
);

// publicKey is bytes32 containing the Ed25519 public key
```

### 5. Full Verification Flow

A complete badge verification should:

1. Parse the badge JSON (OpenBadgeCredential)
2. Check badge status via `BadgeRegistry.getBadgeStatus(credentialId)`
   - Verify hash matches: `BadgeRegistry.verifyHash(credentialId, hash)`
   - Check revocation: `isRevoked == false`
3. Resolve the issuer's public key via did:web, falling back to `KeyRegistry.getKeyAtTime()`
4. Canonicalize the badge data (RDFC-1.0), hash (SHA-256), and verify the Ed25519 signature

## Gas Considerations

- **Read operations** (`getActiveKey`, `getKeyAtTime`, `isRevoked`, `verifyHash`, `getBadgeStatus`, etc.) are `view` functions and cost **zero gas**.
- **Write operations** cost gas on Polygon (MATIC). Typical costs:
  - `registerKey` (KeyRegistry): ~60,000 gas
  - `rotateKey`: ~80,000 gas
  - `registerIssuer` (BadgeRegistry): ~50,000 gas
  - `recordIssuance`: ~90,000 gas
  - `revokeBadge`: ~90,000 gas
  - Batch operations: ~70,000 gas per item
- At typical Polygon gas prices (30-50 gwei), each write costs fractions of a cent.
