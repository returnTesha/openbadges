# 스마트컨트랙트 호출 파라미터 예시

The Badge Service에서 사용하는 Polygon 스마트컨트랙트 2개의 함수별 호출 예시.

- DID: `did:web:thebadge.kr` (플랫폼 단일 DID)
- 배지 ID 형식: `{대학prefix}-{시퀀스}` (예: snu-0001)
- MVP 원칙: 비교과 1개 이수 = 배지 1개 발급

---

## KeyRegistry.sol — 공개키 관리

### constructor (배포 시 자동 실행)

컨트랙트 배포할 때 DID와 초기 공개키가 자동 등록된다. 별도 함수 호출 불필요.

```
// 배포 시 생성자 파라미터
constructor(
  did:       "did:web:thebadge.kr",
  publicKey: 0xa49648088a5e637a696ff0965763edcab053d5f1abbd191b3c43f8d8b60a4b4d
)
```

| 파라미터 | 타입 | 설명 |
|---|---|---|
| `did` | string | 플랫폼 DID (배포 시 고정, 이후 변경 불가) |
| `publicKey` | bytes32 | Ed25519 공개키 32바이트 (Vault에서 생성한 키쌍의 공개키) |

배포자(msg.sender)가 자동으로 owner가 된다.

---

### rotateKey (키 교체 시)

정기 교체 또는 담당자 변경 시 호출. 구 키는 revokedAt이 자동 기록되고 신 키가 추가된다.

```
rotateKey(
  newPublicKey: 0xb7283af...신규공개키32바이트...
)
```

| 파라미터 | 타입 | 설명 |
|---|---|---|
| `newPublicKey` | bytes32 | 새로 생성한 Ed25519 공개키 |

---

### revokeCurrentKey (긴급 폐기)

개인키 유출 의심 시 현재 키를 즉시 폐기한다. 이후 `addKey`로 신규 키를 등록해야 한다.

```
revokeCurrentKey(
  reason: "개인키 유출 의심으로 긴급 폐기"
)
```

| 파라미터 | 타입 | 설명 |
|---|---|---|
| `reason` | string | 폐기 사유 (자유 텍스트) |

---

### addKey (긴급 폐기 후 신규 키 등록)

`revokeCurrentKey` 후 활성 키가 없을 때 새 키를 등록한다.

```
addKey(
  newPublicKey: 0xc9d8e7f...신규공개키32바이트...
)
```

| 파라미터 | 타입 | 설명 |
|---|---|---|
| `newPublicKey` | bytes32 | 새 Ed25519 공개키 |

---

### getActiveKey (현재 활성 키 조회) — view, 가스비 무료

```
getActiveKey()

→ publicKey:   0xa49648088a5e637a696ff0965763edcab053d5f1abbd191b3c43f8d8b60a4b4d
  activatedAt: 1711155600    // 2026-03-23 09:00:00 UTC
```

---

### getKeyAtTime (특정 시점 키 조회) — view, 가스비 무료

과거에 발급된 배지를 검증할 때, 발급 시점에 유효했던 공개키를 조회한다.

```
getKeyAtTime(
  timestamp: 1718409600    // 2026-06-15 00:00:00 UTC
)

→ publicKey:   0xa49648...  // 해당 시점에 유효했던 키
  activatedAt: 1711155600
```

| 파라미터 | 타입 | 설명 |
|---|---|---|
| `timestamp` | uint256 | 조회할 시점 (Unix timestamp, 초 단위) |

---

### getKeyHistory (전체 키 이력) — view, 가스비 무료

```
getKeyHistory()

→ [
    {
      publicKey:    0xa49648...,
      activatedAt:  1711155600,    // 2026-03-23 09:00:00
      revokedAt:    1735689600,    // 2027-01-01 00:00:00
      revokeReason: "정기 교체"
    },
    {
      publicKey:    0xb7283a...,
      activatedAt:  1735689600,    // 2027-01-01 00:00:00
      revokedAt:    0,             // 현재 활성
      revokeReason: ""
    }
  ]
```

---

### getKeyCount / registeredDid / owner — view, 가스비 무료

```
getKeyCount()      → 2                        // 키 교체 1번 = 이력 2개
registeredDid()    → "did:web:thebadge.kr"    // 배포 시 등록된 DID
owner()            → 0x1234...                 // 컨트랙트 소유자 주소
```

---

## BadgeRegistry.sol — 배지 해시 기록 + 취소

### constructor (배포 시 자동 실행)

컨트랙트 배포할 때 issuer DID가 자동 등록된다.

```
constructor(
  issuerDid: "did:web:thebadge.kr"
)
```

