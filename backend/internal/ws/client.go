package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	rdb "github.com/redis/go-redis/v9"

	"buzz48/backend/internal/model"
	iredis "buzz48/backend/internal/redis"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// CORS 허용 (개발 시)
		return true
	},
}

// Client는 단일 WebSocket 연결과 데이터를 주고받는 객체입니다.
type Client struct {
	Hub             *Hub
	Conn            *websocket.Conn
	PostID          string
	SessionID       uuid.UUID
	Nickname        string
	DB              *pgxpool.Pool
	Redis           *rdb.Client
	send            chan []byte
	once            sync.Once
}

// CreateWSClient는 업그레이드 과정을 수행하고 Client를 활성화시킵니다.
func CreateWSClient(hub *Hub, db *pgxpool.Pool, r *rdb.Client, w http.ResponseWriter, req *http.Request, postID string) {
	// 1. 토큰(세션) 검증
	token := req.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Unauthorized: Token required", http.StatusUnauthorized)
		return
	}

	sessionID, err := uuid.Parse(token)
	if err != nil {
		http.Error(w, "Unauthorized: Invalid token format", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Redis 또는 DB에서 닉네임 가져오기
	var nickname string
	cached, _ := iredis.SessionGet(ctx, r, sessionID.String())
	if cached != nil {
		nickname = cached["nickname"]
	}

	if nickname == "" {
		// DB 조회
		err = db.QueryRow(ctx, `SELECT nickname FROM sessions WHERE id=$1`, sessionID).Scan(&nickname)
		if err != nil {
			http.Error(w, "Unauthorized: Session not found", http.StatusUnauthorized)
			return
		}
	}

	// 2. 게시물 존재 및 LIVE 상태 이중 검증
	var createdAt time.Time
	err = db.QueryRow(ctx, `SELECT created_at FROM posts WHERE id=$1`, postID).Scan(&createdAt)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	elapsed := time.Since(createdAt)
	if elapsed >= 48*time.Hour {
		http.Error(w, "Post expired and deleted", http.StatusGone)
		return
	}

	// 3. Upgrade
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := &Client{
		Hub:       hub,
		Conn:      conn,
		PostID:    postID,
		SessionID: sessionID,
		Nickname:  nickname,
		DB:        db,
		Redis:     r,
		send:      make(chan []byte, 256),
	}

	// Hub 등록
	client.Hub.Register(client)

	// Redis 동접자 수 증가 (60초 유예 처리 포함)
	connCount, _ := iredis.ConnIncr(context.Background(), r, sessionID.String(), postID)
	// 동접자 변경 브로드캐스트
	hEvent := model.WSEvent{
		Type:    model.WSEventConnCount,
		Payload: map[string]int64{"conn_count": connCount},
	}
	hEventData, _ := json.Marshal(hEvent)
	_ = iredis.Publish(context.Background(), r, postID, string(hEventData))

	// 4. 연결 직후 snapshot 전송
	client.sendSnapshot(createdAt)

	// 5. 펌프 구동
	go client.writePump()
	go client.readPump()
}

// Close는 안전하게 클라이언트 연결을 닫습니다.
func (c *Client) Close() {
	c.once.Do(func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
		close(c.send)

		// 동접자 수 감소 (60초 유예 어뷰징 방어 처리)
		ctx := context.Background()
		_ = iredis.ConnDecr(ctx, c.Redis, c.SessionID.String(), c.PostID)

		// 60초 뒤에 실제로 접속 안 했으면 깎는 타이머 고루틴
		go func(sID, pID string) {
			time.Sleep(61 * time.Second)
			_ = iredis.ConnFlush(context.Background(), c.Redis, sID, pID)
			// 동접자 수 재조회 후 변경 통지
			cnt, _ := iredis.ConnCount(context.Background(), c.Redis, pID)
			hEv := model.WSEvent{
				Type:    model.WSEventConnCount,
				Payload: map[string]int64{"conn_count": cnt},
			}
			hEvData, _ := json.Marshal(hEv)
			_ = iredis.Publish(context.Background(), c.Redis, pID, string(hEvData))
		}(c.SessionID.String(), c.PostID)
	})
}

