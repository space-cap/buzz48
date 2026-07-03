package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	rdb "github.com/redis/go-redis/v9"

	"buzz48/backend/internal/config"
	"buzz48/backend/internal/db"
	"buzz48/backend/internal/model"
	"buzz48/backend/internal/redis"
)

func main() {
	// 1. 설정 로드
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load fail: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. DB 및 Redis 연결
	pool, err := db.NewPool(ctx, cfg.DBDSN())
	if err != nil {
		log.Fatalf("db connection fail: %v", err)
	}
	defer pool.Close()
	log.Println("🔋 Worker: PostgreSQL 연결 완료")

	rdbClient, err := redis.NewClient(cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		log.Fatalf("redis connection fail: %v", err)
	}
	defer rdbClient.Close()
	log.Println("🔋 Worker: Redis 연결 완료")

	// 3. 각 워커 고루틴 구동
	go startHotScoreWorker(ctx, pool, rdbClient)
	go startLifecycleWorker(ctx, pool, rdbClient)
	go startPurgeWorker(ctx, pool, rdbClient)

	log.Println("🚀 모든 배치 워커(HOT Score, Lifecycle, Purge) 기동 완료")

	// Graceful Shutdown 대기
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("⏳ 배치 워커 종료 중...")
	cancel() // 고루틴에 중단 신호 파급
	time.Sleep(2 * time.Second)
	log.Println("✅ 모든 배치 워커가 정상 종료되었습니다.")
}

// ──────────────────────────────────────────────────────────────
// 1. HOT Score Worker (10초 주기)
// Score = (5분 채팅 수 * 0.7) + (5분 반응 수 * 0.3)
// ──────────────────────────────────────────────────────────────
func startHotScoreWorker(ctx context.Context, pool *pgxpool.Pool, rdbClient *rdb.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runHotScore(ctx, pool, rdbClient)
		}
	}
}

func runHotScore(ctx context.Context, pool *pgxpool.Pool, rdbClient *rdb.Client) {
	// 24시간 내의 LIVE 게시글 ID만 DB에서 획득
	rows, err := pool.Query(ctx, `SELECT id FROM posts WHERE created_at > now() - interval '24 hours'`)
	if err != nil {
		log.Printf("⚠️ [HotScore] DB 조회 실패: %v", err)
		return
	}
	defer rows.Close()

	nowMs := time.Now().UnixMilli()
	livePostIDs := make(map[string]bool)

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}
		postID := id.String()
		livePostIDs[postID] = true

		// 최근 5분 슬라이딩 ZCOUNT
		msgTimesKey := redis.KeyPostMsgTimes(postID)
		reactTimesKey := redis.KeyPostReactionTimes(postID)

		// 5분 이전 활동 Prune 정리
		_ = redis.ActivityPrune5Min(ctx, rdbClient, msgTimesKey, nowMs)
		_ = redis.ActivityPrune5Min(ctx, rdbClient, reactTimesKey, nowMs)

		// 최근 5분 이벤트 카운팅
		msgCount, _ := redis.ActivityCount5Min(ctx, rdbClient, msgTimesKey, nowMs)
		reactCount, _ := redis.ActivityCount5Min(ctx, rdbClient, reactTimesKey, nowMs)

		// 스코어링 공식 적용
		score := (float64(msgCount) * 0.7) + (float64(reactCount) * 0.3)

		// HOT 랭킹 갱신
		_ = redis.HotUpsert(ctx, rdbClient, postID, score)
	}

	// 24시간이 경과해 READ로 전환된 글은 HOT 랭킹 Sorted Set에서 제외시킵니다.
	allHotList, err := rdbClient.ZRevRange(ctx, redis.KeyHotRanking(), 0, -1).Result()
	if err == nil {
		for _, postID := range allHotList {
			if !livePostIDs[postID] {
				// LIVE 목록에 없음 -> 랭킹에서 제거
				_ = redis.HotRemove(ctx, rdbClient, postID)
			}
		}
	}
}

// ──────────────────────────────────────────────────────────────
// 2. Lifecycle Worker (1분 주기)
// LIVE(0~24h) -> READ(24~48h) 상태 전환 및 Redis Stream 해제
// ──────────────────────────────────────────────────────────────
func startLifecycleWorker(ctx context.Context, pool *pgxpool.Pool, rdbClient *rdb.Client) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runLifecycleTransition(ctx, pool, rdbClient)
		}
	}
}

