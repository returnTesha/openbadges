# BadgeRegistry 수동 테스트 가이드

컨트랙트 주소: `(재배포 후 기입)`
네트워크: Polygon Amoy

Remix 또는 Polygonscan Write Contract 탭에서 테스트.

credential_id 형식: `{대학코드}-{카테고리}-{연도}{시퀀스}`
예: `SNU-LEADERSHIP-20261`

---

## 1. 배포 확인 (Read — 가스비 무료)

### issuerDid()
```
파라미터: 없음
예상: "did:web:thebadge.kr"
```

### owner()
```
파라미터: 없음
예상: 0x6A40D43C... (배포자 지갑)
```

---

## 2. 배지 발급 기록 (Write — 가스비 발생)

### recordIssuance — 서울대 리더십 배지
```
credentialId: "SNU-LEADERSHIP-20261"
badgeHash:    0x1a2b3c4d5e6f708192a3b4c5d6e7f80819a2b3c4d5e6f708192a3b4c5d6e7f80
```

### recordIssuance — 서울대 글로벌 커뮤니케이션 배지
```
credentialId: "SNU-GLOBAL_COMM-20262"
badgeHash:    0x2b3c4d5e6f708192a3b4c5d6e7f80819a2b3c4d5e6f708192a3b4c5d6e7f8091
```

### recordIssuance — 연세대 AI 역량 배지
```
credentialId: "YONSEI-AI_DATA-20263"
badgeHash:    0x3c4d5e6f708192a3b4c5d6e7f80819a2b3c4d5e6f708192a3b4c5d6e7f8091200
```

### batchRecordIssuance — 일괄 (3건)
```
credentialIds: ["SNU-LEADERSHIP-20264", "SNU-LEADERSHIP-20265", "YONSEI-GLOBAL_COMM-20266"]
badgeHashes:   [
  0x4d5e6f708192a3b4c5d6e7f80819a2b3c4d5e6f708192a3b4c5d6e7f809122300,
  0x5e6f708192a3b4c5d6e7f80819a2b3c4d5e6f708192a3b4c5d6e7f80912233400,
  0x6f708192a3b4c5d6e7f80819a2b3c4d5e6f708192a3b4c5d6e7f8091223344500
]
```

---

## 3. 발급 확인 (Read — 가스비 무료)

### isIssued
```
credentialId: "SNU-LEADERSHIP-20261"
예상: true
```
```
credentialId: "FAKE-99999"
예상: false
```

### getIssuance
```
credentialId: "SNU-LEADERSHIP-20261"
예상: { credentialId: "SNU-LEADERSHIP-20261", badgeHash: 0x1a2b..., issuedAt: 17111... }
```

### getIssuedBadges
```
파라미터: 없음
예상: ["SNU-LEADERSHIP-20261", "SNU-GLOBAL_COMM-20262", "YONSEI-AI_DATA-20263", ...]
```

### getIssuanceCount
```
파라미터: 없음
예상: 6
```

---

## 4. 해시 검증 (Read — 가스비 무료)

### verifyHash — 일치
```
credentialId: "SNU-LEADERSHIP-20261"
badgeHash:    0x1a2b3c4d5e6f708192a3b4c5d6e7f80819a2b3c4d5e6f708192a3b4c5d6e7f80
예상: matches=true, issuedAt=17111...
```

### verifyHash — 불일치 (변조)
```
credentialId: "SNU-LEADERSHIP-20261"
badgeHash:    0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
예상: matches=false, issuedAt=17111...
```

### verifyHash — 미등록
```
credentialId: "FAKE-99999"
badgeHash:    0x1a2b3c4d5e6f708192a3b4c5d6e7f80819a2b3c4d5e6f708192a3b4c5d6e7f80
예상: matches=false, issuedAt=0
```

---

## 5. 배지 취소 (Write — 가스비 발생)

### revokeBadge — 단건
```
credentialId: "SNU-GLOBAL_COMM-20262"
reason:       "학번 오기입으로 인한 오발급"
```

