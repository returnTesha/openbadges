-- 002_add_credential_fields.sql
-- badges: OB 3.0 credential ID, 블록체인 기록 필드 추가
-- verification_logs: 외부 검증 지원, 상세 결과 저장

BEGIN;

-- ============================================================================
-- 1. badges — credential / blockchain 필드
-- ============================================================================
ALTER TABLE badges ADD COLUMN credential_id TEXT UNIQUE;
ALTER TABLE badges ADD COLUMN blockchain_tx_hash TEXT;
ALTER TABLE badges ADD COLUMN blockchain_hash BYTEA;

CREATE INDEX idx_badges_credential_id ON badges (credential_id);

-- ============================================================================
-- 2. verification_logs — 외부 검증 + 상세 결과
-- ============================================================================
ALTER TABLE verification_logs ALTER COLUMN badge_id DROP NOT NULL;
ALTER TABLE verification_logs ADD COLUMN credential_id TEXT;
ALTER TABLE verification_logs ADD COLUMN issuer_did TEXT;
ALTER TABLE verification_logs ADD COLUMN detail JSONB;

COMMIT;