func runLifecycleTransition(ctx context.Context, pool *pgxpool.Pool, rdbClient *rdb.Client) {
	// 생성된 지 24시간이 지난 게시물 중, 아직 상태 캐시가 READ가 아닌 것들을 감지
	rows, err := pool.Query(ctx, `
		SELECT id FROM posts 
		WHERE created_at <= now() - interval '24 hours' 
		  AND created_at > now() - interval '48 hours'
	`)
	if err != nil {
		log.Printf("⚠️ [Lifecycle] DB 조회 실패: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}
		postID := id.String()

		// 상태 캐시 더블 체크
		cachedState, _ := redis.StateCacheGet(ctx, rdbClient, postID)
		if cachedState == "READ" {
			continue // 이미 처리됨
		}

		// 1. 상태 캐시 설정 (READ, TTL 24시간)
		_ = redis.StateCacheSet(ctx, rdbClient, postID, "READ", 24*time.Hour)

		// 2. Redis Stream에 보관 중이던 실시간 메시지 큐 삭제 (메모리 해제)
		_ = redis.StreamDel(ctx, rdbClient, postID)

		// 3. 상태 변경 이벤트 발행 (WebSocket 멀티 인스턴스 전파)
		stateEvent := model.WSEvent{
			Type: model.WSEventStateChanged,
			Payload: map[string]string{
				"post_id": postID,
				"state":   "READ",
			},
		}
		eventData, _ := json.Marshal(stateEvent)
		_ = redis.Publish(ctx, rdbClient, postID, string(eventData))

		log.Printf("💡 [Lifecycle] 게시물 %s 상태 전환 완료: LIVE -> READ", postID)
	}
}

// ──────────────────────────────────────────────────────────────
// 3. Purge Worker (10분 주기)
// READ(24~48h) -> DELETE(48h~) 파기 및 진행 중 신고 보관 처리
// ──────────────────────────────────────────────────────────────
func startPurgeWorker(ctx context.Context, pool *pgxpool.Pool, rdbClient *rdb.Client) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runPurgePipeline(ctx, pool, rdbClient)
		}
	}
}

func runPurgePipeline(ctx context.Context, pool *pgxpool.Pool, rdbClient *rdb.Client) {
	// 생성 후 48시간이 지난 만료된 게시물 조회
	rows, err := pool.Query(ctx, `SELECT id FROM posts WHERE created_at <= now() - interval '48 hours'`)
	if err != nil {
		log.Printf("⚠️ [Purge] DB 조회 실패: %v", err)
		return
	}
	defer rows.Close()

	var expiredPostIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err == nil {
			expiredPostIDs = append(expiredPostIDs, id)
		}
	}

	if len(expiredPostIDs) == 0 {
		return
	}

	log.Printf("🗑️ [Purge] 파기 대상 게시물 %d건 감지. 순차 배치 삭제를 시작합니다.", len(expiredPostIDs))

	for _, postID := range expiredPostIDs {
		// 트랜잭션 수립하여 원자적 처리 보장
		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Printf("⚠️ [Purge] 트랜잭션 생성 실패 (%s): %v", postID, err)
			continue
		}

		// 신고 검토 중이거나 이의 제기가 얽힌 법적 보관 대상 메시지 추출 (PRD §4.2)
		// target_type='message' 이고 status가 pending, reviewing인 건
		holdRows, err := tx.Query(ctx, `
			SELECT id, sender_session_id, content, created_at 
			FROM messages_archive 
			WHERE post_id = $1 
			  AND (is_legal_hold = true OR id IN (
			      SELECT target_id FROM reports 
			      WHERE target_type = 'message' AND status IN ('pending', 'reviewing')
			  ))
		`, postID)

		if err == nil {
			var holds []struct {
				id       uuid.UUID
				senderID uuid.UUID
				content  string
				created  time.Time
			}
			for holdRows.Next() {
				var item struct {
					id       uuid.UUID
					senderID uuid.UUID
					content  string
					created  time.Time
				}
				if err := holdRows.Scan(&item.id, &item.senderID, &item.content, &item.created); err == nil {
					holds = append(holds, item)
				}
			}
			holdRows.Close()

			// 법적 아카이브로 이관 (최대 30일 보관)
			for _, item := range holds {
				_, _ = tx.Exec(ctx, `
					INSERT INTO legal_hold_archive 
					(id, original_message_id, post_id, content, sender_session_id, created_at, hold_reason, expires_at)
					VALUES ($1, $1, $2, $3, $4, $5, 'report_pending', now() + interval '30 days')
					ON CONFLICT DO NOTHING
				`, item.id, postID, item.content, item.senderID, item.created)
			}
		}

		// 해당 게시물의 반응 아카이브 일괄 삭제
		_, _ = tx.Exec(ctx, `DELETE FROM reactions_archive WHERE message_id IN (SELECT id FROM messages_archive WHERE post_id = $1)`, postID)

		// 해당 게시물의 메시지 아카이브 일괄 삭제
		_, _ = tx.Exec(ctx, `DELETE FROM messages_archive WHERE post_id = $1`, postID)

		// 게시물 원본 삭제
		_, _ = tx.Exec(ctx, `DELETE FROM posts WHERE id = $1`, postID)

		if err := tx.Commit(ctx); err != nil {
			log.Printf("⚠️ [Purge] 트랜잭션 커밋 실패 (%s): %v", postID, err)
			_ = tx.Rollback(ctx)
			continue
		}

		// Redis 자원 영구 삭제
		_ = redis.HotRemove(ctx, rdbClient, postID.String())
		_ = rdbClient.Del(ctx, redis.KeyPostConnCount(postID.String())).Err()
		_ = rdbClient.Del(ctx, redis.KeyPostStateCache(postID.String())).Err()
		_ = rdbClient.Del(ctx, redis.KeyPostMsgTimes(postID.String())).Err()
		_ = rdbClient.Del(ctx, redis.KeyPostReactionTimes(postID.String())).Err()

		log.Printf("🗑️ [Purge] 게시물 %s 및 관련 데이터 완전 영구 파기 완료", postID)
	}
}
