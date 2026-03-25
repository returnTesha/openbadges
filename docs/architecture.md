# 시스템 아키텍처

The Badge 서버는 다인리더스가 운영하는 **중앙 배지 발급 서버**로, 여러 대학의 CPS(Campus Portal System)로부터 배지 발급 요청을 받아 처리한다. 사용자는 CPS 내의 마이페이지 또는 The Badge Service에 접근하여 본인이 수령한 배지를 확인할 수 있다.

## 전체 시스템 구성도

```mermaid
graph LR
    %% ── 외부: 대학 CPS (좌측) ──
    subgraph CPS["대학 CPS"]
        CPS_A["🏫 대학 A CPS<br/><small>비교과·진단 완료</small>"]
        CPS_B["🏫 대학 B CPS<br/><small>비교과·진단 완료</small>"]
        CPS_N["🏫 대학 N CPS<br/><small>비교과·진단 완료</small>"]
    end

    %% ── Dain IDC ──
    subgraph IDC["Dain IDC"]

        subgraph Server["The Badge Server · Go"]
            GW["<b>API Gateway</b><br/><small>Auth · Rate limit · Route<br/>발급자 토큰 검증</small>"]

            IH["<b>Issue Handler</b><br/><small>OB 3.0 JSON-LD 생성</small>"]
            VH["<b>Verify Handler</b><br/><small>Ed25519 서명 검증</small>"]

            SS["<b>Signer Service</b><br/><small>RDFC-1.0 → SHA-256 → Ed25519</small>"]
            DR["<b>DID Resolver</b><br/><small>did:web → Polygon fallback</small>"]

            PG[("🐘 <b>PostgreSQL</b><br/><small>발급자·배지 메타·이력 DB</small>")]
            S3[("📦 <b>MinIO / S3</b><br/><small>원본 PNG/JPG · 배지 JSON</small>")]
        end

        Vault["🔐 <b>HashiCorp Vault</b><br/><small>개인키 보관 · 발급 시에만 사용<br/>키 rotation 관리</small>"]
    end

    %% ── 외부: 사용자 (우측) ──
    subgraph Users["사용자"]
        Learner["👤 <b>사용자 · 학습자</b><br/><small>배지 수령/확인</small>"]
        Verifier["🔍 <b>검증자</b><br/><small>검증 페이지·자체검증</small>"]
        Admin["⚙️ <b>관리자</b><br/><small>디자인 교체·재발급</small>"]
    end

    subgraph Delivery["배지 전달 채널"]
        Email["📧 <b>이메일</b><br/><small>발급 즉시 자동 발송</small>"]
        CPSDl["📥 <b>CPS 다운로드</b><br/><small>재학 중 CPS에서 관리</small>"]
        Site["🌐 <b>The Badge 사이트</b><br/><small>CPS 소멸 후에도 조회·다운로드</small>"]
    end

    %% ── 외부: Polygon (하단) ──
    Polygon["⛓️ <b>Polygon Smart Contract</b><br/><small>공개키 앵커링 · keyHistory 영구 보존</small><br/><small><i>외부 블록체인 — 발급기관 소멸 시 fallback</i></small>"]

    %% ── 연결: CPS → Gateway ──
    CPS_A & CPS_B & CPS_N <--> GW

    %% ── Gateway → 핸들러 ──
    GW --> IH
    GW --> VH

    %% ── Issue 흐름 ──
    IH <--> SS
    SS <--> PG
    PG <--> S3

    %% ── Verify 흐름 ──
    VH --> DR
    DR --> PG

    %% ── Vault 연결 ──
    PG -.->|개인키 요청| Vault
    Vault -.->|공개키 등록| Polygon

    %% ── DID Resolver fallback ──
    DR -.->|did:web 실패 시 fallback| Polygon

    %% ── 사용자 연결 ──
    GW <--> Learner
    Verifier --> GW
    Admin -.-> GW

    %% ── 배지 전달 채널 ──
    Learner --> Email
    Learner --> CPSDl
    Learner --> Site
```

> **범례:** 실선 = 주요 데이터 흐름, 점선 = 보조/fallback 흐름

## 주요 구성요소

