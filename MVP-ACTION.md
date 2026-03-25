# MVP 최종 액션 플랜

The Badge Service MVP 시연 순서.

## 도구별 역할

| 도구 | 용도 |
|---|---|
| **Postman** | 배지 발급, API 검증, 조회, 재발급, 에러 케이스 |
| **브라우저** | 배지 다운로드, Frontend 검증 페이지, Polygonscan |
| **Frontend (localhost:3001)** | 배지 파일 드래그앤드롭 검증 (검증 과정 시각화) |
| **Polygonscan** | 블록체인 온체인 데이터 확인 |

---

## 사전 준비

```bash
# 1. PostgreSQL 컨테이너 실행 확인
docker ps | grep postgres16

# 2. 마이그레이션 실행 (최초 1회)
PGPASSWORD=dainls1q2w3e4r psql -h localhost -U thebadge -d openbadge -f app/migrations/001_initial_schema.sql
PGPASSWORD=dainls1q2w3e4r psql -h localhost -U thebadge -d openbadge -f app/migrations/002_add_credential_fields.sql
PGPASSWORD=dainls1q2w3e4r psql -h localhost -U thebadge -d openbadge -f app/migrations/003_nullable_achievement_id.sql
PGPASSWORD=dainls1q2w3e4r psql -h localhost -U thebadge -d openbadge -f app/migrations/004_credential_sequence.sql
PGPASSWORD=dainls1q2w3e4r psql -h localhost -U thebadge -d openbadge -f app/migrations/005_cps_columns.sql
PGPASSWORD=dainls1q2w3e4r psql -h localhost -U thebadge -d openbadge -f app/migrations/006_achievements_issuer_index.sql

# 3. Go 서버 기동
cd app && source ../.env && go run cmd/server/main.go

# 4. Frontend 기동
cd web && npm run dev    # http://localhost:3001/verify

# 5. Postman 컬렉션 Import
# 파일: the-badge-mvp.postman_collection.json
```

---

## Step 0. 서버 상태 확인

### 0-1. Health Check 🔧 Postman
```
GET http://localhost:3000/health

예상 응답:
{ "data": { "status": "ok" } }
```

### 0-2. DID Document 확인 🔧 Postman 또는 🌐 브라우저
```
GET http://localhost:3000/.well-known/did.json

확인 포인트:
✅ id: "did:web:thebadge.kr"
✅ publicKeyMultibase 존재
✅ service[0].serviceEndpoint.network: "polygon-mainnet"
✅ keyRegistryAddress: "0x4a6f1e4b94fbd6DdFb4e10e0D02CB7c563DBf868"
✅ badgeRegistryAddress: "0xB23E1c103E326D0A28135e91D1D610bB038BE632"
```

---

## Step 1. 배지 발급 (3건) 🔧 Postman

### 1-1. 서울대 리더십 역량 (홍길동)

```
POST http://localhost:3000/api/v1/badges
Content-Type: application/json

{
  "university_code": "SNU",
  "program_id": "NPI00001",
  "program_category": "LEADERSHIP",
  "student_id": "2021-12345",
  "recipient_name": "홍길동",
  "recipient_email": "hong@snu.ac.kr",
  "achievement_name": "리더십 역량 프로그램",
  "achievement_desc": "비교과 리더십 역량 개발 프로그램 이수",
  "criteria": "리더십 캠프 참여 + 멘토링 활동 40시간 이상 완료",
  "image_base64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
}

확인 포인트:
✅ 201 Created
✅ credential_id: SNU-LEADERSHIP-2026{seq}
✅ credentialSubject.id: did:web:thebadge.kr:users:2021-12345
✅ credentialSubject.source.code: SNU
✅ credentialSubject.achievement.category: LEADERSHIP
✅ credentialSubject.achievement.programId: NPI00001
✅ proof.type: DataIntegrityProof
✅ proof.cryptosuite: eddsa-rdfc-2022
```

### 1-2. 서울대 글로벌 커뮤니케이션 (김철수)

```
POST http://localhost:3000/api/v1/badges
Content-Type: application/json

{
  "university_code": "SNU",
  "program_id": "NPI00002",
  "program_category": "GLOBAL_COMM",
  "student_id": "2022-67890",
  "recipient_name": "김철수",
  "recipient_email": "kim@snu.ac.kr",
  "achievement_name": "글로벌 커뮤니케이션 역량",
  "achievement_desc": "비교과 글로벌 역량 프로그램 이수",
  "criteria": "영어토론 동아리 + 국제교류 봉사 30시간 이상",
  "image_base64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="
}

확인 포인트:
✅ 201 Created
✅ credential_id: SNU-GLOBAL_COMM-2026{seq}
```