// sendSnapshot은 연결 즉시 최신 메시지 50개와 상태를 1회 전송합니다.
func (c *Client) sendSnapshot(createdAt time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

	// Redis Stream에서 최근 메시지 조회 (오래된 것부터 정렬을 위해 XRevRangeN 후 역순 정렬)
	xmsgs, _ := iredis.StreamRange(ctx, c.Redis, c.PostID, "", 50)
	recentMessages := make([]model.Message, 0, len(xmsgs))

	// XRevRangeN 결과는 최신 순이므로, 사용자에게 보낼 때는 시간 순서(오래된 것부터)로 뒤집어서 추가
	for i := len(xmsgs) - 1; i >= 0; i-- {
		xm := xmsgs[i]
		
		var replyTo *uuid.UUID
		if rID, ok := xm.Values["reply_to_id"].(string); ok && rID != "" {
			parsed, _ := uuid.Parse(rID)
			replyTo = &parsed
		}

		// message_id 필드 우선 사용, 없으면 uuid.New()로 임시 발급 (오래된 스트림 메시지 대응)
		var msgID uuid.UUID
		if idStr, ok := xm.Values["message_id"].(string); ok && idStr != "" {
			msgID, _ = uuid.Parse(idStr)
		} else {
			msgID = uuid.New()
		}
		createdUnix, _ := strconv.ParseInt(xm.Values["created_at"].(string), 10, 64)

		recentMessages = append(recentMessages, model.Message{
			ID:             msgID,
			PostID:         uuid.MustParse(c.PostID),
			SenderNickname: xm.Values["sender_nickname"].(string),
			Content:        xm.Values["content"].(string),
			ReplyToID:      replyTo,
			CreatedAt:      time.UnixMilli(createdUnix).UTC(),
		})
	}

	connCount, _ := iredis.ConnCount(ctx, c.Redis, c.PostID)

	snapshot := model.WSEvent{
		Type: model.WSEventPostSnapshot,
		Payload: map[string]any{
			"state":             state,
			"conn_count":        connCount,
			"remaining_seconds": remaining,
			"recent_messages":   recentMessages,
		},
	}

	data, _ := json.Marshal(snapshot)
	c.send <- data
}

// Client로부터 들어오는 메시지를 읽어 처리합니다. (Inbound)
func (c *Client) readPump() {
	defer c.Close()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var event struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := json.Unmarshal(message, &event); err != nil {
			c.sendError("VALIDATION_ERROR", "이벤트 형식이 잘못되었습니다.")
			continue
		}

		switch event.Type {
		case "send_message":
			c.handleSendMessage(event.Payload)
		case "toggle_reaction":
			c.handleSendReaction(event.Payload)
		case "sync":
			c.handleSync(event.Payload)
		default:
			c.sendError("INVALID_EVENT", "지원하지 않는 이벤트입니다.")
		}
	}
}

// Client로 메시지를 안전하게 비동기 발송합니다. (Outbound)
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 큐에 대기 중인 다른 메시지들도 한번에 Flush
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendError는 클라이언트 단독으로 에러를 발송합니다.
func (c *Client) sendError(code, message string) {
	errEvent := model.WSEvent{
		Type: model.WSEventError,
		Payload: map[string]string{
			"code":    code,
			"message": message,
		},
	}
	data, _ := json.Marshal(errEvent)
	select {
	case c.send <- data:
	default:
	}
}

type sendMessagePayload struct {
	Content string  `json:"content"`
	ReplyTo *string `json:"reply_to"`
}

