package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	iredis "buzz48/backend/internal/redis"
)

// ────────────────────────────────────────────────────────
// POST /posts — 게시물 작성
// API 명세서 §2.2, PRD §2.1 POST-01~04
// ────────────────────────────────────────────────────────

type createPostReq struct {
	Title          string   `json:"title"`
	Content        string   `json:"content"`
	Category       string   `json:"category"`
	ImageURLs      []string `json:"image_urls"`
	IdempotencyKey string   `json:"idempotency_key"` // PRD §2.2 중복 생성 방지
}

// CreatePost는 게시물을 작성합니다.
func (d *Deps) CreatePost(c *fiber.Ctx) error {
	ctx := c.Context()

	// 세션 검증
	sessionID, err := sessionFromCookie(c)
	if err != nil {
		return errJSON(c, fiber.StatusUnauthorized, "SESSION_EXPIRED", "세션이 없거나 만료되었습니다.")
	}

	var req createPostReq
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "요청 형식이 잘못되었습니다.")
	}

	// 입력 검증
	if len(req.Title) == 0 || len(req.Title) > 100 {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "제목은 1~100자여야 합니다.")
	}
	if len(req.Content) > 3000 {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "본문은 최대 3,000자입니다.")
	}
	if req.Category == "" {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "카테고리를 선택해주세요.")
	}
	if len(req.ImageURLs) > 4 {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "이미지는 최대 4장입니다.")
	}

	// Idempotency Key 처리 (PRD §2.2)
	if req.IdempotencyKey != "" {
		existing, err := findPostByIdempotency(ctx, d.DB, req.IdempotencyKey)
		if err == nil && existing != nil {
			// 이미 생성된 게시물 반환
			return c.Status(fiber.StatusCreated).JSON(existing)
		}
	}

	// DB 삽입
	postID := uuid.New()
	now := time.Now().UTC()
	_, err = d.DB.Exec(ctx,
		`INSERT INTO posts
		 (id, creator_session_id, title, content, category, image_urls, created_at, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		postID, sessionID, req.Title, req.Content, req.Category,
		req.ImageURLs, now, nullableString(req.IdempotencyKey),
	)
	if err != nil {
		// Rate Limit 카운트 롤백 (생성 실패 시)
		// 실제 카운트가 증가했으므로 -1 처리는 생략 (슬라이딩 윈도우로 자동 만료)
		return errJSON(c, fiber.StatusInternalServerError, "DB_ERROR", "게시물 저장 실패: "+err.Error())
	}

	// HOT 랭킹 초기 등록 (스코어 0)
	_ = iredis.HotUpsert(context.Background(), d.Redis, postID.String(), 0)

	resp := fiber.Map{
		"post_id":          postID,
		"title":            req.Title,
		"category":         req.Category,
		"state":            "LIVE",
		"created_at":       now,
		"expires_read_at":  now.Add(24 * time.Hour),
		"expires_delete_at": now.Add(48 * time.Hour),
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// ────────────────────────────────────────────────────────
// GET /posts — 게시판 목록 (커서 페이지네이션)
// API 명세서 §2.2
// ────────────────────────────────────────────────────────

// ListPosts는 게시판 목록을 커서 기반 페이지네이션으로 반환합니다.
func (d *Deps) ListPosts(c *fiber.Ctx) error {
	ctx := c.Context()

	category := c.Query("category", "")
	sort := c.Query("sort", "latest")
	cursor := c.Query("cursor", "")
	search := c.Query("search", "")
	limit := c.QueryInt("limit", 20)
	if limit > 50 {
		limit = 50
	}

	if sort == "hot" {
		return d.listPostsHot(c, limit)
	}
	return d.listPostsLatest(ctx, c, category, cursor, search, limit)
}

// listPostsLatest — 최신순 게시물 목록
func (d *Deps) listPostsLatest(ctx context.Context, c *fiber.Ctx, category, cursor, search string, limit int) error {
	// 커서: created_at (ISO8601) 기반
	var cursorTime time.Time
	if cursor != "" {
		var err error
		cursorTime, err = time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			cursorTime = time.Now().UTC()
		}
	} else {
		cursorTime = time.Now().UTC()
	}

	// 48시간 내 게시물만 조회 (DELETE 상태 제외)
	cutoff := time.Now().UTC().Add(-48 * time.Hour)

	var rows pgx.Rows
	var err error
	if category != "" {
		if search != "" {
			rows, err = d.DB.Query(ctx,
				`SELECT id, title, category, created_at, is_premium, view_count
				 FROM posts
				 WHERE category=$1 AND created_at > $2 AND created_at < $3
				   AND (title ILIKE $4 OR content ILIKE $4)
				 ORDER BY created_at DESC
				 LIMIT $5`,
				category, cutoff, cursorTime, "%"+search+"%", limit+1,
			)
		} else {
			rows, err = d.DB.Query(ctx,
				`SELECT id, title, category, created_at, is_premium, view_count
				 FROM posts
				 WHERE category=$1 AND created_at > $2 AND created_at < $3
				 ORDER BY created_at DESC
				 LIMIT $4`,
				category, cutoff, cursorTime, limit+1,
			)
		}
	} else {
		if search != "" {
			rows, err = d.DB.Query(ctx,
				`SELECT id, title, category, created_at, is_premium, view_count
				 FROM posts
				 WHERE created_at > $1 AND created_at < $2
				   AND (title ILIKE $3 OR content ILIKE $3)
				 ORDER BY created_at DESC
				 LIMIT $4`,
				cutoff, cursorTime, "%"+search+"%", limit+1,
			)
		} else {
			rows, err = d.DB.Query(ctx,
				`SELECT id, title, category, created_at, is_premium, view_count
				 FROM posts
				 WHERE created_at > $1 AND created_at < $2
				 ORDER BY created_at DESC
				 LIMIT $3`,
				cutoff, cursorTime, limit+1,
			)
		}
	}
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "DB_ERROR", err.Error())
	}
	defer rows.Close()

	type PostItem struct {
		PostID           string `json:"post_id"`
		Title            string `json:"title"`
		Category         string `json:"category"`
		State            string `json:"state"`
		ConnCount        int64  `json:"conn_count"`
		ViewCount        int64  `json:"view_count"`
		RemainingSeconds int64  `json:"remaining_seconds"`
		IsNew            bool   `json:"is_new"`
		IsPremium        bool   `json:"is_premium"`
	}

	posts := make([]PostItem, 0, limit)
	var lastCreatedAt time.Time

	for rows.Next() {
		var id uuid.UUID
		var title, category string
		var createdAt time.Time
		var isPremium bool
		var dbViews int64

		if err := rows.Scan(&id, &title, &category, &createdAt, &isPremium, &dbViews); err != nil {
			continue
		}
		lastCreatedAt = createdAt

		// 상태 계산 (이중 검증 원칙)
		elapsed := time.Since(createdAt)
		state := "LIVE"
		remaining := int64((24 * time.Hour - elapsed).Seconds())
		if elapsed >= 24*time.Hour {
			state = "READ"
			remaining = int64((48*time.Hour - elapsed).Seconds())
		}
		if remaining < 0 {
			remaining = 0
		}

		// 동접자 수 및 실시간 조회수 병합 조회
		connCount, _ := iredis.ConnCount(ctx, d.Redis, id.String())
		redisViews, _ := iredis.PostViewCount(ctx, d.Redis, id.String())
		totalViews := dbViews + redisViews

		posts = append(posts, PostItem{
			PostID:           id.String(),
			Title:            title,
			Category:         category,
			State:            state,
			ConnCount:        connCount,
			ViewCount:        totalViews,
			RemainingSeconds: remaining,
			IsNew:            elapsed < 5*time.Minute,
			IsPremium:        isPremium,
		})
	}

	// 다음 커서 계산
	var nextCursor *string
	if len(posts) > limit {
		posts = posts[:limit]
		t := lastCreatedAt.Format(time.RFC3339Nano)
		nextCursor = &t
	}

	return c.JSON(fiber.Map{
		"posts":       posts,
		"next_cursor": nextCursor,
	})
}

// listPostsHot — HOT 스코어순 목록 (Redis Sorted Set 활용)
func (d *Deps) listPostsHot(c *fiber.Ctx, limit int) error {
	ctx := c.Context()

	hotList, err := iredis.HotTop(ctx, d.Redis, 3)
	if err != nil || len(hotList) == 0 {
		return c.JSON(fiber.Map{"posts": []fiber.Map{}})
	}

	posts := make([]fiber.Map, 0, len(hotList))
	for _, z := range hotList {
		postID := z.Member.(string)
		connCount, _ := iredis.ConnCount(ctx, d.Redis, postID)

		// DB에서 게시물 기본 정보 조회
		var title, category string
		var createdAt time.Time
		err := d.DB.QueryRow(ctx,
			`SELECT title, category, created_at FROM posts WHERE id=$1`,
			postID,
		).Scan(&title, &category, &createdAt)
		if err != nil {
			continue
		}

		remaining := int64((24*time.Hour - time.Since(createdAt)).Seconds())
		if remaining < 0 {
			remaining = 0
		}

		posts = append(posts, fiber.Map{
			"post_id":           postID,
			"title":             title,
			"category":          category,
			"conn_count":        connCount,
			"score":             z.Score,
			"remaining_seconds": remaining,
		})
	}

	return c.JSON(fiber.Map{"posts": posts})
}

// ────────────────────────────────────────────────────────
// GET /posts/hot — HOT 상위 3개 전용
// API 명세서 §2.2
// ────────────────────────────────────────────────────────

// GetHotPosts는 HOT 상위 3개 게시물을 반환합니다.
func (d *Deps) GetHotPosts(c *fiber.Ctx) error {
	return d.listPostsHot(c, 3)
}

// ────────────────────────────────────────────────────────
// GET /posts/:post_id — 게시물 상세
// API 명세서 §2.2
// ────────────────────────────────────────────────────────

// GetPost는 게시물 상세 정보를 반환합니다.
// 상태는 created_at 기준으로 매 요청마다 재계산합니다 (PRD §4.2 이중 검증).
func (d *Deps) GetPost(c *fiber.Ctx) error {
	ctx := c.Context()

	postIDStr := c.Params("post_id")
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "잘못된 게시물 ID입니다.")
	}

	createdAtVal := c.Locals("post_created_at")
	elapsedVal := c.Locals("post_elapsed")
	if createdAtVal == nil || elapsedVal == nil {
		return errJSON(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "미들웨어 초기화 오류")
	}
	createdAt := createdAtVal.(time.Time)
	elapsed := elapsedVal.(time.Duration)

	var (
		title            string
		content          string
		category         string
		isPremium        bool
		creatorSessionID uuid.UUID
		dbViews          int64
	)

	err = d.DB.QueryRow(ctx,
		`SELECT title, content, category, is_premium, creator_session_id, view_count
		 FROM posts WHERE id=$1`,
		postID,
	).Scan(&title, &content, &category, &isPremium, &creatorSessionID, &dbViews)

	if err == pgx.ErrNoRows {
		return errJSON(c, fiber.StatusNotFound, "POST_NOT_FOUND", "요청한 게시물을 찾을 수 없습니다.")
	}
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "DB_ERROR", err.Error())
	}

	state := "LIVE"
	remaining := int64((24 * time.Hour - elapsed).Seconds())
	if elapsed >= 24*time.Hour {
		state = "READ"
		remaining = int64((48*time.Hour - elapsed).Seconds())
	}
	if remaining < 0 {
		remaining = 0
	}

	// 상세 진입 시 조회수 카운팅 (중복 조회 방지 가드 작동)
	if state == "LIVE" {
		sID, sErr := sessionFromCookie(c)
		sessionIDStr := "anonymous"
		if sErr == nil {
			sessionIDStr = sID.String()
		}
		_, _ = iredis.PostViewIncr(ctx, d.Redis, sessionIDStr, postID.String())
	}

	connCount, _ := iredis.ConnCount(ctx, d.Redis, postID.String())
	redisViews, _ := iredis.PostViewCount(ctx, d.Redis, postID.String())
	totalViews := dbViews + redisViews

	creatorNick, _ := d.getSessionNickname(ctx, creatorSessionID)

	return c.JSON(fiber.Map{
		"post_id":           postID,
		"title":             title,
		"content":           content,
		"category":          category,
		"state":             state,
		"conn_count":        connCount,
		"view_count":        totalViews,
		"remaining_seconds": remaining,
		"is_premium":        isPremium,
		"creator_nickname":  creatorNick,
		"created_at":        createdAt,
	})
}

// ────────────────────────────────────────────────────────
// GET /posts/:post_id/messages — READ 전용 메시지 조회
// API 명세서 §2.3, PRD §4.2
// ────────────────────────────────────────────────────────

// GetMessages는 READ 상태 게시물의 과거 메시지를 커서 기반으로 반환합니다.
func (d *Deps) GetMessages(c *fiber.Ctx) error {
	ctx := c.Context()

	postIDStr := c.Params("post_id")
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		return errJSON(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "잘못된 게시물 ID입니다.")
	}

	cursor := c.Query("cursor", "")
	limit := c.QueryInt("limit", 50)
	if limit > 100 {
		limit = 100
	}

	// 미들웨어에서 추출한 createdAt 활용
	createdAtVal := c.Locals("post_created_at")
	if createdAtVal == nil {
		return errJSON(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "미들웨어 초기화 오류")
	}

	// 커서 파싱
	var cursorTime time.Time
	if cursor != "" {
		cursorTime, _ = time.Parse(time.RFC3339Nano, cursor)
	} else {
		cursorTime = time.Now().UTC()
	}

	// messages_archive 테이블에서 조회 (READ 상태 데이터는 PostgreSQL에 저장, sessions 테이블 JOIN)
	rows, err := d.DB.Query(ctx,
		`SELECT m.id, m.sender_session_id, s.nickname, m.content, m.reply_to_id, m.is_deleted, m.created_at
		 FROM messages_archive m
		 JOIN sessions s ON m.sender_session_id = s.id
		 WHERE m.post_id=$1 AND m.created_at < $2
		 ORDER BY m.created_at DESC
		 LIMIT $3`,
		postID, cursorTime, limit+1,
	)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "DB_ERROR", err.Error())
	}
	defer rows.Close()

	type MsgItem struct {
		MessageID      string     `json:"message_id"`
		SenderNickname string     `json:"sender_nickname"`
		Content        string     `json:"content"`
		ReplyTo        *string    `json:"reply_to,omitempty"`
		IsDeleted      bool       `json:"is_deleted"`
		CreatedAt      time.Time  `json:"created_at"`
	}

	messages := make([]MsgItem, 0, limit)
	var lastCreatedAt time.Time

	for rows.Next() {
		var (
			msgID     uuid.UUID
			senderID  uuid.UUID
			nickname  string
			content   string
			replyTo   *uuid.UUID
			isDeleted bool
			createdAt time.Time
		)
		if err := rows.Scan(&msgID, &senderID, &nickname, &content, &replyTo, &isDeleted, &createdAt); err != nil {
			continue
		}
		lastCreatedAt = createdAt

		// 삭제된 메시지는 내용을 블라인드 처리 (PRD §6.2)
		if isDeleted {
			content = "신고에 의해 숨겨진 메시지입니다."
		}

		var replyToStr *string
		if replyTo != nil {
			s := replyTo.String()
			replyToStr = &s
		}

		messages = append(messages, MsgItem{
			MessageID:      msgID.String(),
			SenderNickname: nickname,
			Content:        content,
			ReplyTo:        replyToStr,
			IsDeleted:      isDeleted,
			CreatedAt:      createdAt,
		})
	}

	var nextCursor *string
	if len(messages) > limit {
		messages = messages[:limit]
		t := lastCreatedAt.Format(time.RFC3339Nano)
		nextCursor = &t
	}

	return c.JSON(fiber.Map{
		"messages":    messages,
		"next_cursor": nextCursor,
	})
}

// ────────────────────────────────────────────────────────
// 내부 헬퍼
// ────────────────────────────────────────────────────────

// findPostByIdempotency는 동일 idempotency_key로 이미 생성된 게시물을 조회합니다.
func findPostByIdempotency(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, key string) (fiber.Map, error) {
	var (
		id        uuid.UUID
		title     string
		category  string
		createdAt time.Time
	)
	err := db.QueryRow(ctx,
		`SELECT id, title, category, created_at FROM posts WHERE idempotency_key=$1`,
		key,
	).Scan(&id, &title, &category, &createdAt)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(createdAt)
	state := "LIVE"
	if elapsed >= 24*time.Hour {
		state = "READ"
	}

	return fiber.Map{
		"post_id":           id.String(),
		"title":             title,
		"category":          category,
		"state":             state,
		"created_at":        createdAt,
		"expires_read_at":   createdAt.Add(24 * time.Hour),
		"expires_delete_at": createdAt.Add(48 * time.Hour),
	}, nil
}

// nullableString은 빈 문자열을 nil로 변환합니다 (DB nullable 컬럼용).
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
