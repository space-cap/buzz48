package redis

import "fmt"

// DB설계서 §5 Redis 자료구조 키 네이밍 규칙
// 모든 키는 이 함수를 통해 생성하여 오타·불일치를 방지합니다.

// KeyPostMessages — Stream: LIVE 메시지 큐 (post:{id}:messages)
func KeyPostMessages(postID string) string {
	return fmt.Sprintf("post:%s:messages", postID)
}

// KeyPostConnCount — String: 실시간 동접자 수 INCR/DECR (post:{id}:conn_count)
func KeyPostConnCount(postID string) string {
	return fmt.Sprintf("post:%s:conn_count", postID)
}

// KeyPostMsgTimes — Sorted Set: 최근 5분 채팅 수 (post:{id}:msg_times)
func KeyPostMsgTimes(postID string) string {
	return fmt.Sprintf("post:%s:msg_times", postID)
}

// KeyPostReactionTimes — Sorted Set: 최근 5분 반응 수 (post:{id}:reaction_times)
func KeyPostReactionTimes(postID string) string {
	return fmt.Sprintf("post:%s:reaction_times", postID)
}

// KeyPostReactions — Set: 메시지별 반응 유저 집합 (post:{id}:reactions:{msg_id})
func KeyPostReactions(postID, messageID string) string {
	return fmt.Sprintf("post:%s:reactions:%s", postID, messageID)
}

// KeyPostStateCache — String: LIVE/READ 상태 캐시 (post:{id}:state_cache)
// 진실의 근거는 created_at이며, 이 키는 조회 성능용 캐시입니다.
func KeyPostStateCache(postID string) string {
	return fmt.Sprintf("post:%s:state_cache", postID)
}

// KeyHotRanking — Sorted Set: 전체 게시물 HOT 순위 (hot:ranking)
func KeyHotRanking() string {
	return "hot:ranking"
}

// KeySession — Hash: 세션 캐시 (session:{session_id})
func KeySession(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// KeyRateLimitPostCreation — String counter: 게시물 작성 Rate Limit (rate_limit:post_creation:{session_id})
func KeyRateLimitPostCreation(sessionID string) string {
	return fmt.Sprintf("rate_limit:post_creation:%s", sessionID)
}

// KeyRateLimitMessageSend — String counter: 메시지 전송 Rate Limit 초당 5건 (rate_limit:message_send:{session_id})
func KeyRateLimitMessageSend(sessionID string) string {
	return fmt.Sprintf("rate_limit:message_send:%s", sessionID)
}

// KeyDisconnectPending — String: 재연결 유예 플래그 TTL 60초 (disconnect_pending:{session_id}:{post_id})
func KeyDisconnectPending(sessionID, postID string) string {
	return fmt.Sprintf("disconnect_pending:%s:%s", sessionID, postID)
}

// KeyPubSubPost — Pub/Sub 채널: WebSocket 멀티 인스턴스 팬아웃 (pubsub:post:{post_id})
func KeyPubSubPost(postID string) string {
	return fmt.Sprintf("pubsub:post:%s", postID)
}

// KeyPostViewCount — String: 게시글 실시간 조회수 카운터 (post:{post_id}:view_count)
func KeyPostViewCount(postID string) string {
	return fmt.Sprintf("post:%s:view_count", postID)
}

// KeyPostViewGuard — String: 중복 조회 방지 가드 TTL 10분 (view_guard:{session_id}:{post_id})
func KeyPostViewGuard(sessionID, postID string) string {
	return fmt.Sprintf("view_guard:%s:%s", sessionID, postID)
}
