package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostGuard는 게시물의 존재 여부와 수명 상태(LIVE/READ/DELETE)를 이중 검증하는 미들웨어입니다.
// 48시간이 경과한(DELETE 상태) 게시물은 404 에러(POST_NOT_FOUND)를 반환합니다.
func PostGuard(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		postIDStr := c.Params("post_id")
		if postIDStr == "" {
			return c.Next()
		}

		postID, err := uuid.Parse(postIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "VALIDATION_ERROR",
					"message": "잘못된 게시물 ID 형식입니다.",
				},
			})
		}

		var createdAt time.Time
		err = db.QueryRow(c.Context(), `SELECT created_at FROM posts WHERE id=$1`, postID).Scan(&createdAt)
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "POST_NOT_FOUND",
					"message": "요청한 게시물을 찾을 수 없습니다.",
				},
			})
		} else if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "DB_ERROR",
					"message": "데이터베이스 조회 오류",
				},
			})
		}

		elapsed := time.Since(createdAt)

		// PRD §4.2: 48시간이 지난 게시물은 영구 삭제 대상이며 더 이상 조회할 수 없습니다.
		if elapsed >= 48*time.Hour {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "POST_NOT_FOUND",
					"message": "이 게시물은 만료되어 삭제되었습니다.",
				},
			})
		}

		// 요청 컨텍스트에 수명 관련 상태를 주입하여 핸들러에서 재조회하지 않도록 최적화합니다.
		c.Locals("post_created_at", createdAt)
		c.Locals("post_elapsed", elapsed)

		return c.Next()
	}
}