### 1-3. 연세대 AI 데이터 분석 (이영희)

```
POST http://localhost:3000/api/v1/badges
Content-Type: application/json

{
  "university_code": "YONSEI",
  "program_id": "YNP00001",
  "program_category": "AI_DATA",
  "student_id": "2023-11111",
  "recipient_name": "이영희",
  "recipient_email": "lee@yonsei.ac.kr",
  "achievement_name": "AI 데이터 분석 역량",
  "achievement_desc": "비교과 AI·데이터 분석 프로그램 이수",
  "criteria": "Python 데이터 분석 과정 수료 + 캡스톤 프로젝트 완료",
  "image_base64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPj/HwADBwIAMCbHYQAAAABJRU5ErkJggg=="
}

확인 포인트:
✅ 201 Created
✅ credential_id: YONSEI-AI_DATA-2026{seq}
✅ credentialSubject.source.code: YONSEI
```

---

## Step 2. 배지 다운로드 🌐 브라우저

발급된 3개 배지를 각각 .json 파일로 다운로드한다.
credential_id는 Step 1 응답에서 확인.

```
GET http://localhost:3000/api/v1/badges/c/SNU-LEADERSHIP-2026{seq}/download
→ SNU-LEADERSHIP-2026{seq}.json 파일 저장됨

GET http://localhost:3000/api/v1/badges/c/SNU-GLOBAL_COMM-2026{seq}/download
→ SNU-GLOBAL_COMM-2026{seq}.json 파일 저장됨

GET http://localhost:3000/api/v1/badges/c/YONSEI-AI_DATA-2026{seq}/download
→ YONSEI-AI_DATA-2026{seq}.json 파일 저장됨
```

브라우저 주소창에 URL 입력하면 바로 다운로드됨.

---

## Step 3. 배지 검증 (API) 🔧 Postman

### 3-1. 정상 배지 검증 → valid: true

```
POST http://localhost:3000/api/v1/badges/verify-sync
Content-Type: application/json

(Step 1-1에서 발급받은 배지 JSON 전체를 body에 넣기)

확인 포인트:
✅ valid: true
✅ issuer_did: did:web:thebadge.kr
✅ recipient_name: 홍길동
✅ achievement_name: 리더십 역량 프로그램
✅ errors: []
```

### 3-2. 위변조 배지 검증 → valid: false

Step 1-1 응답을 복사한 뒤 `credentialSubject.name`을 `"위조자"`로 변경:

```
POST http://localhost:3000/api/v1/badges/verify-sync
Content-Type: application/json

(변조된 배지 JSON)

확인 포인트:
✅ valid: false
✅ errors: ["Ed25519 signature verification failed"] 또는 유사 메시지
```

### 3-3. 가짜 배지 검증 → valid: false

```
POST http://localhost:3000/api/v1/badges/verify-sync
Content-Type: application/json

{
  "@context": ["https://www.w3.org/ns/credentials/v2"],
  "id": "https://thebadge.kr/credentials/FAKE-99999",
  "type": ["VerifiableCredential", "OpenBadgeCredential"],
  "issuer": { "id": "did:web:thebadge.kr" },
  "validFrom": "2026-03-23T12:00:00Z",
  "credentialSubject": { "name": "위조자", "achievement": { "name": "가짜" } },
  "proof": {
    "type": "DataIntegrityProof",
    "cryptosuite": "eddsa-rdfc-2022",
    "verificationMethod": "did:web:thebadge.kr#key-1",
    "proofPurpose": "assertionMethod",
    "proofValue": "zFAKEINVALIDSIGNATURE"
  }
}

확인 포인트:
✅ valid: false
```

---

## Step 4. Frontend 검증 (드래그 앤 드롭) 🌐 브라우저 (localhost:3001/verify)

### 4-1. 정상 배지 파일 검증

1. 브라우저에서 `http://localhost:3001/verify` 접속
2. Step 2에서 다운로드한 `SNU-LEADERSHIP-2026{seq}.json` 파일을 드래그 앤 드롭
3. 검증 과정 5단계 애니메이션 확인:
   ```
   Step 1: 배지 JSON 파싱 ✅
   Step 2: DID 조회 ✅
   Step 3: Ed25519 서명 검증 ✅
   Step 4: 블록체인 해시 대조 ✅
   Step 5: 취소 여부 확인 ✅
   최종 결과: ✅ 검증 성공
   ```
4. 배지 정보 표시 확인:
   - 발급기관: 다인리더스 The Badge Service
   - 수령자: 홍길동
   - 비교과: 리더십 역량 프로그램
   - 배지 이미지 표시

### 4-2. 위변조 파일 검증

