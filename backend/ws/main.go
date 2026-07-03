package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"buzz48/backend/internal/config"
	"buzz48/backend/internal/db"
	"buzz48/backend/internal/redis"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Phase 1에서 Origin 검증 강화
		return true
	},
}

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

	// HTTP 라우터
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", healthHandler)
	mux.HandleFunc("/v1/ws/posts/", wsHandler)

	srv := &http.Server{
		Addr:         ":" + cfg.WSPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // WebSocket: 타임아웃 없음
		IdleTimeout:  60 * time.Second,
	}

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("🚀 WebSocket 서버 시작: ws://localhost:%s", cfg.WSPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ws server: %v", err)
		}
	}()

	<-quit
	log.Println("⏳ WebSocket 서버 종료 중...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("✅ WebSocket 서버 종료 완료")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","service":"buzz48-ws"}`))
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("🔌 WebSocket 연결: %s", r.RemoteAddr)

	// Phase 1에서 실제 핸들러 구현 예정
	conn.WriteJSON(map[string]string{
		"type":    "error",
		"message": "WebSocket handler — Phase 1에서 구현 예정",
	})
}
