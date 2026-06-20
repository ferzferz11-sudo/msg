package main

// redis_rate_limiter.go — Redis-backed sliding window rate limiter
// Falls back to in-memory if Redis is unavailable.

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	redisClient    *redis.Client
	redisOnce      sync.Once
	redisAvailable bool
)

func getRedisClient() *redis.Client {
	redisOnce.Do(func() {
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		redisClient = redis.NewClient(&redis.Options{
			Addr:         addr,
			DialTimeout:  2 * time.Second,
			ReadTimeout:  1 * time.Second,
			WriteTimeout: 1 * time.Second,
			PoolSize:     10,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Warnf("Redis not available, using in-memory rate limiter: %v", err)
			redisAvailable = false
		} else {
			logger.Info("Redis connected for rate limiting")
			redisAvailable = true
		}
	})
	return redisClient
}

// RedisRateLimiter uses Redis sorted sets for sliding window rate limiting.
// Falls back to in-memory rateLimiter if Redis is unavailable.
type RedisRateLimiter struct {
	memory *rateLimiter
	limit  int
	window time.Duration
	prefix string // Redis key prefix, e.g. "rl:owl:"
}

func NewRedisRateLimiter(limit int, window time.Duration, prefix string) *RedisRateLimiter {
	return &RedisRateLimiter{
		memory: newRateLimiter(limit, window),
		limit:  limit,
		window: window,
		prefix: prefix,
	}
}

func (rl *RedisRateLimiter) Allow(userID string) bool {
	if !redisAvailable {
		return rl.memory.allow(userID)
	}

	ctx := context.Background()
	client := getRedisClient()
	key := rl.prefix + userID
	now := time.Now()
	cutoff := now.Add(-rl.window)

	pipe := client.Pipeline()
	// Remove expired entries
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", cutoff.UnixNano()))
	// Count current entries
	countCmd := pipe.ZCard(ctx, key)
	// Add new entry
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixNano()), Member: now.UnixNano()})
	// Set TTL
	pipe.Expire(ctx, key, rl.window+time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Warnf("Redis rate limit error, falling back to memory: %v", err)
		return rl.memory.allow(userID)
	}

	return countCmd.Val() < int64(rl.limit)
}

func (rl *RedisRateLimiter) Cancel(userID string) {
	if !redisAvailable {
		rl.memory.cancel(userID)
		return
	}

	ctx := context.Background()
	client := getRedisClient()
	key := rl.prefix + userID

	// Remove last entry (approximate)
	result, err := client.ZRevRangeWithScores(ctx, key, 0, 0).Result()
	if err == nil && len(result) > 0 {
		client.ZRem(ctx, key, result[0].Member)
	}
}

func (rl *RedisRateLimiter) Remaining(userID string) int {
	if !redisAvailable {
		return rl.memory.remaining(userID)
	}

	ctx := context.Background()
	client := getRedisClient()
	key := rl.prefix + userID
	cutoff := time.Now().Add(-rl.window)

	count, err := client.ZCount(ctx, key,
		fmt.Sprintf("%d", cutoff.UnixNano()),
		fmt.Sprintf("%d", time.Now().UnixNano()),
	).Result()
	if err != nil {
		return rl.memory.remaining(userID)
	}

	remaining := rl.limit - int(count)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (rl *RedisRateLimiter) Cleanup() {
	if !redisAvailable {
		rl.memory.cleanup()
		return
	}
	// Redis handles cleanup via TTL automatically
}