1. 다운로드한 .json 파일을 텍스트 에디터로 열어서 `recipient_name`을 변경
2. 저장 후 Frontend에 드래그 앤 드롭
3. 검증 실패 확인:
   ```
   Step 3: Ed25519 서명 검증 ❌
   최종 결과: ❌ 검증 실패
   ```

---

## Step 5. 조회 🔧 Postman

### 5-1. 배지 목록

```
GET http://localhost:3000/api/v1/badges?page=1&per_page=20

확인 포인트:
✅ total_count: 3 이상
✅ items에 SNU-LEADERSHIP, SNU-GLOBAL_COMM, YONSEI-AI_DATA 포함
```

### 5-2. credential_id로 배지 조회

```
GET http://localhost:3000/api/v1/badges/c/SNU-LEADERSHIP-2026{seq}

확인 포인트:
✅ credential_id, recipient_name, status, credential_json 포함
```

### 5-3. 발급 이력

```
GET http://localhost:3000/api/v1/history/issues?page=1&per_page=20

확인 포인트:
✅ total_count: 3 이상
```

### 5-4. 검증 이력

```
GET http://localhost:3000/api/v1/history/verifications?page=1&per_page=20

확인 포인트:
✅ Step 3에서 검증한 이력이 기록됨
✅ valid, invalid 결과 모두 존재
```

---

## Step 6. 이미지 교체 (재발급) 🔧 Postman

### 6-1. 기존 배지 이미지 교체 후 재발급

```
POST http://localhost:3000/api/v1/badges/c/SNU-LEADERSHIP-2026{seq}/reissue
Content-Type: application/json

{
  "image_base64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAEklEQVQIW2P8z8BQDwADhQGAWjR9awAAAABJRU5ErkJggg=="
}

확인 포인트:
✅ 201 Created
✅ 새 credential_id 발급됨 (기존과 다름)
✅ 이미지가 새 이미지로 교체됨
✅ 나머지 메타데이터 (수령자, 비교과명 등)는 유지
✅ 새로 서명됨 (proof.proofValue 변경)
```

### 6-2. 재발급 배지 다운로드 + 검증

```
GET http://localhost:3000/api/v1/badges/c/{새_credential_id}/download
→ 새 배지 파일 다운로드

POST http://localhost:3000/api/v1/badges/verify-sync
→ 새 배지 검증: valid: true
```

---

## Step 7. 에러 케이스 🔧 Postman

### 7-1. 필수 필드 누락

```
POST http://localhost:3000/api/v1/badges
Content-Type: application/json

{ "achievement_name": "테스트" }

확인 포인트:
✅ 422 Unprocessable Entity
✅ RFC 7807 형식 (title: "Validation Failed")
✅ errors 배열에 누락 필드 표시
```

### 7-2. 존재하지 않는 배지 조회

```
GET http://localhost:3000/api/v1/badges/c/FAKE-99999

확인 포인트:
✅ 404 Not Found
```

### 7-3. 잘못된 JSON 검증

```
POST http://localhost:3000/api/v1/badges/verify-sync
Content-Type: application/json

이것은 JSON이 아닙니다

확인 포인트:
✅ 400 Bad Request
```

---

## Step 8. 블록체인 확인 🌐 브라우저 (Polygonscan)

### 8-1. KeyRegistry 확인
- https://polygonscan.com/address/0x4a6f1e4b94fbd6DdFb4e10e0D02CB7c563DBf868
- Read Contract → `getActiveKey()` → 공개키 확인
- Read Contract → `registeredDid()` → `did:web:thebadge.kr` 확인

### 8-2. BadgeRegistry 확인
- https://polygonscan.com/address/0xB23E1c103E326D0A28135e91D1D610bB038BE632
- Read Contract → `issuerDid()` → `did:web:thebadge.kr` 확인
- Read Contract → `isIssued("SNU-LEADERSHIP-2026{seq}")` → true
- Read Contract → `getBadgeStatus("SNU-LEADERSHIP-2026{seq}")` → issued=true, revoked=false

---

## 체크리스트

```
[ ] Step 0: 서버 기동 + DID Document 확인           🔧 Postman
[ ] Step 1: 배지 3건 발급                            🔧 Postman
[ ] Step 2: 배지 3건 다운로드                         🌐 브라우저
[ ] Step 3: API 검증 (정상, 위변조, 가짜)              🔧 Postman
[ ] Step 4: Frontend 드래그앤드롭 검증                 🌐 localhost:3001
[ ] Step 5: 목록/이력 조회                            🔧 Postman
[ ] Step 6: 이미지 교체 재발급 + 재검증                🔧 Postman
[ ] Step 7: 에러 케이스                               🔧 Postman
[ ] Step 8: Polygonscan 블록체인 확인                  🌐 Polygonscan
```
