-- 003_nullable_achievement_id.sql
-- MVP: achievement 없이도 배지 발급 가능하도록 achievement_id nullable 변경

BEGIN;

ALTER TABLE badges ALTER COLUMN achievement_id DROP NOT NULL;

COMMIT;
