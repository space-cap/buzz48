package redis

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ──────────────────────────────────────────────────────────────
// § 동접자 카운터 (conn_count)
// 아키텍처 §4.3: 재연결 유예(60초) — 즉시 DECR하지 않고
//               disconnect_pending 키 TTL 60초 방식으로 어뷰징 방어
// ──────────────────────────────────────────────────────────────

// viewIncrScript — 중복 조회 방지 가드(10분 TTL)가 없을 때에만 조회수 INCR
var viewIncrScript = redis.NewScript(`
local exists = redis.call('EXISTS', KEYS[1])
if exists == 0 then
    redis.call('SET', KEYS[1], '1', 'EX', ARGV[1])
    return redis.call('INCR', KEYS[2])
else
    local v = redis.call('GET', KEYS[2])
    if v then return tonumber(v) else return 0 end
end
`)

// PostViewIncr는 중복 검사 후 안전하게 실시간 조회수를 1 증가시킵니다.
func PostViewIncr(ctx context.Context, rdb *redis.Client, sessionID, postID string) (int64, error) {
	keys := []string{
		KeyPostViewGuard(sessionID, postID),
		KeyPostViewCount(postID),
	}
	// 가드 키 10분(600초) 설정
	res, err := viewIncrScript.Run(ctx, rdb, keys, 600).Int64()
	if err != nil {
		return 0, err
	}
	return res, nil
}

// PostViewCount는 Redis 내 실시간 조회수 카운터 값을 조회합니다.
func PostViewCount(ctx context.Context, rdb *redis.Client, postID string) (int64, error) {
	val, err := rdb.Get(ctx, KeyPostViewCount(postID)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// ConnIncr — WebSocket 연결 시 동접자 수 처리.
//   - disconnect_pending 키가 있으면(재연결): 키만 삭제하고 INCR 생략 (이미 카운트됨)
//   - disconnect_pending 키가 없으면(신규 연결): INCR +1
//
// Lua 스크립트로 원자적으로 처리하여 레이스 컨디션 방지.
var connIncrScript = redis.NewScript(`
local pending = redis.call('DEL', KEYS[1])
if pending == 0 then
    return redis.call('INCR', KEYS[2])
else
    local v = redis.call('GET', KEYS[2])
    if v then return tonumber(v) else return 0 end
end
`)

func ConnIncr(ctx context.Context, rdb *redis.Client, sessionID, postID string) (int64, error) {
	keys := []string{
		KeyDisconnectPending(sessionID, postID),
		KeyPostConnCount(postID),
	}
	res, err := connIncrScript.Run(ctx, rdb, keys).Int64()
	if err != nil {
		return 0, err
	}
	return res, nil
}

// ConnDecr — WebSocket 연결 해제 시 disconnect_pending 키 설정(TTL 60초).
// 60초 내 재연결이 없으면 ConnFlush를 통해 실제 DECR 처리합니다.
// (PRD §5.2 어뷰징 방어: 60초 내 재연결은 동일 접속으로 간주)
func ConnDecr(ctx context.Context, rdb *redis.Client, sessionID, postID string) error {
	return rdb.Set(ctx,
		KeyDisconnectPending(sessionID, postID),
		"1",
		60*time.Second,
	).Err()
}

// ConnFlush — disconnect_pending TTL 만료 후 워커가 호출.
// pending 키가 없으면(재연결됨) 실제 DECR 건너뜀.
func ConnFlush(ctx context.Context, rdb *redis.Client, sessionID, postID string) error {
	pendingKey := KeyDisconnectPending(sessionID, postID)
	// pending 키가 남아있을 때만 DECR (아직 재연결 안 된 경우)
	exists, err := rdb.Exists(ctx, pendingKey).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		// 이미 재연결됨 — DECR 불필요
		return nil
	}
	pipe := rdb.Pipeline()
	pipe.Del(ctx, pendingKey)
	pipe.Decr(ctx, KeyPostConnCount(postID))
	_, err = pipe.Exec(ctx)
	return err
}

// ConnCount — 현재 동접자 수 조회 (없으면 0 반환)
func ConnCount(ctx context.Context, rdb *redis.Client, postID string) (int64, error) {
	val, err := rdb.Get(ctx, KeyPostConnCount(postID)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return val, err
}

// ──────────────────────────────────────────────────────────────
// § LIVE 메시지 Stream (post:{id}:messages)
// 아키텍처 §2.3: Redis Stream XADD/XRANGE — 재연결 누락 구간 복구용
// TTL은 LIVE 구간 종료(24h) 후 워커가 명시 삭제
// ──────────────────────────────────────────────────────────────

// StreamAdd — 채팅 메시지를 Redis Stream에 추가. message_id를 자동 생성하여 반환.
func StreamAdd(ctx context.Context, rdb *redis.Client, postID string, fields map[string]interface{}) (string, error) {
	return rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: KeyPostMessages(postID),
		MaxLen: 10000, // 최대 1만 건 자동 트리밍 (~메모리 보호)
		Approx: true,
		Values: fields,
	}).Result()
}

