# [API 명세서] 버즈48 (Buzz48)

> **버전**: v1.0 | **작성일**: 2026-07-03 | **기반 문서**: PRD v1.0, 시스템 아키텍처 설계서 v1.0, DB 설계서 v1.0
> **문서 목적**: 프론트엔드/백엔드 분업 개발이 가능한 수준의 REST API 및 WebSocket 이벤트 명세 제공.

---

## 1. 공통 사항

### 1.1 Base URL

```
REST API   : https://api.buzz48.app/v1
WebSocket  : wss://ws.buzz48.app/v1
```

### 1.2 인증

| 방식 | 대상 | 설명 |
| :--- | :--- | :--- |
| 세션 쿠키 | 익명 유저 (기본) | `HttpOnly`, `Secure` 쿠키에 세션 토큰(JWT, session_id 포함) 저장. 최초 접속 시 `POST /sessions`로 발급 |
| Bearer 토큰 | 로그인 유저 | 소셜 로그인 성공 시 발급되는 Access Token을 `Authorization: Bearer {token}` 헤더로 전송 |

WebSocket 연결 시에도 동일한 세션 토큰을 최초 핸드셰이크에서 검증하며, 이후 매 메시지마다 재검증하지 않는다(아키텍처 문서 §8).

### 1.3 공통 에러 응답 포맷

```json
{
  "error": {
    "code": "ROOM_NOT_FOUND",
    "message": "요청한 토론방을 찾을 수 없습니다.",
    "details": {}
  }
}
```

### 1.4 공통 에러 코드

| 코드 | HTTP Status | 설명 |
| :--- | :--- | :--- |
| `SESSION_EXPIRED` | 401 | 세션 만료 또는 유효하지 않음 |
| `ROOM_NOT_FOUND` | 404 | 방이 존재하지 않거나 이미 DELETE 상태 |
| `ROOM_NOT_LIVE` | 403 | LIVE 상태가 아닌 방에 쓰기 시도 (PRD CHAT-01) |
| `RATE_LIMITED` | 429 | Rate Limit 초과 (응답에 `retry_after_seconds` 포함) |
| `VALIDATION_ERROR` | 400 | 요청 필드 검증 실패 |
| `FORBIDDEN_CONTENT` | 422 | 금칙어 필터 1차 차단 (PRD MOD-02) |
| `PAYMENT_FAILED` | 402 | 결제 실패 |

---

## 2. REST API

### 2.1 세션 및 계정

#### `POST /sessions`
최초 접속 시 익명 세션을 발급한다. (PRD SES-01, SES-02)

**Request**: body 없음

**Response** `201 Created`
```json
{
  "session_id": "b3f1...-uuid",
  "nickname": "졸린 눈의 해달",
  "created_at": "2026-07-03T09:00:00Z"
}
```

#### `PATCH /sessions/me/nickname`
닉네임 수정. (PRD SES-03)

**Request**
```json
{ "nickname": "커스텀 닉네임" }
```

**Response** `200 OK`
```json
{ "nickname": "커스텀 닉네임", "updated_at": "2026-07-03T09:05:00Z" }
```
**에러**: 금칙어 포함 시 `FORBIDDEN_CONTENT` (PRD §1.3 커스텀 닉네임도 금칙어 필터 적용)

#### `POST /auth/social/{provider}/callback`
소셜 로그인 콜백 처리(구글·카카오·네이버). 기존 익명 세션의 닉네임·이력을 계정에 귀속. (PRD SES-04)

**Request**
```json
{ "code": "oauth_authorization_code", "session_id": "b3f1...-uuid" }
```

**Response** `200 OK`
```json
{
  "access_token": "jwt...",
  "user_id": "u-uuid",
  "nickname": "커스텀 닉네임",
  "merged_session_id": "b3f1...-uuid"
}
```

---

### 2.2 토론방

#### `POST /rooms`
토론방 생성. (PRD ROOM-01~04)

**Request**
```json
{
  "title": "오늘 코스피 왜 이래요",
  "content": "장 초반부터 이상한데 다들 어떻게 보시나요",
  "category": "경제·주식",
  "image_urls": ["https://cdn.buzz48.app/uploads/xxxx.jpg"],
  "idempotency_key": "client-generated-uuid"
}
```
> `idempotency_key`: 네트워크 재시도 시 중복 생성 방지 (PRD §2.2)

**Response** `201 Created`
```json
{
  "room_id": "r-uuid",
  "title": "오늘 코스피 왜 이래요",
  "category": "경제·주식",
  "state": "LIVE",
  "created_at": "2026-07-03T09:10:00Z",
  "expires_read_at": "2026-07-04T09:10:00Z",
  "expires_delete_at": "2026-07-05T09:10:00Z"
}
```

**에러**: 1시간 3개 초과 시 `RATE_LIMITED`
```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "토론방 생성 한도를 초과했습니다.",
    "details": { "retry_after_seconds": 1800, "limit": "3/hour" }
  }
}
```

