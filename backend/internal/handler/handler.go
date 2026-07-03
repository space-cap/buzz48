// Package handler는 Buzz48 REST API 핸들러를 담습니다.
// 각 도메인(session, post, message)별로 파일을 분리하여 관리합니다.
package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	rdb "github.com/redis/go-redis/v9"
)

// Deps는 모든 핸들러가 공유하는 의존성 묶음입니다.
type Deps struct {
	DB    *pgxpool.Pool
	Redis *rdb.Client
}

// ErrResponse는 공통 에러 응답 포맷입니다. (API 명세서 §1.3)
type ErrResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errJSON은 에러 응답을 JSON으로 반환하는 헬퍼입니다.
func errJSON(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"error": ErrResponse{Code: code, Message: message},
	})
}
