package handler

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	iredis "buzz48/backend/internal/redis"
)

// ────────────────────────────────────────────────────────
// 닉네임 자동 생성 사전
// PRD §1.2 SES-02: [상태·감정 수식어] + [명사] 조합
// ────────────────────────────────────────────────────────

var adjectives = []string{
	"졸린 눈의", "커피 네 잔째인", "분노의", "설레는", "철학적인",
	"배고픈", "몽상가", "냉소적인", "열정 넘치는", "조용한",
	"수상한", "활기찬", "고독한", "익살스러운", "진지한",
	"예민한", "무심한", "쾌활한", "신중한", "즉흥적인",
}

var nouns = []string{
	"해달", "직장인", "키보드 워리어", "코딩 고수", "밤부엉이",
	"낮잠 챔피언", "탐정", "여행자", "철학자", "고양이",
	"수달", "청설모", "라면 덕후", "주식 투자자", "게임 폐인",
	"독서광", "등산가", "커피 중독자", "축구팬", "음악가",
}

// randomNickname은 랜덤 닉네임을 생성합니다.
func randomNickname() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	adj := adjectives[r.Intn(len(adjectives))]
	noun := nouns[r.Intn(len(nouns))]
	return adj + " " + noun
}

// ────────────────────────────────────────────────────────
// POST /sessions — 익명 세션 발급
// API 명세서 §2.1, PRD §1.2 SES-01, SES-02
// ────────────────────────────────────────────────────────

// CreateSession은 익명 세션을 발급합니다.
func (d *Deps) CreateSession(c *fiber.Ctx) error {
	ctx := c.Context()

	sessionID := uuid.New()
	nickname := randomNickname()
	now := time.Now().UTC()

	// PostgreSQL에 세션 저장
	_, err := d.DB.Exec(ctx,
		`INSERT INTO sessions (id, nickname, created_at, last_active_at)
		 VALUES ($1, $2, $3, $3)`,
		sessionID, nickname, now,
	)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "DB_ERROR", "세션 생성 실패: "+err.Error())
	}

	// Redis 세션 캐시 (Hash, TTL 90일)
	_ = iredis.SessionSet(context.Background(), d.Redis, sessionID.String(), map[string]interface{}{
		"nickname":   nickname,
		"created_at": now.Format(time.RFC3339),
	}, 90*24*time.Hour)

	// 응답 쿠키 설정 (HttpOnly, Secure)
	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    sessionID.String(),
		HTTPOnly: true,
		SameSite: "Strict",
		MaxAge:   60 * 60 * 24 * 90, // 90일
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"session_id": sessionID,
		"nickname":   nickname,
		"created_at": now,
	})
}

// GetMySession은 현재 요청자의 세션 정보를 조회하여 반환합니다.
func (d *Deps) GetMySession(c *fiber.Ctx) error {
	ctx := c.Context()
	sessionID, err := sessionFromCookie(c)
	if err != nil {
		return errJSON(c, fiber.StatusUnauthorized, "SESSION_EXPIRED", "세션이 없거나 만료되었습니다.")
	}

	nickname, err := d.getSessionNickname(ctx, sessionID)
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "SESSION_EXPIRED", "세션을 찾을 수 없습니다.")
	}

	return c.JSON(fiber.Map{
		"session_id": sessionID,
		"nickname":   nickname,
	})
}

// ────────────────────────────────────────────────────────
// PATCH /sessions/me/nickname — 닉네임 변경
// API 명세서 §2.1, PRD §1.2 SES-03
// ────────────────────────────────────────────────────────

type patchNicknameReq struct {
	Nickname string `json:"nickname"`
}

// PatchNickname은 세션 닉네임을 변경합니다.
func (d *Deps) PatchNickname(c *fiber.Ctx) error {
	ctx := c.Context()

	// 세션 추출
	sessionID, err := sessionFromCookie(c)
	if err != nil {
		return errJSON(c, fiber.StatusUnauthorized, "SESSION_EXPIRED", "세션이 없거나 만료되었습니다.")
	}

	var req patchNicknameReq
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "요청 형식이 잘못되었습니다.")
	}
	if len(req.Nickname) == 0 || len(req.Nickname) > 50 {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "닉네임은 1~50자여야 합니다.")
	}

	// TODO Phase 2: 금칙어 필터 적용 (PRD §1.3)

	// DB 업데이트
	tag, err := d.DB.Exec(ctx,
		`UPDATE sessions SET nickname=$1 WHERE id=$2`,
		req.Nickname, sessionID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		return errJSON(c, fiber.StatusNotFound, "SESSION_EXPIRED", "세션을 찾을 수 없습니다.")
	}

	// Redis 캐시 갱신
	_ = iredis.SessionSet(context.Background(), d.Redis, sessionID.String(), map[string]interface{}{
		"nickname": req.Nickname,
	}, 90*24*time.Hour)

	return c.JSON(fiber.Map{
		"nickname":   req.Nickname,
		"updated_at": time.Now().UTC(),
	})
}

// ────────────────────────────────────────────────────────
// 내부 헬퍼
// ────────────────────────────────────────────────────────

// sessionFromCookie는 쿠키에서 session_id를 파싱합니다.
func sessionFromCookie(c *fiber.Ctx) (uuid.UUID, error) {
	raw := c.Cookies("session_id")
	if raw == "" {
		// Authorization 헤더 Bearer 토큰도 허용 (로그인 유저)
		raw = extractBearer(c.Get("Authorization"))
	}
	if raw == "" {
		return uuid.Nil, fmt.Errorf("session not found")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid session id: %w", err)
	}
	return id, nil
}

// extractBearer는 "Bearer {token}" 형식에서 토큰 부분을 추출합니다.
func extractBearer(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return ""
}

// getSessionNickname은 세션 닉네임을 DB에서 조회합니다. (캐시 미스 대비)
func (d *Deps) getSessionNickname(ctx context.Context, sessionID uuid.UUID) (string, error) {
	// 1. Redis 캐시 우선 조회
	cached, err := iredis.SessionGet(ctx, d.Redis, sessionID.String())
	if err == nil && cached != nil {
		if nick, ok := cached["nickname"]; ok {
			return nick, nil
		}
	}

	// 2. 캐시 미스 → DB 조회
	var nickname string
	err = d.DB.QueryRow(ctx,
		`SELECT nickname FROM sessions WHERE id=$1`, sessionID,
	).Scan(&nickname)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("session not found")
		}
		return "", err
	}
	return nickname, nil
}
