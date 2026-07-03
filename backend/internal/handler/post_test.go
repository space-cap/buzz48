package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"buzz48/backend/internal/config"
	"buzz48/backend/internal/db"
	iredis "buzz48/backend/internal/redis"
)

// TestPostLifecycle은 세션 발급부터 게시글 작성까지의 API 동작을 검증하는 통합 테스트입니다.
// 실행 명령어: go test -v ./internal/handler
func TestPostLifecycle(t *testing.T) {
	// 1. 설정 로드
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load: %v", err)
	}

	ctx := context.Background()

	// 2. DB 및 Redis 연결
	pool, err := db.NewPool(ctx, cfg.DBDSN())
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	rdbClient, err := iredis.NewClient(cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		t.Fatalf("redis connect: %v", err)
	}
	defer rdbClient.Close()

	// 3. Deps 의존성 생성
	h := &Deps{
		DB:    pool,
		Redis: rdbClient,
	}

	// 4. 테스트용 임시 Fiber 앱 셋업
	app := fiber.New()
	app.Post("/sessions", h.CreateSession)
	app.Post("/posts", h.CreatePost)

	// ── STEP 1: 세션 생성 API 테스트 ──
	t.Run("Create Session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/sessions", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("session req fail: %v", err)
		}
		if resp.StatusCode != fiber.StatusCreated {
			t.Errorf("expected status 201, got %d", resp.StatusCode)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode fail: %v", err)
		}

		if _, exists := body["session_id"]; !exists {
			t.Errorf("missing session_id in response")
		}
		if _, exists := body["nickname"]; !exists {
			t.Errorf("missing nickname in response")
		}
	})

	// ── STEP 2: 게시물 작성 API 테스트 ──
	t.Run("Create Post", func(t *testing.T) {
		// 테스트용 임시 세션 생성
		sessionID := uuid.New()
		nickname := "테스터 해달"
		_, err := pool.Exec(ctx,
			`INSERT INTO sessions (id, nickname, created_at, last_active_at) VALUES ($1, $2, now(), now())`,
			sessionID, nickname,
		)
		if err != nil {
			t.Fatalf("failed to create test session: %v", err)
		}
		defer func() {
			// 테스트 세션 클린업
			_, _ = pool.Exec(ctx, `DELETE FROM sessions WHERE id=$1`, sessionID)
		}()

		// 테스트용 게시물 바디 데이터 정의
		postReq := map[string]interface{}{
			"title":           "유닛 테스트 제목",
			"content":         "유닛 테스트 본문 내용입니다.",
			"category":        "자유",
			"image_urls":      []string{"https://test.com/image.jpg"},
			"idempotency_key": "test-key-" + uuid.New().String(),
		}
		bodyBytes, _ := json.Marshal(postReq)

		req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		// Authorization 헤더에 테스트 세션 주입
		req.Header.Set("Authorization", "Bearer "+sessionID.String())

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("post req fail: %v", err)
		}
		if resp.StatusCode != fiber.StatusCreated {
			t.Errorf("expected status 201, got %d", resp.StatusCode)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode fail: %v", err)
		}

		postIDStr, exists := body["post_id"].(string)
		if !exists || postIDStr == "" {
			t.Errorf("missing post_id in response")
		}

		// 생성된 테스트 게시물 클린업
		if postIDStr != "" {
			postID, _ := uuid.Parse(postIDStr)
			_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE id=$1`, postID)
			_ = iredis.HotRemove(ctx, rdbClient, postIDStr)
		}
	})
}
