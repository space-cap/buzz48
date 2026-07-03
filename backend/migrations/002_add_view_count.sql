-- 002_add_view_count.sql
-- posts 테이블에 조회수(방문 수) 컬럼을 추가합니다.
ALTER TABLE posts ADD COLUMN view_count INT NOT NULL DEFAULT 0;
