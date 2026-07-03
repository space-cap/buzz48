package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"

	"buzz48/backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ 설정 로드 실패: %v", err)
	}
	dsn := cfg.DBDSN()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("🔌 Neon PostgreSQL 연결 중...")
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("❌ 연결 실패: %v", err)
	}
	defer conn.Close(ctx)
	fmt.Println("✅ 연결 성공")

	// ── 1단계: 스키마 초기화 (DROP + CREATE) ──
	fmt.Println("🗑️  기존 스키마 삭제 중...")
	_, err = conn.Exec(ctx, `
		DROP SCHEMA public CASCADE;
		CREATE SCHEMA public;
		GRANT ALL ON SCHEMA public TO PUBLIC;
	`)
	if err != nil {
		log.Fatalf("❌ 스키마 초기화 실패: %v", err)
	}
	fmt.Println("✅ 스키마 초기화 완료 (모든 테이블 삭제됨)")

	// ── 2단계: 마이그레이션 실행 ──
	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")

	sqlFile := filepath.Join(migrationsDir, "001_initial_schema.sql")
	sql, err := os.ReadFile(sqlFile)
	if err != nil {
		log.Fatalf("❌ SQL 파일 읽기 실패: %v", err)
	}

	fmt.Printf("📄 마이그레이션 실행 중: %s\n", filepath.Base(sqlFile))
	_, err = conn.Exec(ctx, string(sql))
	if err != nil {
		log.Fatalf("❌ 마이그레이션 실패:\n%v", err)
	}
	fmt.Println("✅ 마이그레이션 완료!")
	fmt.Println()

	// ── 3단계: 생성된 테이블 목록 확인 ──
	rows, err := conn.Query(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`)
	if err != nil {
		log.Fatalf("❌ 테이블 목록 조회 실패: %v", err)
	}
	defer rows.Close()

	fmt.Println("📋 생성된 테이블 목록:")
	for rows.Next() {
		var name string
		rows.Scan(&name)
		fmt.Printf("   - %s\n", name)
	}
}
