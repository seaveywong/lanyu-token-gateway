package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/seaveywong/lanyu-token-gateway/packages/contracts"
)

// RateLimiter performs rate limiting using a Redis-backed sliding window algorithm.
type RateLimiter struct {
	redis  *redis.Client
	logger *slog.Logger
}

// NewRateLimiter creates a new RateLimiter backed by the given Redis client.
func NewRateLimiter(redis *redis.Client) *RateLimiter {
	return &RateLimiter{
		redis:  redis,
		logger: slog.Default().With(slog.String("component", "rate_limiter")),
	}
}

const (
	// Default window size and limit
	defaultWindowSecs = 60
	defaultLimit      = 60
)

// Limit is a chi middleware that checks rate limits using a Redis sliding window.
// Returns 429 with a Retry-After header if the limit is exceeded.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := OrgIDFromContext(r.Context())
		if orgID == "" {
			// No authenticated context; pass through (auth middleware should catch this)
			next.ServeHTTP(w, r)
			return
		}

		// Build rate limit key: rate_limit:{org_id}:{path}
		limitKey := "rate_limit:" + orgID + ":" + r.URL.Path
		window := time.Duration(defaultWindowSecs) * time.Second
		limit := int64(defaultLimit)

		// Sliding window: use sorted set in Redis
		now := time.Now().UnixNano()
		windowStart := time.Now().Add(-window).UnixNano()

		pipe := rl.redis.Pipeline()

		// Remove entries outside the current window
		pipe.ZRemRangeByScore(r.Context(), limitKey, "0", strconv.FormatInt(windowStart, 10))

		// Count entries within the current window
		countCmd := pipe.ZCard(r.Context(), limitKey)

		// Add current request timestamp
		pipe.ZAdd(r.Context(), limitKey, redis.Z{
			Score:  float64(now),
			Member: strconv.FormatInt(now, 10),
		})

		// Set TTL on the key to auto-cleanup
		pipe.Expire(r.Context(), limitKey, window+time.Second)

		_, err := pipe.Exec(r.Context())
		if err != nil {
			rl.logger.Error("rate limit check failed", slog.String("error", err.Error()))
			// Fail open on Redis errors
			next.ServeHTTP(w, r)
			return
		}

		count := countCmd.Val()
		if count > limit {
			retryAfter := int(window.Seconds())
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(contracts.GatewayError{
				Code:              contracts.ErrRateLimitExceeded,
				Message:           "rate limit exceeded, please retry after " + strconv.Itoa(retryAfter) + " seconds",
				Type:              "rate_limit_error",
				RetryAfterSeconds: retryAfter,
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
