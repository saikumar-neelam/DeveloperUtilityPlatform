package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// slidingWindowScript is a Lua script that implements a sliding window counter.
// KEYS[1] = rate-limit key
// ARGV[1] = current timestamp in microseconds
// ARGV[2] = window size in microseconds
// ARGV[3] = max requests per window
// Returns 1 if allowed, 0 if rate-limited.
var slidingWindowScript = redis.NewScript(`
local key      = KEYS[1]
local now      = tonumber(ARGV[1])
local window   = tonumber(ARGV[2])
local limit    = tonumber(ARGV[3])
local cutoff   = now - window

redis.call("ZREMRANGEBYSCORE", key, "-inf", cutoff)
local count = redis.call("ZCARD", key)
if count >= limit then
  return 0
end
redis.call("ZADD", key, now, now)
redis.call("PEXPIRE", key, math.ceil(window / 1000))
return 1
`)

// RedisLimiter is a distributed sliding-window rate limiter backed by Redis.
type RedisLimiter struct {
	rdb      *redis.Client
	window   time.Duration
	maxReqs  int
	prefix   string
}

// NewRedis connects to Redis and returns a RedisLimiter.
// window is the time window, maxReqs is the max allowed requests per window.
func NewRedis(redisURL string, window time.Duration, maxReqs int) (*RedisLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return &RedisLimiter{
		rdb:     rdb,
		window:  window,
		maxReqs: maxReqs,
		prefix:  "rl:",
	}, nil
}

// Allow returns true if the key is within the rate limit.
// Falls back to allowing the request if Redis is unavailable.
func (l *RedisLimiter) Allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	now := time.Now().UnixMicro()
	windowUs := l.window.Microseconds()

	result, err := slidingWindowScript.Run(ctx, l.rdb,
		[]string{l.prefix + key},
		now, windowUs, l.maxReqs,
	).Int()
	if err != nil {
		// Redis unavailable — fail open (allow request)
		return true
	}
	return result == 1
}