### batchRevoke — 일괄
```
credentialIds: ["SNU-LEADERSHIP-20264", "SNU-LEADERSHIP-20265"]
reason:        "시스템 오류로 인한 일괄 오발급"
```

### 에러: 미등록 배지 취소
```
credentialId: "FAKE-99999"
reason:       "테스트"
예상 revert: "BadgeRegistry: credential not recorded"
```

### 에러: 이미 취소된 배지
```
credentialId: "SNU-GLOBAL_COMM-20262"
reason:       "중복 테스트"
예상 revert: "BadgeRegistry: credential already revoked"
```

---

## 6. 취소 확인 (Read — 가스비 무료)

### isRevoked
```
credentialId: "SNU-GLOBAL_COMM-20262"
예상: true
```
```
credentialId: "SNU-LEADERSHIP-20261"
예상: false
```

### getRevocation
```
credentialId: "SNU-GLOBAL_COMM-20262"
예상: { credentialId: "SNU-GLOBAL_COMM-20262", reason: "학번 오기입으로 인한 오발급", revokedAt: ... }
```

### getRevokedBadges
```
파라미터: 없음
예상: ["SNU-GLOBAL_COMM-20262", "SNU-LEADERSHIP-20264", "SNU-LEADERSHIP-20265"]
```

### getRevocationCount
```
파라미터: 없음
예상: 3
```

---

## 7. 전체 상태 (Read — 가스비 무료)

### getBadgeStatus — 정상 배지
```
credentialId: "SNU-LEADERSHIP-20261"
예상: issued=true, revoked=false, badgeHash=0x1a2b..., issuedAt=..., revokedAt=0
```

### getBadgeStatus — 취소된 배지
```
credentialId: "SNU-GLOBAL_COMM-20262"
예상: issued=true, revoked=true, badgeHash=0x2b3c..., issuedAt=..., revokedAt=...
```

### getBadgeStatus — 미등록
```
credentialId: "FAKE-99999"
예상: issued=false, revoked=false, badgeHash=0x000...0, issuedAt=0, revokedAt=0
```

---

## 8. 소유권 이전 (소스 검토 완료, 실제 테스트 불필요)

### transferOwnership
```
newOwner: 0x새주소
효과: owner 변경, 이후 Write 함수는 새 주소만 호출 가능
주의: 되돌릴 수 없음
```

---

## 에러 정리

| 상황 | 함수 | revert 메시지 |
|---|---|---|
| 중복 기록 | recordIssuance | "credential already recorded" |
| 빈 해시 | recordIssuance | "zero hash not allowed" |
| 미등록 취소 | revokeBadge | "credential not recorded" |
| 중복 취소 | revokeBadge | "credential already revoked" |
| 권한 없음 | 모든 Write | "caller is not the owner" |
| 빈 주소 | transferOwnership | "new owner is zero address" |

---

## 테스트 순서 (권장)

```
1.  issuerDid(), owner() → 배포 확인
2.  recordIssuance("SNU-LEADERSHIP-20261", hash) → 기록
3.  isIssued("SNU-LEADERSHIP-20261") → true
4.  getIssuance("SNU-LEADERSHIP-20261") → 상세
5.  verifyHash("SNU-LEADERSHIP-20261", hash) → true
6.  verifyHash("SNU-LEADERSHIP-20261", fakeHash) → false
7.  recordIssuance("SNU-GLOBAL_COMM-20262", hash2) → 두 번째
8.  getIssuanceCount() → 2
9.  revokeBadge("SNU-GLOBAL_COMM-20262", "오발급") → 취소
10. isRevoked("SNU-GLOBAL_COMM-20262") → true
11. getBadgeStatus("SNU-LEADERSHIP-20261") → issued=true, revoked=false
12. getBadgeStatus("SNU-GLOBAL_COMM-20262") → issued=true, revoked=true
13. getIssuedBadges() → 전체 목록
14. getRevokedBadges() → 취소 목록
```
