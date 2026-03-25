-- 006_achievements_issuer_index.sql
-- ListAchievements WHERE issuer_id 성능 개선

BEGIN;

CREATE INDEX idx_achievements_issuer_id ON achievements (issuer_id);

COMMIT;
