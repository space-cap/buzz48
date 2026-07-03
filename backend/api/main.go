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

	// Fiber 앱 생성
	app := fiber.New(fiber.Config{
		AppName:      "Buzz48 API v1",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	// 미들웨어
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000",
		AllowHeaders: "Origin, Content-Type, Authorization",
	}))

	// 라우트
	registerRoutes(app)

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

func registerRoutes(app *fiber.App) {
	v1 := app.Group("/v1")

	// 헬스체크
	v1.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "buzz48-api"})
	})

	// Phase 1에서 구현 예정
	v1.Get("/sessions", placeholder("sessions"))
	v1.Post("/sessions", placeholder("sessions"))
	v1.Get("/rooms", placeholder("rooms"))
	v1.Post("/rooms", placeholder("rooms"))
	v1.Get("/rooms/hot", placeholder("rooms/hot"))
	v1.Get("/rooms/:room_id", placeholder("rooms/:room_id"))
	v1.Get("/rooms/:room_id/messages", placeholder("rooms/:room_id/messages"))
}

func placeholder(name string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error":   "not_implemented",
			"message": name + " endpoint — Phase 1에서 구현 예정",
		})
	}
}
