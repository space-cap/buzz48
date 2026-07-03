-- =============================================================
-- 버즈48 (Buzz48) — 초기 스키마 마이그레이션 v1.1
-- 기반 문서: 05_DB설계서.md v1.1
-- 대상 DB: Neon Serverless PostgreSQL
-- 변경: rooms → posts (게시판 게시물 컨셉 반영)
-- =============================================================

-- -----------------------------------------------
-- §3.1 계정 및 세션
-- -----------------------------------------------

-- 로그인 유저 (소셜 로그인 연동 시에만 생성)
CREATE TABLE IF NOT EXISTS users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    social_provider     VARCHAR(20) NOT NULL,        -- 'google' | 'kakao' | 'naver'
    social_id           VARCHAR(100) NOT NULL,
    nickname            VARCHAR(50) NOT NULL,
    email               VARCHAR(255),
    is_creator          BOOLEAN NOT NULL DEFAULT FALSE, -- PRD §9.2 개설자 인증 여부
    creator_verified_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at       TIMESTAMPTZ,
    UNIQUE (social_provider, social_id)
);

-- 익명 세션 (모든 유저는 익명 세션에서 출발, 로그인 시 user_id 연결)
CREATE TABLE IF NOT EXISTS sessions (
    id              UUID PRIMARY KEY,             -- 클라이언트 발급 UUID v4
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    nickname        VARCHAR(50) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id    ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON sessions(last_active_at); -- 90일 미접속 파기 배치용

-- IP 로그 (개인정보보호법 §5.3, 최대 30일 보관 후 파기)
CREATE TABLE IF NOT EXISTS ip_logs (
    id          BIGSERIAL PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES sessions(id),
    ip_address  INET NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ip_logs_created_at ON ip_logs(created_at); -- 30일 TTL 파기 배치 스캔용

-- -----------------------------------------------
-- §3.2 게시물 및 메시지 아카이브
-- -----------------------------------------------

-- 게시물 (생성 시점 정보만 저장, LIVE 상태 실시간 데이터는 Redis)
-- state 컬럼 없음: created_at으로 항상 재계산 (PRD §4.2, 아키텍처 §3.2 이중 검증 원칙)
CREATE TABLE IF NOT EXISTS posts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_session_id  UUID NOT NULL REFERENCES sessions(id),
    creator_user_id     UUID REFERENCES users(id),
    title               VARCHAR(100) NOT NULL,
    content             TEXT,
    category            VARCHAR(20) NOT NULL,
    image_urls          TEXT[],
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(), -- LIFE-02 판정 기준 시각
    is_premium          BOOLEAN NOT NULL DEFAULT FALSE,
    archived_to_pg_at   TIMESTAMPTZ,                       -- LIVE→READ 전환 시 Redis→PG 이관 완료 시각
    idempotency_key     VARCHAR(100) UNIQUE
);
CREATE INDEX IF NOT EXISTS idx_posts_category_created ON posts(category, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_created_at       ON posts(created_at); -- Lifecycle/Purge Worker 스캔용

-- 메시지 아카이브 (월별 파티션, created_at 기준)
CREATE TABLE IF NOT EXISTS messages_archive (
    id                  UUID NOT NULL DEFAULT gen_random_uuid(),
    post_id             UUID NOT NULL REFERENCES posts(id),
    sender_session_id   UUID NOT NULL REFERENCES sessions(id),
    reply_to_id         UUID,                   -- 자기참조, 파티션 특성상 애플리케이션 레벨 검증
    content             TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_deleted          BOOLEAN NOT NULL DEFAULT FALSE, -- 신고로 인한 블라인드 처리 (MOD-03)
    is_legal_hold       BOOLEAN NOT NULL DEFAULT FALSE, -- PRD §4.2: 신고 처리 중인 콘텐츠는 48h 파기 예외
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- 현재 월 파티션 (2026-07)
CREATE TABLE IF NOT EXISTS messages_archive_2026_07
    PARTITION OF messages_archive
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');

-- 다음 달 파티션 (2026-08) — 미리 생성
CREATE TABLE IF NOT EXISTS messages_archive_2026_08
    PARTITION OF messages_archive
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

CREATE INDEX IF NOT EXISTS idx_msg_archive_post_id ON messages_archive(post_id, created_at);
CREATE INDEX IF NOT EXISTS idx_msg_archive_sender  ON messages_archive(sender_session_id);

-- 이모지 반응 아카이브
-- message_id FK 생략: messages_archive는 파티션 테이블로 PostgreSQL FK 미지원, 애플리케이션 레벨 검증
CREATE TABLE IF NOT EXISTS reactions_archive (
    id          BIGSERIAL PRIMARY KEY,
    message_id  UUID NOT NULL,
    session_id  UUID NOT NULL REFERENCES sessions(id),
    emoji_type  VARCHAR(10) NOT NULL,           -- '👍' | '❤️' | '😂' | '😡'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (message_id, session_id)             -- CHAT-04: 메시지당 1인 1반응
);
CREATE INDEX IF NOT EXISTS idx_reactions_message_id ON reactions_archive(message_id);

-- -----------------------------------------------
-- §3.3 신고 및 제재
-- -----------------------------------------------

CREATE TABLE IF NOT EXISTS reports (
    id                  BIGSERIAL PRIMARY KEY,
    target_type         VARCHAR(10) NOT NULL,    -- 'message' | 'post'
    target_id           UUID NOT NULL,
    reporter_session_id UUID NOT NULL REFERENCES sessions(id),
    reason              VARCHAR(20) NOT NULL,    -- '욕설/혐오' | '허위정보' | '스팸/도배' | '개인정보노출'
    status              VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending | reviewing | confirmed | dismissed
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at         TIMESTAMPTZ,
    reviewer_note       TEXT,
    UNIQUE (target_type, target_id, reporter_session_id) -- MOD-02: 동일 세션 중복 신고는 1건 처리
);
CREATE INDEX IF NOT EXISTS idx_reports_target ON reports(target_type, target_id, created_at);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status) WHERE status = 'pending'; -- 관리자 큐 조회용

-- 제재 이력 (90일 롤링 윈도우로 카운트, PRD §6.2)
CREATE TABLE IF NOT EXISTS sanctions (
    id              BIGSERIAL PRIMARY KEY,
    session_id      UUID REFERENCES sessions(id),
    user_id         UUID REFERENCES users(id),
    violation_type  VARCHAR(20) NOT NULL,        -- '욕설혐오' | '스팸도배' | '허위정보'
    level           SMALLINT NOT NULL,           -- 1 | 2 | 3차
    action          VARCHAR(30) NOT NULL,        -- 'message_delete' | 'chat_ban_24h' | 'permanent_ip_ban'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ                  -- 24h/7일 정지의 만료 시각. 영구 차단은 NULL
);
CREATE INDEX IF NOT EXISTS idx_sanctions_session_recent ON sanctions(session_id, created_at); -- 90일 롤링 카운트 쿼리용

-- 법적 보관 아카이브 (PRD §4.2: 48h 경과해도 신고 처리 중인 콘텐츠는 별도 보관)
CREATE TABLE IF NOT EXISTS legal_hold_archive (
    id                  UUID PRIMARY KEY,
    original_message_id UUID NOT NULL,
    post_id             UUID NOT NULL,
    content             TEXT NOT NULL,
    sender_session_id   UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    hold_reason         VARCHAR(50) NOT NULL,    -- 'report_pending' | 'legal_request'
    expires_at          TIMESTAMPTZ NOT NULL     -- 최대 30일 보관 후 최종 파기
);

-- -----------------------------------------------
-- §3.4 결제 및 정산 (프리미엄 게시물, Phase 3)
-- -----------------------------------------------

CREATE TABLE IF NOT EXISTS premium_posts (
    post_id         UUID PRIMARY KEY REFERENCES posts(id),
    creator_id      UUID NOT NULL REFERENCES users(id),
    entry_price     INTEGER NOT NULL,            -- 인앱 재화 단위
    commission_rate NUMERIC(4,2) NOT NULL DEFAULT 0.25, -- 20~30% 중 정책값
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id),
    post_id             UUID NOT NULL REFERENCES posts(id),
    amount              INTEGER NOT NULL,
    currency            VARCHAR(10) NOT NULL DEFAULT 'KRW',
    pg_transaction_id   VARCHAR(100),            -- PG사 거래 ID
    status              VARCHAR(20) NOT NULL,    -- 'completed' | 'refunded' | 'partial_refunded'
    refund_amount       INTEGER DEFAULT 0,
    refund_reason       VARCHAR(50),             -- PRD §9.2: '5분내취소' | '조기종료비례환불'
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_post_id ON payments(post_id);

CREATE TABLE IF NOT EXISTS settlements (
    id                BIGSERIAL PRIMARY KEY,
    creator_id        UUID NOT NULL REFERENCES users(id),
    period_start      DATE NOT NULL,
    period_end        DATE NOT NULL,
    gross_amount      INTEGER NOT NULL,
    commission_amount INTEGER NOT NULL,
    net_amount        INTEGER NOT NULL,
    status            VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending | paid | carried_over
    paid_at           TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_settlements_creator ON settlements(creator_id, period_start);

-- -----------------------------------------------
-- §3.5 알림 로그
-- -----------------------------------------------

CREATE TABLE IF NOT EXISTS notifications_log (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             UUID REFERENCES users(id),
    session_id          UUID REFERENCES sessions(id),
    post_id             UUID REFERENCES posts(id),
    notification_type   VARCHAR(20) NOT NULL,    -- 'hot_entry' | 'expiry_1h' | 'reply' | 'reaction'
    sent_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_date           DATE NOT NULL DEFAULT CURRENT_DATE -- PRD §7.2 중복 차단 인덱스용 (UTC 기준)
);
-- PRD §7.2: 동일 유저-게시물-유형 1일 1회 제한 (로그인 유저용)
CREATE UNIQUE INDEX IF NOT EXISTS idx_noti_dedup_user ON notifications_log (
    user_id, post_id, notification_type, sent_date
) WHERE user_id IS NOT NULL;

-- PRD §7.2: 동일 세션-게시물-유형 1일 1회 제한 (익명 유저용)
CREATE UNIQUE INDEX IF NOT EXISTS idx_noti_dedup_session ON notifications_log (
    session_id, post_id, notification_type, sent_date
) WHERE session_id IS NOT NULL AND user_id IS NULL;