| 파라미터 | 타입 | 설명 |
|---|---|---|
| `issuerDid` | string | 플랫폼 DID (배포 시 고정, 이후 변경 불가) |

배포자(msg.sender)가 자동으로 owner가 된다.

---

### recordIssuance (배지 발급 시마다)

비교과 1개 이수 → 배지 1개 발급 → 서명된 JSON의 해시를 온체인에 기록한다.

```
recordIssuance(
  credentialId: "SNU-LEADERSHIP-20261",
  badgeHash:    0xa1b2c3d4e5f67890abcdef1234567890abcdef1234567890abcdef1234567890
)
```

| 파라미터 | 타입 | 설명 |
|---|---|---|
| `credentialId` | string | 배지 고유 ID ({대학코드}-{카테고리}-{연도}{시퀀스}) |
| `badgeHash` | bytes32 | 서명 완료된 배지 JSON → SHA-256 해시 (32바이트) |

**badgeHash 생성 방법:**
```
1. 배지 JSON 발급 완료 (proof 필드 포함)
2. JSON을 문자열로 직렬화
3. SHA-256 해시 계산
4. 결과 32바이트가 badgeHash
```

---

### batchRecordIssuance (일괄 발급 기록)

여러 배지를 한 트랜잭션으로 기록한다. credentialIds와 badgeHashes 배열 순서가 매칭되어야 한다.

```
batchRecordIssuance(
  credentialIds: ["SNU-LEADERSHIP-20261", "SNU-GLOBAL_COMM-20262", "YONSEI-AI_DATA-20263"],
  badgeHashes:   [0xa1b2..., 0xc3d4..., 0xe5f6...]
)
```

---

### revokeBadge (오발급 취소)

기록된 배지만 취소할 수 있다 (recordIssuance 선행 필수).

```
revokeBadge(
  credentialId: "SNU-LEADERSHIP-20261",
  reason:       "학번 오기입으로 인한 오발급"
)
```

| 파라미터 | 타입 | 설명 |
|---|---|---|
| `credentialId` | string | 취소할 배지 ID |
| `reason` | string | 취소 사유 (자유 텍스트) |

---

### batchRevoke (일괄 취소)

```
batchRevoke(
  credentialIds: ["SNU-LEADERSHIP-20261", "SNU-GLOBAL_COMM-20262"],
  reason:        "시스템 오류로 인한 일괄 오발급"
)
```

---

### getBadgeStatus (전체 상태 조회) — view, 가스비 무료

배지의 발급/취소 상태, 해시, 시각을 한번에 조회한다.

```
getBadgeStatus("snu-0001")

→ issued:    true
  revoked:   false
  badgeHash: 0xa1b2c3d4...
  issuedAt:  1711155600    // 2026-03-23 09:00:00
  revokedAt: 0
```

취소된 배지:
```
getBadgeStatus("snu-0003")

→ issued:    true
  revoked:   true
  badgeHash: 0xf1e2d3c4...
  issuedAt:  1711155600    // 2026-03-23 09:00:00
  revokedAt: 1711242000    // 2026-03-24 09:00:00
```

---

### verifyHash (해시 대조 검증) — view, 가스비 무료

배지 파일을 가지고 있을 때, 해당 파일의 해시가 온체인 기록과 일치하는지 확인한다.

```
verifyHash(
  credentialId: "snu-0001",
  badgeHash:    0xa1b2c3d4...    // 검증할 배지 파일의 SHA-256
)

→ matches:  true               // 일치 = 원본 그대로
  issuedAt: 1711155600

→ matches:  false              // 불일치 = 파일이 변조됨
  issuedAt: 1711155600
```

---

### 기타 조회 — view, 가스비 무료

```
isIssued("SNU-LEADERSHIP-20261")                → true
isRevoked("SNU-LEADERSHIP-20261")              → false

getIssuance("SNU-LEADERSHIP-20261")
→ { credentialId: "SNU-LEADERSHIP-20261", badgeHash: 0xa1b2..., issuedAt: 1711155600 }

getRevocation("SNU-GLOBAL_COMM-20262")
→ { credentialId: "SNU-GLOBAL_COMM-20262", reason: "학번 오기입", revokedAt: 1711242000 }

getIssuedBadges()      → ["SNU-LEADERSHIP-20261", "SNU-GLOBAL_COMM-20262", "YONSEI-AI_DATA-20263"]
getIssuanceCount()     → 3

getRevokedBadges()     → ["SNU-GLOBAL_COMM-20262"]
getRevocationCount()   → 1

issuerDid()            → "did:web:thebadge.kr"    // 배포 시 등록된 DID
owner()                → 0x1234...                 // 컨트랙트 소유자 주소

transferOwnership(0xNewOwnerAddress...)
```
