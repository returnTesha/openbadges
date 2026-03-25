-- 001_initial_schema.sql
-- The Badge Project — PostgreSQL schema for OB 3.0 badge issuance system.
-- Requires PostgreSQL 13+ for gen_random_uuid().

BEGIN;

-- ============================================================================
-- 1. issuers (발급 기관)
-- ============================================================================
CREATE TABLE issuers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    did          TEXT NOT NULL UNIQUE,           -- e.g., did:web:example.com
    name         TEXT NOT NULL,                  -- 기관명
    url          TEXT,                           -- 기관 URL
    logo_base64  TEXT,                           -- 기관 로고 (base64 encoded)
    public_key   BYTEA NOT NULL,                -- Ed25519 공개키 (32 bytes)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================================
-- 2. achievements (배지 정의)
-- ============================================================================
CREATE TABLE achievements (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issuer_id          UUID NOT NULL REFERENCES issuers(id),
    name               TEXT NOT NULL,             -- 배지 이름
    description        TEXT NOT NULL,             -- 배지 설명
    criteria_narrative TEXT NOT NULL,             -- 이수 기준
    image_base64       TEXT,                      -- 배지 이미지 (base64 encoded)
    image_url          TEXT,                      -- MinIO 원본 이미지 URL
    tags               TEXT[],                    -- 태그 배열
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================================
-- 3. badges (발급된 배지)
-- ============================================================================
CREATE TABLE badges (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    achievement_id    UUID NOT NULL REFERENCES achievements(id),
    issuer_id         UUID NOT NULL REFERENCES issuers(id),
    recipient_name    TEXT NOT NULL,              -- 수령자 이름
    recipient_email   TEXT NOT NULL,              -- 수령자 이메일
    recipient_did     TEXT,                       -- 수령자 DID (optional)
    credential_json   JSONB NOT NULL,             -- 서명된 OB 3.0 JSON 전체
    proof_value       TEXT NOT NULL,              -- Ed25519 서명값 (base64)
    minio_key         TEXT,                       -- MinIO 저장 키
    status            TEXT NOT NULL DEFAULT 'active'
                      CHECK (status IN ('active', 'revoked', 'expired')),
    issued_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ,               -- 만료일 (optional)
    revoked_at        TIMESTAMPTZ,               -- 취소일
    revocation_reason TEXT                        -- 취소 사유
);

-- ============================================================================
-- 4. verification_logs (검증 이력)
-- ============================================================================
CREATE TABLE verification_logs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    badge_id       UUID NOT NULL REFERENCES badges(id),
    verifier_ip    TEXT,                          -- 검증자 IP (hashed for privacy)
    result         TEXT NOT NULL
                   CHECK (result IN ('valid', 'invalid', 'revoked', 'expired')),
    failure_reason TEXT,                          -- 실패 사유
    verified_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================================
-- 5. key_history (키 교체 이력)
-- ============================================================================
CREATE TABLE key_history (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issuer_id    UUID NOT NULL REFERENCES issuers(id),
    public_key   BYTEA NOT NULL,                -- Ed25519 공개키
    activated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ,                   -- NULL = 현재 활성 키
    tx_hash      TEXT                            -- Polygon 트랜잭션 해시
);

-- ============================================================================
-- Indexes
-- ============================================================================

-- badges: fast lookup by recipient, achievement, and status filtering.
CREATE INDEX idx_badges_recipient_email ON badges (recipient_email);
CREATE INDEX idx_badges_achievement_id  ON badges (achievement_id);
CREATE INDEX idx_badges_status          ON badges (status);

-- verification_logs: lookup by badge for history queries.
CREATE INDEX idx_verification_logs_badge_id ON verification_logs (badge_id);

-- key_history: lookup active/historical keys per issuer.
CREATE INDEX idx_key_history_issuer_id ON key_history (issuer_id);

COMMIT;
