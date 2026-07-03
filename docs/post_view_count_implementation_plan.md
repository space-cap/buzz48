# 게시물 조회수(방문 화력) 기능 추가 계획서

본 문서는 Buzz48에 실시간 및 누적 게시물 조회수(방문 화력) 기능을 추가하기 위한 아키텍처 설계 및 구현 계획서입니다. 

---

## 1. 아키텍처 설계 원칙

1. **DB 트래픽 고갈 방지 (Write-Back 패턴)**:
   - 조회수를 올릴 때마다 DB에 직접 `UPDATE`를 수행하면 트래픽 폭증 시 커넥션 풀 부족으로 전체 서비스 장애가 유발됩니다.
   - 따라서 조회수 카운팅은 **Redis의 고속 카운터(`INCR`)**를 일차적으로 태우며, 주기적으로 배치 워커(Go Worker)가 Redis 값을 DB로 일괄 반영(Write-Back)하는 구조를 채택합니다.

2. **단시간 어뷰징(새로고침 연타) 차단**:
   - 동일 사용자(동일 세션)가 새로고침(F5)을 하거나 반복 진입할 경우의 지표 오염을 방지하기 위해 중복 제한을 구현합니다.
   - 세션 ID와 포스트 ID 조합의 중복 가드 키(`view_guard:{session_id}:{post_id}`)를 Redis에 10분 TTL(만료 시간)로 발급하여, 가드 키가 존재할 시 카운트 증가를 생략합니다.

---

## 2. 주요 작업 범위

### 2.1 데이터베이스 (PostgreSQL)
- **[NEW] [002_add_view_count.sql](file:///h:/lee/buzz48/backend/migrations/002_add_view_count.sql)**:
  - `posts` 테이블에 `view_count INT DEFAULT 0` 컬럼을 덧붙이는 마이그레이션 DDL을 추가합니다.

### 2.2 백엔드 (Go / Fiber / Redis)
- **[MODIFY] [store.go](file:///h:/lee/buzz48/backend/internal/redis/store.go)**:
  - `ConnIncr`처럼 조회수 카운팅을 원자적으로 수행하는 `PostViewIncr(sessionID, postID)` 헬퍼 함수를 추가합니다.
  - 이 함수는 중복 조회 가드 키(`view_guard:{session_id}:{post_id}`)가 없을 때에만 `view_count` 키를 1 증가시키고 가드 키를 생성합니다.
- **[MODIFY] [post.go](file:///h:/lee/buzz48/backend/internal/handler/post.go)**:
  - 상세페이지 조회 API(`GET /v1/posts/:id`) 핸들러 도달 시 `PostViewIncr`를 백그라운드나 인라인으로 태워 조회수를 집계합니다.
  - 목록 조회(`ListPosts`) 및 상세 조회 응답 JSON 객체에 `view_count` 필드를 추가하여, DB의 기본 컬럼 값과 Redis의 실시간 카운트 가중값을 합산하여 내려보냅니다.
- **[MODIFY] [worker/main.go](file:///h:/lee/buzz48/backend/cmd/worker/main.go)**:
  - 주기적(예: 2분 간격)으로 Redis의 실시간 `view_count` 데이터를 한 번에 모아서 PostgreSQL `posts` 테이블에 반영(Flush)하고 Redis 싱크 상태를 갱신하는 고루틴 워커 태스크를 추가합니다.

### 2.3 프론트엔드 (Next.js / TypeScript)
- **[MODIFY] [api.ts](file:///h:/lee/buzz48/frontend/src/lib/api.ts)**:
  - `PostItem` 및 응답 인터페이스에 `view_count: number` 필드를 새롭게 정의합니다.
- **[MODIFY] [page.tsx](file:///h:/lee/buzz48/frontend/src/app/page.tsx)**:
  - 메인 컴팩트 피드 목록 행의 참여 인원수 왼쪽에 누적 조회수 뱃지(예: `🔥 1,234`)를 자연스럽게 시각화합니다.
- **[MODIFY] [PostDetailClient.tsx](file:///h:/lee/buzz48/frontend/src/app/posts/\[id\]/PostDetailClient.tsx)**:
  - 상세 페이지 본문 영역 상단 메타 정보 영역에 누적 조회수를 표기합니다.

---

## 3. 검증 계획

### 3.1 자동화 테스트
- `PostViewIncr` 헬퍼 함수에 대해 세션이 다를 때와 같을 때(10분 내)의 카운트 증감 로직 검증용 Redis 연동 테스트 작성.
- 배치 워커의 Write-back 플러시 기능 로직 단위 테스트 수행.

### 3.2 수동 테스트 시나리오
1. **중복 차단**: A 유저 세션으로 특정 글 상세 진입 시 조회수 1 증가 확인 ➡️ 새로고침 연타 후 조회수가 유지되는지 검증.
2. **카운트 합산**: B 유저 세션(다른 브라우저)으로 진입 시 조회수가 2로 상승하는지 확인.
3. **데이터 보존**: 5분 뒤 DB `posts` 테이블의 `view_count` 레코드를 조회하여 캐시 카운트가 정상적으로 영구 동기화되었는지 확인.