// StreamRange — 특정 구간 메시지 조회 (재연결 sync 시 사용).
// lastID: 마지막으로 수신한 메시지 ID (exclusive). 빈 문자열이면 전체 조회.
func StreamRange(ctx context.Context, rdb *redis.Client, postID, lastID string, count int64) ([]redis.XMessage, error) {
	start := "-"   // 가장 오래된 것까지 범위 확장
	if lastID != "" {
		start = "(" + lastID // exclusive — lastID의 직전(더 최신)까지 역순 스캔
	}
	return rdb.XRevRangeN(ctx, KeyPostMessages(postID), "+", start, count).Result()
}

// StreamDel — READ 전환 후 Redis Stream 정리 (메모리 회수)
func StreamDel(ctx context.Context, rdb *redis.Client, postID string) error {
	return rdb.Del(ctx, KeyPostMessages(postID)).Err()
}

// ──────────────────────────────────────────────────────────────
// § 슬라이딩 윈도우 카운터 — 최근 5분 채팅/반응 수 (msg_times, reaction_times)
// PRD §5.1, 아키텍처 §4.1
// ──────────────────────────────────────────────────────────────

// ActivityAdd — 현재 타임스탬프를 Sorted Set에 추가. ttl은 Set 자체 보관 기간(LIVE 동안).
func ActivityAdd(ctx context.Context, rdb *redis.Client, key, member string, nowUnixMilli int64) error {
	pipe := rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(nowUnixMilli), Member: member})
	pipe.Expire(ctx, key, 25*time.Hour) // LIVE 24h + 여유 1h
	_, err := pipe.Exec(ctx)
	return err
}

// ActivityCount5Min — 최근 5분간 유니크 이벤트 수 반환 (슬라이딩 윈도우).
func ActivityCount5Min(ctx context.Context, rdb *redis.Client, key string, nowUnixMilli int64) (int64, error) {
	min := nowUnixMilli - 5*60*1000 // 5분 = 300,000ms
	return rdb.ZCount(ctx, key, ms2str(min), ms2str(nowUnixMilli)).Result()
}

// ActivityPrune5Min — 5분 이전 항목 정리 (주기적 호출). HOT Score Worker에서 호출.
func ActivityPrune5Min(ctx context.Context, rdb *redis.Client, key string, nowUnixMilli int64) error {
	max := nowUnixMilli - 5*60*1000
	return rdb.ZRemRangeByScore(ctx, key, "-inf", ms2str(max)).Err()
}

// ──────────────────────────────────────────────────────────────
// § HOT 랭킹 Sorted Set (hot:ranking)
// PRD §5.1 HOT-01~03, 아키텍처 §4.2
// ──────────────────────────────────────────────────────────────

// HotUpsert — 게시물의 HOT 스코어를 갱신합니다.
func HotUpsert(ctx context.Context, rdb *redis.Client, postID string, score float64) error {
	return rdb.ZAdd(ctx, KeyHotRanking(), redis.Z{
		Score:  score,
		Member: postID,
	}).Err()
}

// HotTop — HOT 상위 N개 게시물 ID 목록을 스코어와 함께 반환합니다.
func HotTop(ctx context.Context, rdb *redis.Client, n int64) ([]redis.Z, error) {
	return rdb.ZRevRangeWithScores(ctx, KeyHotRanking(), 0, n-1).Result()
}

// HotRemove — DELETE된 게시물을 HOT 랭킹에서 제거합니다.
func HotRemove(ctx context.Context, rdb *redis.Client, postID string) error {
	return rdb.ZRem(ctx, KeyHotRanking(), postID).Err()
}

// ──────────────────────────────────────────────────────────────
// § 게시물 상태 캐시 (post:{id}:state_cache)
// 아키텍처 §3.2: 진실의 근거는 created_at — 이 캐시는 조회 성능용.
// ──────────────────────────────────────────────────────────────

