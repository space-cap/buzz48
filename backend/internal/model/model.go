package model

import (
	"time"

	"github.com/google/uuid"
)

// --- 공통 ---

// PostState는 게시물 상태를 나타냅니다. DB에 저장되지 않고 created_at으로 재계산됩니다.
type PostState string

const (
	PostStateLive   PostState = "LIVE"   // 0~24h: 실시간 채팅 가능
	PostStateRead   PostState = "READ"   // 24~48h: 읽기 전용
	PostStateDelete PostState = "DELETE" // 48h 초과: 파기 대상
)

// --- 세션 ---

// Session은 익명 세션을 나타냅니다.
type Session struct {
	ID           uuid.UUID  `json:"id"`
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	Nickname     string     `json:"nickname"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
}

// --- 게시물 ---

// Post는 게시판 게시물을 나타냅니다.
// State(LIVE/READ/DELETE)는 DB에 저장하지 않고 created_at 기준으로 항상 재계산합니다.
type Post struct {
	ID               uuid.UUID  `json:"id"`
	CreatorSessionID uuid.UUID  `json:"creator_session_id"`
	CreatorUserID    *uuid.UUID `json:"creator_user_id,omitempty"`
	Title            string     `json:"title"`
	Content          string     `json:"content"`
	Category         string     `json:"category"`
	ImageURLs        []string   `json:"image_urls,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	IsPremium        bool       `json:"is_premium"`
	ArchivedToPGAt   *time.Time `json:"archived_to_pg_at,omitempty"`
}

// State는 created_at 기준으로 현재 게시물 상태를 계산합니다.
// PRD §4.2, 아키텍처 §3.2 이중 검증 원칙 구현.
func (p *Post) State() PostState {
	elapsed := time.Since(p.CreatedAt)
	switch {
	case elapsed < 24*time.Hour:
		return PostStateLive
	case elapsed < 48*time.Hour:
		return PostStateRead
	default:
		return PostStateDelete
	}
}

// TimeRemaining은 현재 상태에서 다음 상태로 전환까지 남은 시간을 반환합니다.
func (p *Post) TimeRemaining() time.Duration {
	elapsed := time.Since(p.CreatedAt)
	switch {
	case elapsed < 24*time.Hour:
		return 24*time.Hour - elapsed // LIVE → READ 까지
	case elapsed < 48*time.Hour:
		return 48*time.Hour - elapsed // READ → DELETE 까지
	default:
		return 0
	}
}

// --- 메시지 ---

// Message는 채팅 메시지를 나타냅니다.
type Message struct {
	ID              uuid.UUID  `json:"id"`
	PostID          uuid.UUID  `json:"post_id"`
	SenderSessionID uuid.UUID  `json:"sender_session_id"`
	SenderNickname  string     `json:"sender_nickname"`
	ReplyToID       *uuid.UUID `json:"reply_to_id,omitempty"`
	Content         string     `json:"content"`
	CreatedAt       time.Time  `json:"created_at"`
	IsDeleted       bool       `json:"is_deleted"`
	IsLegalHold     bool       `json:"is_legal_hold"`
}

// --- 신고 ---

// Report는 신고 정보를 나타냅니다.
type Report struct {
	ID                uuid.UUID  `json:"id"`
	TargetType        string     `json:"target_type"` // "message" | "post"
	TargetID          uuid.UUID  `json:"target_id"`
	ReporterSessionID uuid.UUID  `json:"reporter_session_id"`
	Reason            string     `json:"reason"`
	Status            string     `json:"status"` // pending | reviewing | confirmed | dismissed
	CreatedAt         time.Time  `json:"created_at"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
}

// --- WebSocket 이벤트 ---

// WSEventType은 WebSocket 이벤트 타입입니다.
type WSEventType string

const (
	WSEventPostSnapshot   WSEventType = "post_snapshot"    // 연결 시 최근 메시지 50개
	WSEventMessageRecv    WSEventType = "message_received"  // 새 메시지 수신
	WSEventReactionUpdate WSEventType = "reaction_updated"  // 이모지 반응 갱신
	WSEventConnCount      WSEventType = "conn_count_updated" // 동접자 수 갱신
	WSEventStateChanged   WSEventType = "post_state_changed" // LIVE→READ 전환
	WSEventSyncResult     WSEventType = "sync_result"       // 재연결 시 누락 메시지
	WSEventError          WSEventType = "error"             // 오류
)

// WSEvent는 WebSocket 이벤트 페이로드입니다.
type WSEvent struct {
	Type    WSEventType `json:"type"`
	Payload any         `json:"payload"`
}