| 구성요소 | 역할 | 기술 스택 |
|---|---|---|
| **API Gateway** | 인증, 요청 라우팅, Rate Limit 처리 | Go / Fiber Middleware |
| **Issue Handler** | 배지 발급 요청 처리, JSON-LD 생성 | Go |
| **Verify Handler** | 배지 서명 검증 처리 | Go |
| **Signer Service** | Ed25519 서명 생성 (개인키 관리) | Go / `crypto/ed25519` |
| **DID Resolver** | did:web 기반 공개키 조회 | Go |
| **PostgreSQL** | 발급자 정보, 배지 메타데이터, 발급 이력 저장 | PostgreSQL |
| **MinIO / S3** | 배지 JSON 파일, 이미지 파일 저장 및 배포 | MinIO (S3 호환) |

## 내부 컴포넌트 관계도

```mermaid
graph TB
    subgraph Handler["핸들러 계층"]
        IH["Issue Handler"]
        VH["Verify Handler"]
    end

    subgraph Service["서비스 계층"]
        SS["Signer Service"]
        DR["DID Resolver"]
    end

    subgraph Storage["저장소 계층"]
        PG[(PostgreSQL)]
        S3[(MinIO / S3)]
    end

    subgraph External["외부 인프라"]
        Vault[(HashiCorp Vault)]
        Polygon[(Polygon Contract)]
    end

    GW["API Gateway"] --> IH
    GW --> VH

    IH <-->|서명 요청/응답| SS
    SS <-->|메타데이터 R/W| PG
    PG <-->|파일 저장/조회| S3

    VH -->|공개키 조회| DR
    DR -->|이력 조회| PG

    PG -.->|개인키 요청| Vault
    Vault -.->|공개키 등록| Polygon
    DR -.->|did:web 실패 시 fallback| Polygon
```

## 전체 데이터 흐름

시스템의 주요 데이터 흐름은 **세 단계**로 구성된다.

### 1단계: 발급 요청

```mermaid
sequenceDiagram
    participant CPS as 대학 CPS
    participant GW as API Gateway
    participant IH as Issue Handler

    CPS->>GW: POST /api/badges (학습자 정보, Achievement ID)
    GW->>GW: 인증 토큰 검증
    GW->>IH: 요청 라우팅
```

- 대학 CPS에서 학습자의 비교과 이수 또는 역량 진단 완료 이벤트 발생, 또는 사용자가 The Badge 서비스에 접근하여 요청
- The Badge API로 배지 발급 요청(HTTP POST) 전송
- API Gateway에서 인증 검증 후 Issue Handler로 라우팅

### 2단계: 배지 생성 및 서명

```mermaid
sequenceDiagram
    participant IH as Issue Handler
    participant SS as Signer Service
    participant Vault as HashiCorp Vault
    participant S3 as MinIO / S3
    participant PG as PostgreSQL

    IH->>IH: OB 3.0 JSON-LD 문서 생성
    IH->>SS: 서명 요청
    SS->>Vault: 개인키 조회
    Vault-->>SS: Ed25519 개인키
    SS->>SS: RDFC-1.0 정규화
    SS->>SS: SHA-256 해시
    SS->>SS: Ed25519 서명
    SS-->>IH: 서명된 배지 JSON
    IH->>S3: 배지 JSON + 이미지 저장
    IH->>PG: 메타데이터 + 발급 이력 저장
```

### 3단계: 배지 전달

```mermaid
sequenceDiagram
    participant IH as Issue Handler
    participant User as 학습자

    IH-->>User: 배지 전달 (CPS / Email / Download)
    Note over User: 배지 JSON 파일 또는<br/>다운로드 URL 수령
```

## 전체 흐름 통합 다이어그램

```mermaid
flowchart LR
    subgraph 발급요청["1단계: 발급 요청"]
        A[대학 CPS] -->|POST| B[API Gateway]
        B -->|인증 후 라우팅| C[Issue Handler]
    end

    subgraph 생성서명["2단계: 생성 & 서명"]
        C -->|JSON-LD 생성| D[OB 3.0 문서]
        D --> E[Signer Service]
        E -->|개인키| F[Vault]
        E -->|RDFC-1.0 → SHA-256 → Ed25519| G[서명된 배지]
    end

    subgraph 저장전달["3단계: 저장 & 전달"]
        G -->|파일 저장| H[MinIO]
        G -->|이력 저장| I[PostgreSQL]
        G -->|전달| J[학습자]
    end
```