// handleSendMessage 처리
func (c *Client) handleSendMessage(payload json.RawMessage) {
	var req sendMessagePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendError("VALIDATION_ERROR", "잘못된 메시지 페이로드입니다.")
		return
	}

	// 0. Rate Limit 검증 (초당 5건)
	count, rlErr := iredis.RateLimitMessageSend(context.Background(), c.Redis, c.SessionID.String())
	if rlErr == nil && count > 5 {
		c.sendError("RATE_LIMITED", "메시지 전송 한도를 초과했습니다 (최대 초당 5건).")
		return
	}

	if len(req.Content) == 0 || len(req.Content) > 500 {
		c.sendError("VALIDATION_ERROR", "메시지는 1~500자여야 합니다.")
		return
	}

	// 1. 상태 이중 검증
	var createdAt time.Time
	err := c.DB.QueryRow(context.Background(), `SELECT created_at FROM posts WHERE id=$1`, c.PostID).Scan(&createdAt)
	if err != nil {
		c.sendError("POST_NOT_FOUND", "게시물을 찾을 수 없습니다.")
		return
	}

	if time.Since(createdAt) >= 24*time.Hour {
		c.sendError("POST_NOT_LIVE", "이 게시물은 읽기 전용 상태입니다.")
		return
	}

	// TODO Phase 2: 금칙어 필터

	// 2. ID 생성 및 시간 기록
	msgID := uuid.New()
	now := time.Now().UTC()
	nowUnix := now.UnixMilli()

	// 3. Redis Stream에 적재 (message_id 명시 저장 → 스냅샷 복원 시 UUID 식별 용도)
	fields := map[string]interface{}{
		"message_id":      msgID.String(),
		"sender_nickname": c.Nickname,
		"content":         req.Content,
		"created_at":      strconv.FormatInt(nowUnix, 10),
	}
	if req.ReplyTo != nil {
		fields["reply_to_id"] = *req.ReplyTo
	}

	_, err = iredis.StreamAdd(context.Background(), c.Redis, c.PostID, fields)
	if err != nil {
		c.sendError("REDIS_ERROR", "메시지 전송 실패")
		return
	}

	// 최근 5분 슬라이딩 윈도우 카운터에 활동 누적
	_ = iredis.ActivityAdd(context.Background(), c.Redis, iredis.KeyPostMsgTimes(c.PostID), msgID.String(), nowUnix)

	// 4. PostgreSQL 비동기 아카이빙 (Write-Behind)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var replyToUUID *uuid.UUID
		if req.ReplyTo != nil && *req.ReplyTo != "" {
			parsed, err := uuid.Parse(*req.ReplyTo)
			if err == nil {
				replyToUUID = &parsed
			}
		}

		_, dbErr := c.DB.Exec(ctx,
			`INSERT INTO messages_archive (id, post_id, sender_session_id, reply_to_id, content, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			msgID, uuid.MustParse(c.PostID), c.SessionID, replyToUUID, req.Content, now,
		)
		if dbErr != nil {
			log.Printf("⚠️ PostgreSQL 비동기 아카이브 실패 (메시지 유실 방지 모니터링 필요): %v", dbErr)
		}
	}()

	// 5. Pub/Sub에 메시지 발행 -> 모든 WS 인스턴스의 구독자들에게 팬아웃
	recvEvent := model.WSEvent{
		Type: model.WSEventMessageRecv,
		Payload: map[string]any{
			"message_id":      msgID.String(),
			"sender_nickname": c.Nickname,
			"content":         req.Content,
			"reply_to":        req.ReplyTo,
			"created_at":      now,
		},
	}
	eventData, _ := json.Marshal(recvEvent)
	_ = iredis.Publish(context.Background(), c.Redis, c.PostID, string(eventData))
}

type sendReactionPayload struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

// handleSendReaction 처리
func (c *Client) handleSendReaction(payload json.RawMessage) {
	var req sendReactionPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendError("VALIDATION_ERROR", "잘못된 반응 페이로드입니다.")
		return
	}

	validEmojis := map[string]bool{"👍": true, "❤️": true, "😂": true, "😡": true}
	if !validEmojis[req.Emoji] {
		c.sendError("VALIDATION_ERROR", "허용되지 않는 이모지입니다.")
		return
	}

	ctx := context.Background()

	// Redis Set을 이용해 해당 메시지에 이 세션이 이미 반응했는지 확인 및 원자적 토글
	reactKey := iredis.KeyPostReactions(c.PostID, req.MessageID)
	member := c.SessionID.String() + ":" + req.Emoji

	exists, _ := c.Redis.SIsMember(ctx, reactKey, member).Result()
	
	var count int64
	var action string

	if exists {
		// 이미 반응함 -> 반응 제거 (토글 취소)
		c.Redis.SRem(ctx, reactKey, member)
		action = "removed"
		
		// DB 비동기 아카이브에서 삭제
		go func() {
			_, _ = c.DB.Exec(context.Background(),
				`DELETE FROM reactions_archive WHERE message_id=$1 AND session_id=$2 AND emoji_type=$3`,
				uuid.MustParse(req.MessageID), c.SessionID, req.Emoji,
			)
		}()
	} else {
		// 반응 없음 -> 반응 추가
		c.Redis.SAdd(ctx, reactKey, member)
		action = "added"

		// 5분 슬라이딩 윈도우 카운터에 활동 누적
		nowUnix := time.Now().UnixMilli()
		_ = iredis.ActivityAdd(ctx, c.Redis, iredis.KeyPostReactionTimes(c.PostID), uuid.New().String(), nowUnix)

		// DB 비동기 아카이브에 삽입
		go func() {
			_, _ = c.DB.Exec(context.Background(),
				`INSERT INTO reactions_archive (message_id, session_id, emoji_type)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (message_id, session_id) DO UPDATE SET emoji_type=$3`,
				uuid.MustParse(req.MessageID), c.SessionID, req.Emoji,
			)
		}()
	}

	// 해당 이모지 전체 카운트 계산을 위해 Set 전체 멤버 조회 후 집계
	members, _ := c.Redis.SMembers(ctx, reactKey).Result()
	for _, m := range members {
		// session_id:emoji 포맷이므로 파싱하여 해당 이모지와 일치하는 것 카운트
		if len(m) > 37 && m[37:] == req.Emoji {
			count++
		}
	}

	// 브로드캐스트 전송
	reactEv := model.WSEvent{
		Type: model.WSEventReactionUpdate,
		Payload: map[string]any{
			"message_id": req.MessageID,
			"emoji":      req.Emoji,
			"count":      count,
			"action":     action,
		},
	}
	reactEvData, _ := json.Marshal(reactEv)
	_ = iredis.Publish(ctx, c.Redis, c.PostID, string(reactEvData))
}

type syncPayload struct {
	LastMessageID string `json:"last_message_id"`
}

// handleSync 처리 (재연결 시 누락 메시지 전송)
func (c *Client) handleSync(payload json.RawMessage) {
	var req syncPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendError("VALIDATION_ERROR", "잘못된 동기화 페이로드입니다.")
		return
	}

	ctx := context.Background()

	// Redis Stream에서 누락 구간메시지 (최대 100건) 가져오기
	xmsgs, err := iredis.StreamRange(ctx, c.Redis, c.PostID, req.LastMessageID, 100)
	if err != nil {
		c.sendError("REDIS_ERROR", "동기화 실패")
		return
	}

	missedMessages := make([]model.Message, 0, len(xmsgs))
	for i := len(xmsgs) - 1; i >= 0; i-- { // 시간 순으로 맞춤
		xm := xmsgs[i]
		
		var replyTo *uuid.UUID
		if rID, ok := xm.Values["reply_to_id"].(string); ok && rID != "" {
			parsed, _ := uuid.Parse(rID)
			replyTo = &parsed
		}

		msgID, _ := uuid.Parse(xm.ID)
		createdUnix, _ := strconv.ParseInt(xm.Values["created_at"].(string), 10, 64)

		missedMessages = append(missedMessages, model.Message{
			ID:             msgID,
			PostID:         uuid.MustParse(c.PostID),
			SenderNickname: xm.Values["sender_nickname"].(string),
			Content:        xm.Values["content"].(string),
			ReplyToID:      replyTo,
			CreatedAt:      time.UnixMilli(createdUnix).UTC(),
		})
	}

	syncResult := model.WSEvent{
		Type: model.WSEventSyncResult,
		Payload: map[string]any{
			"missed_messages": missedMessages,
		},
	}
	data, _ := json.Marshal(syncResult)
	c.send <- data
}
