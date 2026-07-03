package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPostgreSQLPing(t *testing.T) {
	host := getEnv("DB_HOST", "ep-young-breeze-aoz29ou7.c-2.ap-southeast-1.aws.neon.tech")
	port := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "neondb")
	user := getEnv("DB_USER", "neondb_owner")
	password := getEnv("DB_PASSWORD", "")
	sslMode := getEnv("DB_SSL_MODE", "require")

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbName, sslMode,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("❌ PostgreSQL 연결 실패: %v", err)
	}
	defer conn.Close(ctx)

	var version string
	err = conn.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		t.Fatalf("❌ 쿼리 실패: %v", err)
	}

	t.Logf("✅ PostgreSQL 연결 성공!")
	t.Logf("   Host: %s:%s", host, port)
	t.Logf("   DB:   %s", dbName)
	t.Logf("   Ver:  %s", version)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