#### `GET /rooms`
타임라인 목록 조회. (기획서 §3.1)

**Query Parameters**

| 파라미터 | 설명 | 기본값 |
| :--- | :--- | :--- |
| `category` | 카테고리 필터 | 전체 |
| `sort` | `latest`(최신순, 기본) \| `hot`(HOT 스코어순) | `latest` |
| `cursor` | 페이지네이션 커서 | - |
| `limit` | 페이지 크기 | 20 |

**Response** `200 OK`
```json
{
  "rooms": [
    {
      "room_id": "r-uuid",
      "title": "오늘 코스피 왜 이래요",
      "category": "경제·주식",
      "state": "LIVE",
      "conn_count": 432,
      "remaining_seconds": 29534,
      "is_new": false
    }
  ],
  "next_cursor": "eyJ..."
}
```
> `state`는 서버가 `created_at` 기준으로 매 요청마다 재계산하여 반환(PRD §4.2 이중 검증 원칙 — 캐시 값이 아닌 실제 판정값)

#### `GET /rooms/hot`
HOT 상위 3개 방 조회. (PRD HOT-03)

**Response** `200 OK`
```json
{
  "rooms": [
    { "room_id": "r-uuid-1", "title": "...", "conn_count": 432, "score": 218.4, "remaining_seconds": 29534 }
  ]
}
```

#### `GET /rooms/{room_id}`
토론방 상세(원문 게시글 + 메타 정보). 채팅 메시지 자체는 WebSocket 또는 §2.3 API로 별도 조회.

**Response** `200 OK`
```json
{
  "room_id": "r-uuid",
  "title": "오늘 코스피 왜 이래요",
  "content": "장 초반부터 이상한데...",
  "category": "경제·주식",
  "state": "LIVE",
  "conn_count": 432,
  "remaining_seconds": 29534,
  "is_premium": false,
  "creator_nickname": "커피 네 잔째인 직장인"
}
```
**에러**: 존재하지 않거나 DELETE된 방은 `ROOM_NOT_FOUND` (PRD §4.2 — 전용 종료 안내 페이지로 클라이언트가 처리)

---

### 2.3 메시지 (READ 상태 조회 전용)

LIVE 상태 메시지는 WebSocket으로만 송수신한다. 이 API는 READ 상태로 전환된 방의 과거 메시지를 스크롤 조회할 때만 사용한다(PRD §4.2 커서 기반 페이지네이션).

#### `GET /rooms/{room_id}/messages`

**Query Parameters**: `cursor`, `limit`(기본 50)

**Response** `200 OK`
```json
{
  "messages": [
    {
      "message_id": "m-uuid",
      "sender_nickname": "분노의 키보드 워리어",
      "content": "이거 진짜 심각하네요",
      "reply_to": null,
      "reactions": { "👍": 12, "❤️": 3 },
      "created_at": "2026-07-03T10:00:00Z",
      "is_deleted": false
    }
  ],
  "next_cursor": "eyJ..."
}
```
> `is_deleted: true`인 메시지는 `content`가 `"신고에 의해 숨겨진 메시지입니다"`로 대체되어 반환 (PRD §6.2)

---

### 2.4 신고

#### `POST /reports`
(PRD MOD-01)

**Request**
```json
{
  "target_type": "message",
  "target_id": "m-uuid",
  "reason": "욕설/혐오"
}
```

**Response** `201 Created`
```json
{ "report_id": 10234, "status": "pending" }
```
**에러**: 동일 세션 중복 신고 시 `409 Conflict` (`ALREADY_REPORTED`)

#### `POST /reports/{report_id}/appeal`
AI 필터 오탐 이의제기. (PRD §6.2)

**Response** `202 Accepted`
```json
{ "status": "reviewing", "sla_hours": 24 }
```

---

### 2.5 프리미엄 토론방 결제

#### `POST /rooms/{room_id}/payments`
프리미엄 토론방 입장 결제. (PRD PAY-01, PAY-02)

**Request**
```json
{ "pg_method": "kakaopay" }
```

**Response** `200 OK`
```json
{
  "payment_id": "p-uuid",
  "amount": 3000,
  "currency": "KRW",
  "status": "completed",
  "commission_rate": 0.25
}
```

#### `POST /payments/{payment_id}/refund`
환불 요청. (PRD §9.2 — 5분 이내 미참여 시 100% 환불, 조기 종료 시 비례 환불)

**Response** `200 OK`
```json
{ "payment_id": "p-uuid", "refund_amount": 3000, "refund_reason": "5분내취소" }
```

#### `GET /creators/me/settlements`
개설자 정산 내역 조회. (PRD §9.2 — 익월 15일 정산)

**Response** `200 OK`
```json
{
  "settlements": [
    { "period": "2026-06", "gross_amount": 500000, "commission_amount": 125000, "net_amount": 375000, "status": "paid" }
  ]
}
```

---

## 3. WebSocket API

### 3.1 연결 및 인증

