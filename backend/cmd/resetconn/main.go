package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	// .env 로드 (프로젝트 루트 절대 경로)
	_ = godotenv.Load(`h:\lee\buzz48\.env`)

	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")
	password := os.Getenv("REDIS_PASSWORD")
	if host == "" {
		log.Fatal("REDIS_HOST 환경변수가 없습니다")
	}
	if port == "" {
		port = "6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
	})
	defer rdb.Close()

	ctx := context.Background()

	// post:*:conn_count 패턴으로 모든 동접자 카운터 키 스캔
	var cursor uint64
	var keys []string
	for {
		var batch []string
		var scanErr error
		batch, cursor, scanErr = rdb.Scan(ctx, cursor, "post:*:conn_count", 100).Result()
		if scanErr != nil {
			log.Fatalf("Redis SCAN 실패: %v", scanErr)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		fmt.Println("리셋할 conn_count 키가 없습니다.")
		return
	}

	fmt.Printf("총 %d개의 conn_count 키를 발견했습니다:\n", len(keys))
	for _, k := range keys {
		val, _ := rdb.Get(ctx, k).Result()
		fmt.Printf("  - %s = %s\n", k, val)
	}

	// 모두 0으로 리셋 (DEL 후 재생성 없이 단순 SET 0)
	for _, k := range keys {
		if err := rdb.Set(ctx, k, 0, 0).Err(); err != nil {
			log.Printf("  ⚠️ %s 리셋 실패: %v", k, err)
		} else {
			fmt.Printf("  ✅ %s → 0 리셋 완료\n", k)
		}
	}

	// disconnect_pending 키도 정리
	var pendingKeys []string
	cursor = 0
	for {
		var batch []string
		var scanErr2 error
		batch, cursor, scanErr2 = rdb.Scan(ctx, cursor, "session:*:disconnect_pending:*", 100).Result()
		if scanErr2 != nil {
			break
		}
		pendingKeys = append(pendingKeys, batch...)
		if cursor == 0 {
			break
		}
	}

	if len(pendingKeys) > 0 {
		fmt.Printf("\n잔여 disconnect_pending 키 %d개 정리 중...\n", len(pendingKeys))
		for _, pk := range pendingKeys {
			rdb.Del(ctx, pk)
		}
		fmt.Println("  ✅ 정리 완료")
	}

	fmt.Println("\n🎉 모든 conn_count 카운터가 0으로 리셋되었습니다. 서버를 재시작하세요.")
}
