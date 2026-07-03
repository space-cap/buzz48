package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"

	iredis "buzz48/backend/internal/redis"
)

// PostRateLimit는 게시물 작성 시 세션당 1시간 3개 제한을 적용하는 미들웨어입니다.
// PRD §2.1 POST-03, API 명세서 §4
func PostRateLimit(redisClient *rdb.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 세션 ID 추출
		sessionIDStr := c.Cookies("session_id")
		if sessionIDStr == "" {
			sessionIDStr = extractBearer(c.Get("Authorization"))
		}

		if sessionIDStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "SESSION_EXPIRED",
					"message": "세션이 없거나 만료되었습니다.",
				},
			})
		}

		_, err := uuid.Parse(sessionIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "VALIDATION_ERROR",
					"message": "잘못된 세션 ID 형식입니다.",
				},
			})
		}

		// Redis로 Rate Limit 카운트 조회 및 갱신
		count, rlErr := iredis.RateLimitPostCreation(c.Context(), redisClient, sessionIDStr)
		if rlErr != nil {
			// Redis 오류 시 장애 격리 원칙에 따라 통과시키고 에러 로그만 남김
			return c.Next()
		}

		if count > 3 {
			ttl, _ := iredis.RateLimitPostCreationTTL(c.Context(), redisClient, sessionIDStr)
			retryAfter := int64(ttl.Seconds())
			if retryAfter < 0 {
				retryAfter = 0
			}

			c.Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "RATE_LIMITED",
					"message": "게시물 작성 한도를 초과했습니다.",
					"details": fiber.Map{
						"retry_after_seconds": retryAfter,
						"limit":               "3/hour",
					},
				},
			})
		}

		return c.Next()
	}
}

func extractBearer(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return ""
}
