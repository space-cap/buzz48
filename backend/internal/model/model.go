package model

import (
	"time"

	"github.com/google/uuid"
)

// --- 공통 ---

// RoomState는 토론방 상태를 나타냅니다. DB에 저장되지 않고 created_at으로 재계산됩니다.
type RoomState string

const (
	RoomStateLive   RoomState = "LIVE"   // 0~24h
	RoomStateRead   RoomState = "READ"   // 24~48h
	RoomStateDelete RoomState = "DELETE" // 48h 초과
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

// --- 토론방 ---

// Room은 토론방 기본 정보를 나타냅니다.
type Room struct {
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

// State는 created_at 기준으로 현재 방 상태를 계산합니다.
func (r *Room) State() RoomState {
	elapsed := time.Since(r.CreatedAt)
	switch {
	case elapsed < 24*time.Hour:
		return RoomStateLive
	case elapsed < 48*time.Hour:
		return RoomStateRead
	default:
		return RoomStateDelete
	}
}

// --- 메시지 ---

// Message는 채팅 메시지를 나타냅니다.
type Message struct {
	ID              uuid.UUID  `json:"id"`
	RoomID          uuid.UUID  `json:"room_id"`
	SenderSessionID uuid.UUID  `json:"sender_session_id"`
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
	TargetType        string     `json:"target_type"` // "message" | "room"
	TargetID          uuid.UUID  `json:"target_id"`
	ReporterSessionID uuid.UUID  `json:"reporter_session_id"`
	Reason            string     `json:"reason"`
	Status            string     `json:"status"` // pending | reviewing | confirmed | dismissed
	CreatedAt         time.Time  `json:"created_at"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
}
