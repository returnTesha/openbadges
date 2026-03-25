# MVP 시나리오

서울대학교 학생이 비교과 프로그램을 이수하고 The Badge Service에서 오픈배지를 발급받는 전체 흐름.

## 등장인물
- **서울대 CPS**: 배지 발급 요청 (Postman으로 시뮬레이션)
- **The Badge Server**: Go Fiber API 서버 (localhost:3000)
- **검증자**: 배지 파일을 검증 페이지에 업로드
- **관리자**: 배지 이미지 교체

## 사전 조건
- The Badge Server 가동 중
- PostgreSQL 컨테이너 실행 중 (openbadge DB)
- Polygon Amoy 컨트랙트 배포 완료
  - KeyRegistry: `0xc141FA97886D3EadDE68ee63bB054D4C1aE9f0A1`
  - BadgeRegistry: `0x4030093DA3463Fe813b2bD16f8837aA609d4efAB`

---

## 시나리오 1: 배지 발급

### 1-1. 서울대 CPS가 발급 요청

```
POST http://localhost:3000/api/v1/badges
Authorization: Bearer {jwt_token}
Content-Type: application/json

{
  "achievement_id": "snu-leadership-2026",
  "achievement_name": "리더십 역량 프로그램",
  "achievement_desc": "비교과 리더십 역량 개발 프로그램 이수",
  "criteria": "리더십 캠프 참여 + 멘토링 활동 40시간 이상 완료",
  "image_base64": "data:image/png;base64,iVBORw0KGgo...",
  "recipient_name": "홍길동",
  "recipient_email": "hong@snu.ac.kr",
  "recipient_did": "did:example:student:2021-12345"
}
```

### 1-2. 서버 처리 과정

```
1. API Gateway: JWT 인증 검증
2. Issue Handler: OB 3.0 JSON-LD 생성
3. Signer Service: Vault에서 Ed25519 개인키 → RDFC 정규화 → SHA-256 → 서명
4. proof 필드 추가
5. MinIO: 배지 JSON + 이미지 저장
6. PostgreSQL: 메타데이터 + 발급 이력 저장
7. Blockchain: BadgeRegistry.recordIssuance(credentialId, badgeHash) 호출
```

### 1-3. 응답 (201 Created)

```json
{
  "data": {
    "@context": [
      "https://www.w3.org/ns/credentials/v2",
      "https://purl.imsglobal.org/spec/ob/v3p0/context-3.0.3.json"
    ],
    "id": "https://thebadge.kr/credentials/snu-0001",
    "type": ["VerifiableCredential", "OpenBadgeCredential"],
    "issuer": {
      "id": "did:web:thebadge.kr",
      "type": ["Profile"],
      "name": "다인리더스 The Badge Service",
      "url": "https://thebadge.kr",
      "didFallback": {
        "method": "polygon",
        "contractAddress": "0xc141FA97886D3EadDE68ee63bB054D4C1aE9f0A1",
        "network": "polygon-amoy"
      }
    },
    "issuanceDate": "2026-03-23T12:00:00Z",
    "credentialSubject": {
      "id": "did:example:student:2021-12345",
      "type": ["AchievementSubject"],
      "name": "홍길동",
      "identifier": {
        "type": "StudentId",
        "identityValue": "2021-12345"
      },
      "source": {
        "type": "Profile",
        "name": "서울대학교",
        "url": "https://www.snu.ac.kr"
      },
      "achievement": {
        "id": "https://thebadge.kr/achievements/snu-leadership-2026",
        "type": ["Achievement"],
        "name": "리더십 역량 프로그램",
        "description": "비교과 리더십 역량 개발 프로그램 이수",
        "criteria": {
          "narrative": "리더십 캠프 참여 + 멘토링 활동 40시간 이상 완료"
        },
        "image": {
          "id": "data:image/png;base64,iVBORw0KGgo...",
          "type": "Image"
        }
      }
    },
    "proof": {
      "type": "DataIntegrityProof",
      "cryptosuite": "eddsa-rdfc-2022",
      "created": "2026-03-23T12:00:00Z",
      "verificationMethod": "did:web:thebadge.kr#key-1",
      "proofPurpose": "assertionMethod",
      "proofValue": "z3FXQjecWufY46..."
    }
  }
}
```

