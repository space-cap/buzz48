package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	rdb "github.com/redis/go-redis/v9"

	"buzz48/backend/internal/config"
	"buzz48/backend/internal/db"
	"buzz48/backend/internal/redis"
	"buzz48/backend/internal/ws"
)

var (
	hub               *ws.Hub
	globalDBPool      *pgxpool.Pool
	globalRedisClient *rdb.Client
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
	globalDBPool = pool
	log.Println("✅ PostgreSQL 연결 완료")

	// Redis 연결
	rdbClient, err := redis.NewClient(cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		log.Fatalf("redis connect: %v", err)
	}
	defer rdbClient.Close()
	globalRedisClient = rdbClient
	log.Println("✅ Redis 연결 완료")

	// WebSocket Hub 초기화
	hub = ws.NewHub(rdbClient)

	// HTTP 라우터
	mux := http.NewServeMux()
	
	// 헬스체크 매핑 (로컬 경로 및 운영 /ws 경로 동시 지원)
	mux.HandleFunc("/v1/health", healthHandler)
	mux.HandleFunc("/ws/v1/health", healthHandler)
	mux.HandleFunc("/ws/health", healthHandler)

	// WebSocket 연결 매핑 (로컬 경로 및 운영 /ws 경로 동시 지원)
	mux.HandleFunc("/v1/ws/posts/", wsHandler)
	mux.HandleFunc("/ws/v1/ws/posts/", wsHandler)
	mux.HandleFunc("/ws/posts/", wsHandler)

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
	// URL 경로 예시: /v1/ws/posts/{post_id}
	// /v1/ws/posts/ 접두사 이후의 문자열을 post_id로 추출
	const prefix = "/v1/ws/posts/"
	if len(r.URL.Path) <= len(prefix) {
		http.Error(w, "Bad Request: Missing post_id", http.StatusBadRequest)
		return
	}
	postID := r.URL.Path[len(prefix):]
	if postID == "" {
		http.Error(w, "Bad Request: Missing post_id", http.StatusBadRequest)
		return
	}

	// db pool과 redis, w, r을 넘겨서 클라이언트 소켓 연결 시작
	ws.CreateWSClient(hub, globalDBPool, globalRedisClient, w, r, postID)
}