```
wss://ws.buzz48.app/v1/rooms/{room_id}?token={session_token}
```

- 최초 핸드셰이크에서 `token` 검증 후 커넥션에 세션 정보 바인딩.
- 연결 성공 시 서버는 즉시 `room_snapshot` 이벤트로 현재 상태(최근 메시지 50개, 동접자 수, 방 상태)를 전송한다.

### 3.2 클라이언트 → 서버 이벤트

#### `send_message`
```json
{
  "event": "send_message",
  "payload": {
    "content": "저도 그렇게 생각해요",
    "reply_to": "m-uuid-optional"
  }
}
```
- 서버는 수신 시각 기준으로 `created_at + 24h` 재계산하여 LIVE 여부 판정(PRD §3.2, §4.2). LIVE가 아니면 `error` 이벤트(`ROOM_NOT_LIVE`) 반환.
- 500자 초과 시 `error`(`VALIDATION_ERROR`).

#### `send_reaction`
```json
{
  "event": "send_reaction",
  "payload": { "message_id": "m-uuid", "emoji": "👍" }
}
```
- 동일 유저 재클릭 시 서버가 자동으로 취소 처리 후 `reaction_updated` 브로드캐스트(PRD CHAT-04).

#### `sync`
재연결 시 누락 메시지 복구 요청. (PRD §3.2)
```json
{
  "event": "sync",
  "payload": { "last_message_id": "m-uuid-last-known" }
}
```

### 3.3 서버 → 클라이언트 이벤트

#### `room_snapshot`
연결 직후 1회 전송.
```json
{
  "event": "room_snapshot",
  "payload": {
    "state": "LIVE",
    "conn_count": 432,
    "remaining_seconds": 29534,
    "recent_messages": [ /* 최근 50개, §2.3 메시지 포맷과 동일 */ ]
  }
}
```

#### `message_received`
전체 브로드캐스트. 목표 지연시간 200ms 이내(P95) (아키텍처 문서 §2, PRD CHAT-02)
```json
{
  "event": "message_received",
  "payload": {
    "message_id": "m-uuid",
    "sender_nickname": "졸린 눈의 해달",
    "content": "저도 그렇게 생각해요",
    "reply_to": "m-uuid-optional",
    "created_at": "2026-07-03T10:05:00Z"
  }
}
```

#### `reaction_updated`
```json
{
  "event": "reaction_updated",
  "payload": { "message_id": "m-uuid", "emoji": "👍", "count": 13, "action": "added" }
}
```

#### `room_state_changed`
LIVE→READ 전환 등 상태 변경 시 즉시 통지. (PRD LIFE-03)
```json
{
  "event": "room_state_changed",
  "payload": { "state": "READ", "changed_at": "2026-07-04T09:10:00Z" }
}
```
- 클라이언트는 이 이벤트 수신 시 입력창 비활성화 + "읽기 전용 토론방입니다" 안내 노출 (기획서 §3.2 하단 입력창)

#### `conn_count_updated`
```json
{ "event": "conn_count_updated", "payload": { "conn_count": 433 } }
```

#### `sync_result`
`sync` 요청에 대한 응답 — 누락 구간 메시지 일괄 전송.
```json
{
  "event": "sync_result",
  "payload": { "missed_messages": [ /* message_received와 동일 포맷 배열 */ ] }
}
```

#### `content_blinded`
신고 누적으로 메시지가 블라인드 처리된 경우. (PRD MOD-03, §6.2)
```json
{ "event": "content_blinded", "payload": { "message_id": "m-uuid" } }
```

#### `error`
```json
{
  "event": "error",
  "payload": { "code": "ROOM_NOT_LIVE", "message": "이 토론방은 읽기 전용 상태입니다." }
}
```

---

## 4. Rate Limit 정책 요약 (DB/아키텍처 문서 연동)

| 대상 | 제한 | 근거 |
| :--- | :--- | :--- |
| 토론방 생성 | 세션/계정당 1시간 3개 (60분 슬라이딩) | PRD ROOM-03 |
| 메시지 전송 | 초당 5건(스팸 방지, 세부값은 운영 중 조정) | PRD §4.4, §6 |
| 신고 접수 | 동일 대상 세션당 1일 10건 | PRD §6 확장 |
| 알림 발송 | 동일 유저·방·유형 1일 1회 | PRD §7.2 |

Rate Limit 초과 시 REST는 `429` + `RATE_LIMITED`, WebSocket은 `error` 이벤트로 통지.

---

## 5. 다음 문서와의 연결

- 본 문서의 §2.5 결제 API → **결제/정산 설계서**의 PG 연동 상세 흐름(웹훅, 실패 재시도 등) 근거
- §3 WebSocket 이벤트 전체 → 프론트엔드 구현 시 클라이언트 상태 머신 설계 근거
- §1.4, §4 에러/Rate Limit 정책 → **인프라/운영 문서**의 모니터링·알러팅 임계치 설정 근거
