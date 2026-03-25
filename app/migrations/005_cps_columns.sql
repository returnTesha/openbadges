-- 005_cps_columns.sql
-- CPS 연동용 대학/프로그램/학번 컬럼 추가

BEGIN;

ALTER TABLE badges ADD COLUMN university_code TEXT;
ALTER TABLE badges ADD COLUMN program_id TEXT;
ALTER TABLE badges ADD COLUMN student_id TEXT;

COMMIT;
