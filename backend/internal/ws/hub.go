package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	rdb "github.com/redis/go-redis/v9"

	"buzz48/backend/internal/model"
	iredis "buzz48/backend/internal/redis"
)

// Hub는 모든 활성 WebSocket 커넥션과 Redis Pub/Sub 채널을 관리합니다.
type Hub struct {
	redisClient *rdb.Client
	mu          sync.RWMutex
	// posts: postID -> 클라이언트 세트
	posts map[string]map[*Client]bool
	// subs: postID -> Redis PubSub 구독 중단 신호용 채널
	subs map[string]chan struct{}
}

// NewHub는 새로운 Hub 인스턴스를 생성합니다.
func NewHub(r *rdb.Client) *Hub {
	return &Hub{
		redisClient: r,
		posts:       make(map[string]map[*Client]bool),
		subs:        make(map[string]chan struct{}),
	}
}

// Register는 클라이언트를 특정 게시물 허브에 등록하고 첫 접속 시 Pub/Sub 구독을 켭니다.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	postID := c.PostID
	if _, exists := h.posts[postID]; !exists {
		h.posts[postID] = make(map[*Client]bool)
		// 첫 접속이므로 Redis Pub/Sub 구독 고루틴 시작
		stopChan := make(chan struct{})
		h.subs[postID] = stopChan
		go h.subscribePost(postID, stopChan)
	}
	h.posts[postID][c] = true
}

// Unregister는 클라이언트를 해제하고, 해당 게시물의 마지막 클라이언트가 해제되면 Pub/Sub을 끕니다.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	postID := c.PostID
	if clients, exists := h.posts[postID]; exists {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.posts, postID)
			// 더 이상 연결된 클라이언트가 없으므로 Redis Pub/Sub 구독 종료
			if stopChan, subExists := h.subs[postID]; subExists {
				close(stopChan)
				delete(h.subs, postID)
			}
		}
	}
}

// Broadcast는 특정 게시물에 연결된 모든 로컬 클라이언트들에게 메시지를 전송합니다.
func (h *Hub) Broadcast(postID string, event model.WSEvent) {
	h.mu.RLock()
	clients := h.posts[postID]
	h.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal event: %v", err)
		return
	}

	for c := range clients {
		select {
		case c.send <- data:
		default:
			// 버퍼가 꽉 찬 클라이언트는 해제 처리
			c.Close()
		}
	}
}

// subscribePost는 Redis Pub/Sub 채널을 구독하고 발행되는 실시간 메시지를 브로드캐스트합니다.
func (h *Hub) subscribePost(postID string, stopChan chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pubsub := iredis.Subscribe(ctx, h.redisClient, postID)
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("📡 Redis Pub/Sub 채널 구독 시작: %s", postID)

	for {
		select {
		case <-stopChan:
			log.Printf("📴 Redis Pub/Sub 채널 구독 해제: %s", postID)
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			
			// 수신된 JSON을 WSEvent 객체로 복원하여 모든 클라이언트에 브로드캐스트
			var event model.WSEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("invalid PubSub message: %v", err)
				continue
			}

			h.Broadcast(postID, event)
		}
	}
}
