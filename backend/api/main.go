package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"buzz48/backend/internal/config"
	"buzz48/backend/internal/db"
	"buzz48/backend/internal/handler"
	"buzz48/backend/internal/middleware"
	"buzz48/backend/internal/redis"
)

func main() {
	// 설정 로드
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}

	// DB 연결
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DBDSN())
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()
	log.Println("✅ PostgreSQL 연결 완료")

	// Redis 연결
	rdb, err := redis.NewClient(cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		log.Fatalf("redis connect: %v", err)
	}
	defer rdb.Close()
	log.Println("✅ Redis 연결 완료")

	// 핸들러 의존성 주입
	h := &handler.Deps{
		DB:    pool,
		Redis: rdb,
	}

	// Fiber 앱 생성
	app := fiber.New(fiber.Config{
		AppName:      "Buzz48 API v1",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		// 공통 에러 핸들러
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "INTERNAL_ERROR",
					"message": err.Error(),
				},
			})
		},
	})

	// 미들웨어
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${latency} | ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowOrigins,
		AllowHeaders:     "Origin, Content-Type, Authorization",
		AllowCredentials: true, // 쿠키 허용
	}))

	// 라우트 등록
	registerRoutes(app, h)

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.APIPort
		log.Printf("🚀 API 서버 시작: http://localhost%s", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("server listen: %v", err)
		}
	}()

	<-quit
	log.Println("⏳ API 서버 종료 중...")
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("✅ API 서버 종료 완료")
}

func registerRoutes(app *fiber.App, h *handler.Deps) {
	v1 := app.Group("/v1")

	// ── 헬스체크 ──────────────────────────────────────────
	v1.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "buzz48-api"})
	})

	// ── 실시간 테스트 콘솔 ──────────────────────────────────
	v1.Get("/test", h.TestConsole)

	// ── 세션 API (PRD §1, API 명세서 §2.1) ───────────────
	v1.Post("/sessions", h.CreateSession)
	v1.Get("/sessions/me", h.GetMySession)
	v1.Patch("/sessions/me/nickname", h.PatchNickname)

	// ── 게시물 API (PRD §2, §4, §5, API 명세서 §2.2) ─────
	v1.Post("/posts", middleware.PostRateLimit(h.Redis), h.CreatePost)
	v1.Get("/posts", h.ListPosts)
	v1.Get("/posts/hot", h.GetHotPosts)    // /posts/:id 보다 먼저 등록
	v1.Get("/posts/:post_id", middleware.PostGuard(h.DB), h.GetPost)

	// ── 메시지 API — READ 전용 (API 명세서 §2.3) ──────────
	v1.Get("/posts/:post_id/messages", middleware.PostGuard(h.DB), h.GetMessages)

	// ── 신고 API (Phase 2) ────────────────────────────────
	v1.Post("/reports", placeholder("POST /reports"))
}

func placeholder(name string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error":   "not_implemented",
			"message": name + " — Phase 2에서 구현 예정",
		})
	}
}
