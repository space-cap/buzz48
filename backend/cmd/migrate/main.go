package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	dsn := buildDSN()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("🔌 Neon PostgreSQL 연결 중...")
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 연결 실패: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)
	fmt.Println("✅ 연결 성공")

	// migrations/ 폴더 경로 (이 파일 기준 두 단계 위 → backend/migrations/)
	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")

	sqlFile := filepath.Join(migrationsDir, "001_initial_schema.sql")
	sql, err := os.ReadFile(sqlFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ SQL 파일 읽기 실패: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📄 마이그레이션 실행 중: %s\n", filepath.Base(sqlFile))

	_, err = conn.Exec(ctx, string(sql))
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 마이그레이션 실패:\n%v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ 마이그레이션 완료!")
	fmt.Println()

	// 생성된 테이블 목록 확인
	rows, err := conn.Query(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  테이블 목록 조회 실패: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("📋 생성된 테이블 목록:")
	for rows.Next() {
		var name string
		rows.Scan(&name)
		fmt.Printf("   - %s\n", name)
	}
}

func buildDSN() string {
	host := getEnv("DB_HOST", "ep-young-breeze-aoz29ou7.c-2.ap-southeast-1.aws.neon.tech")
	port := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "neondb")
	user := getEnv("DB_USER", "neondb_owner")
	password := getEnv("DB_PASSWORD", "")
	sslMode := getEnv("DB_SSL_MODE", "require")

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbName, sslMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
