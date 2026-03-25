-- 004_credential_sequence.sql
-- 글로벌 credential_id 시퀀스 (연도별 리셋 없음)
-- credential_id 형식: {연도}{시퀀스} (예: 20261, 20262, ...)

BEGIN;

CREATE SEQUENCE badge_credential_seq START WITH 1 INCREMENT BY 1 NO CYCLE;

COMMIT;