---

## 시나리오 2: 배지 검증

### 2-1. 동기 검증 (Frontend용)

```
POST http://localhost:3000/api/v1/badges/verify-sync
Content-Type: multipart/form-data

badge_file: (발급받은 배지 JSON 파일)
```

또는 JSON body로:

```
POST http://localhost:3000/api/v1/badges/verify-sync
Content-Type: application/json

{
  "credential_json": { ... 발급받은 배지 JSON 전체 ... }
}
```

### 2-2. 서버 검증 과정

```
1. 배지 JSON 파싱
2. issuer.id에서 DID 추출 → did:web:thebadge.kr
3. did:web 조회 (/.well-known/did.json) → 공개키 획득
   (실패 시 → didFallback.contractAddress → Polygon KeyRegistry에서 getKeyAtTime)
4. Ed25519 서명 검증 (RDFC 정규화 → SHA-256 → 서명 대조)
5. BadgeRegistry.isRevoked(credentialId) → 취소 여부 확인
6. BadgeRegistry.verifyHash(credentialId, hash) → 해시 대조
7. 만료일 확인
```

### 2-3. 검증 성공 응답

```json
{
  "data": {
    "valid": true,
    "credential": { ... },
    "issuer_did": "did:web:thebadge.kr",
    "issuer_name": "다인리더스 The Badge Service",
    "recipient_name": "홍길동",
    "achievement_name": "리더십 역량 프로그램",
    "issued_at": "2026-03-23T12:00:00Z",
    "expires_at": "",
    "errors": []
  }
}
```

### 2-4. 검증 실패 응답 (위변조 시)

```json
{
  "data": {
    "valid": false,
    "issuer_did": "did:web:thebadge.kr",
    "issuer_name": "다인리더스 The Badge Service",
    "errors": ["signature verification failed: Ed25519 signature mismatch"]
  }
}
```

---

## 시나리오 3: 디자인 교체 + 재발급

### 3-1. 관리자가 새 이미지 업로드

```
PUT http://localhost:3000/api/v1/achievements/snu-leadership-2026/image
Authorization: Bearer {admin_token}
Content-Type: multipart/form-data

image: (새 배지 이미지 PNG)
```

### 3-2. 학생이 재발급 요청

```
POST http://localhost:3000/api/v1/badges/reissue
Authorization: Bearer {jwt_token}
Content-Type: application/json

{
  "original_badge_file": { ... 기존 배지 JSON 전체 ... }
}
```

### 3-3. 서버 처리

```
1. 기존 배지 서명 검증 (유효한 배지인지 확인)
2. achievement_id로 최신 이미지 조회
3. 새 이미지로 배지 JSON 재구성
4. 새로 서명 (현재 활성 키로)
5. DB + MinIO 저장
6. 블록체인 해시 기록
7. 새 배지 전달
```

---

## 시나리오 4: 오발급 취소

### 4-1. 관리자가 배지 취소

```
DELETE http://localhost:3000/api/v1/badges/snu-0001
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "reason": "학번 오기입으로 인한 오발급"
}
```

### 4-2. 서버 처리

```
1. DB: badges 테이블 status → revoked 업데이트
2. Blockchain: BadgeRegistry.revokeBadge("snu-0001", "학번 오기입으로 인한 오발급")
3. 이후 해당 배지 검증 시 → "revoked" 결과 반환
```

---

## 보조 API

### 헬스체크
```
GET http://localhost:3000/health
→ {"data": {"status": "ok"}}
```

### DID Document 조회
```
GET http://localhost:3000/.well-known/did.json
→ DID Document (공개키 포함)
```

### 배지 조회
```
GET http://localhost:3000/api/v1/badges/snu-0001
GET http://localhost:3000/api/v1/badges?page=1&per_page=20&status=active
```

### 발급 이력
```
GET http://localhost:3000/api/v1/history/issues?page=1&per_page=20
```

### 검증 이력
```
GET http://localhost:3000/api/v1/history/verifications?page=1&per_page=20
```
