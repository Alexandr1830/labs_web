package auth

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Один общий клиент на процесс. Адрес из REDIS_ADDR, пароль из REDIS_PASSWORD.
var (
	once   sync.Once
	client *redis.Client
)

func redisClient() *redis.Client {
	once.Do(func() {
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		client = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0,
		})
	})
	return client
}

// BlacklistJTI — кладёт jti в blacklist с TTL = до истечения exp токена.
// После TTL ключ исчезнет сам и не будет занимать память.
func BlacklistJTI(ctx context.Context, jti string, ttlSeconds int64) error {
	if ttlSeconds <= 0 {
		ttlSeconds = 1 // минимальный TTL, чтобы Redis принял
	}
	return redisClient().Set(ctx, "bl:"+jti, "1", time.Duration(ttlSeconds)*time.Second).Err()
}

// IsBlacklisted — true если jti в чёрном списке.
func IsBlacklisted(ctx context.Context, jti string) bool {
	n, err := redisClient().Exists(ctx, "bl:"+jti).Result()
	if err != nil {
		return false // если Redis недоступен, не блокируем доступ
	}
	return n > 0
}