// StateCacheSet — 상태 캐시 저장. TTL은 캐시 유효 기간.
func StateCacheSet(ctx context.Context, rdb *redis.Client, postID, state string, ttl time.Duration) error {
	return rdb.Set(ctx, KeyPostStateCache(postID), state, ttl).Err()
}

// StateCacheGet — 상태 캐시 조회. 캐시 미스 시 빈 문자열 반환.
func StateCacheGet(ctx context.Context, rdb *redis.Client, postID string) (string, error) {
	val, err := rdb.Get(ctx, KeyPostStateCache(postID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return val, err
}

// ──────────────────────────────────────────────────────────────
// § Rate Limit — 게시물 작성 (60분 슬라이딩 윈도우)
// PRD §2.1 POST-03: 세션/계정당 1시간 3개 제한
// ──────────────────────────────────────────────────────────────

// RateLimitPostCreation — 게시물 작성 시도 기록. 현재 카운트 반환.
// 최초 호출 시 TTL 3600초(1시간 슬라이딩 윈도우)를 설정합니다.
func RateLimitPostCreation(ctx context.Context, rdb *redis.Client, sessionID string) (int64, error) {
	key := KeyRateLimitPostCreation(sessionID)
	pipe := rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Hour) // 최초에만 적용; 이후 호출은 무시됨
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

// RateLimitPostCreationTTL — 다음 생성 가능까지 남은 초 반환.
func RateLimitPostCreationTTL(ctx context.Context, rdb *redis.Client, sessionID string) (time.Duration, error) {
	return rdb.TTL(ctx, KeyRateLimitPostCreation(sessionID)).Result()
}

// RateLimitMessageSend — 메시지 전송 시도 기록. 현재 카운트 반환.
// 최초 호출 시 TTL 1초를 설정합니다 (초당 5건 제한).
func RateLimitMessageSend(ctx context.Context, rdb *redis.Client, sessionID string) (int64, error) {
	key := KeyRateLimitMessageSend(sessionID)
	pipe := rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Second) // 1초 유효
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

// ──────────────────────────────────────────────────────────────
// § 세션 캐시 (session:{session_id})  Hash
// ──────────────────────────────────────────────────────────────

// SessionSet — 세션 정보 Hash 저장. TTL은 세션 만료 시간.
func SessionSet(ctx context.Context, rdb *redis.Client, sessionID string, fields map[string]interface{}, ttl time.Duration) error {
	pipe := rdb.Pipeline()
	pipe.HSet(ctx, KeySession(sessionID), fields)
	pipe.Expire(ctx, KeySession(sessionID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// SessionGet — 세션 정보 Hash 전체 반환. 캐시 미스 시 nil map 반환.
func SessionGet(ctx context.Context, rdb *redis.Client, sessionID string) (map[string]string, error) {
	result, err := rdb.HGetAll(ctx, KeySession(sessionID)).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil // 캐시 미스
	}
	return result, nil
}

// SessionDel — 세션 캐시 삭제 (로그아웃/만료 시)
func SessionDel(ctx context.Context, rdb *redis.Client, sessionID string) error {
	return rdb.Del(ctx, KeySession(sessionID)).Err()
}

// ──────────────────────────────────────────────────────────────
// § Pub/Sub 채널 — 멀티 인스턴스 WebSocket 팬아웃
// 아키텍처 §2.2: pubsub:post:{post_id}
// ──────────────────────────────────────────────────────────────

// Publish — 메시지를 Pub/Sub 채널에 발행합니다.
func Publish(ctx context.Context, rdb *redis.Client, postID, message string) error {
	return rdb.Publish(ctx, KeyPubSubPost(postID), message).Err()
}

// Subscribe — Pub/Sub 채널을 구독합니다. 반환된 *PubSub은 호출자가 Close해야 합니다.
func Subscribe(ctx context.Context, rdb *redis.Client, postIDs ...string) *redis.PubSub {
	channels := make([]string, len(postIDs))
	for i, id := range postIDs {
		channels[i] = KeyPubSubPost(id)
	}
	return rdb.Subscribe(ctx, channels...)
}

// ──────────────────────────────────────────────────────────────
// § 내부 유틸
// ──────────────────────────────────────────────────────────────

// ms2str — int64(Unix ms)를 Redis Score 문자열로 변환합니다.
func ms2str(ms int64) string {
	return strconv.FormatInt(ms, 10)
}
