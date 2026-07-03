package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config는 애플리케이션 전체 환경변수를 보관합니다.
type Config struct {
	AppEnv  string
	APIPort string
	WSPort  string

	RedisHost     string
	RedisPort     string
	RedisPassword string

	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string

	JWTSecret string
}

// Load는 .env 파일을 읽고 Config를 반환합니다.
// .env 파일이 없으면 OS 환경변수만 사용합니다.
func Load() (*Config, error) {
	// 루트의 .env 파일 로드 (없어도 에러 아님)
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	cfg := &Config{
		AppEnv:  getEnv("APP_ENV", "development"),
		APIPort: getEnv("API_PORT", "8080"),
		WSPort:  getEnv("WS_PORT", "8081"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBName:     getEnv("DB_NAME", "buzz48"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBSSLMode:  getEnv("DB_SSL_MODE", "disable"),

		JWTSecret: getEnv("JWT_SECRET", ""),
	}

	if cfg.JWTSecret == "" && cfg.AppEnv == "production" {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}

	return cfg, nil
}

// DBPoolerDSN은 pgx 연결용 DSN을 반환합니다.
func (c *Config) DBDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

// RedisAddr은 Redis 연결 주소를 반환합니다.
func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
