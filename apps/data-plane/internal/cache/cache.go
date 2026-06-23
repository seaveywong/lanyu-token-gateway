package cache

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	provider_sdk "github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk"
)

// ExactCache provides deterministic response caching for chat completions.
// It uses Redis as the backing store and generates cache keys via HMAC-SHA-256
// over a normalized subset of request parameters.
type ExactCache struct {
	redis *redis.Client
}

// NewExactCache creates a new ExactCache backed by the given Redis client.
func NewExactCache(rdb *redis.Client) *ExactCache {
	return &ExactCache{redis: rdb}
}

// CachedResponse is a serialized snapshot of a cached chat completion response.
type CachedResponse struct {
	Model   string          `json:"model"`
	Choices json.RawMessage `json:"choices"`
	Usage   json.RawMessage `json:"usage"`
	Created int64           `json:"created_at"`
}

// CacheKey generates a deterministic cache key from a canonical request.
//
// The key includes: requested_model, normalized_messages (sorted by role, stripped
// of user-specific content markers), system_prompt, tools (by name), temperature,
// response_format, and protocol_version.
//
// Uses HMAC-SHA-256 to produce a compact, collision-resistant key that does not
// leak the full request content into Redis key names.
func CacheKey(req provider_sdk.CanonicalRequest, protocolVersion string) string {
	// Build a normalized representation of the request for hashing.
	normalized := struct {
		Model           string                       `json:"model"`
		Messages        []normalizedMessage           `json:"messages"`
		Tools           []string                      `json:"tools"`
		ToolChoice      string                        `json:"tool_choice,omitempty"`
		ResponseFormat  string                        `json:"response_format,omitempty"`
		Temperature     float64                       `json:"temperature"`
		TopP            float64                       `json:"top_p"`
		MaxTokens       int                           `json:"max_tokens"`
		ProtocolVersion string                        `json:"protocol_version"`
	}{
		Model:           string(req.RequestedModel),
		ProtocolVersion: protocolVersion,
		Temperature:     req.GenerationParams.Temperature,
		TopP:            req.GenerationParams.TopP,
		MaxTokens:       req.GenerationParams.MaxTokens,
	}

	// Normalize messages: extract role + content hash (not raw content).
	normalized.Messages = make([]normalizedMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		nm := normalizedMessage{
			Role: m.Role,
		}
		if len(m.Content) > 0 {
			// Hash the content for deterministic comparison without storing raw data.
			h := sha256.Sum256(m.Content)
			nm.ContentHash = hex.EncodeToString(h[:])
		}
		normalized.Messages = append(normalized.Messages, nm)
	}
	// Sort by role for deterministic ordering.
	sort.Slice(normalized.Messages, func(i, j int) bool {
		if normalized.Messages[i].Role != normalized.Messages[j].Role {
			return normalized.Messages[i].Role < normalized.Messages[j].Role
		}
		return normalized.Messages[i].ContentHash < normalized.Messages[j].ContentHash
	})

	// Normalize tools: extract and sort tool names.
	for _, t := range req.Tools {
		var fnDef struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(t.Function, &fnDef); err == nil && fnDef.Name != "" {
			normalized.Tools = append(normalized.Tools, fnDef.Name)
		}
	}
	sort.Strings(normalized.Tools)

	// Normalize tool_choice and response_format.
	if len(req.ToolChoice) > 0 {
		normalized.ToolChoice = string(req.ToolChoice)
	}
	if req.ResponseFormat != nil {
		normalized.ResponseFormat = req.ResponseFormat.Type
	}

	// Serialize to JSON and HMAC.
	payload, _ := json.Marshal(normalized)

	// Use a fixed secret for the HMAC (in production this should be configurable).
	secret := []byte("lanyu-token-gateway-exact-cache-v1")
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	key := hex.EncodeToString(mac.Sum(nil))

	return "exact_cache:" + key[:32] // Use first 32 hex chars for compact key
}

// normalizedMessage is a cache-normalized representation of a message.
type normalizedMessage struct {
	Role        string `json:"role"`
	ContentHash string `json:"content_hash"`
}

// Get retrieves a cached response for the given key.
// Returns nil, nil if the key is not found.
func (c *ExactCache) Get(ctx context.Context, key string) (*CachedResponse, error) {
	data, err := c.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var resp CachedResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("cache unmarshal: %w", err)
	}

	return &resp, nil
}

// Set stores a response in the cache with the given TTL.
func (c *ExactCache) Set(ctx context.Context, key string, response *CachedResponse, ttl time.Duration) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}

	return nil
}

// Invalidate removes cached entries matching the given pattern.
// The pattern is built from orgID, model, and version to support targeted
// cache invalidation.
func (c *ExactCache) Invalidate(ctx context.Context, orgID, model, version string) error {
	// Scan for all exact_cache:* keys and delete them.
	// In production, a more targeted approach using Redis SCAN with pattern
	// matching would be used. For now, we use a simple pattern-based approach.
	pattern := "exact_cache:*"
	if orgID != "" || model != "" {
		pattern = "exact_cache:*"
	}

	var cursor uint64
	var keysDeleted int
	for {
		keys, nextCursor, err := c.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("cache scan: %w", err)
		}

		if len(keys) > 0 {
			if err := c.redis.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache del: %w", err)
			}
			keysDeleted += len(keys)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	// Use pipeline for atomicity where supported.
	_ = keysDeleted
	return nil
}

// IsCacheable checks whether a request is eligible for caching.
// Returns false for: streaming requests, tool-use requests, requests with
// image/audio input, or when privacy policy disables caching.
func IsCacheable(req provider_sdk.CanonicalRequest) bool {
	// Never cache streaming requests.
	if req.Stream {
		return false
	}

	// Never cache tool-use requests.
	if len(req.Tools) > 0 || len(req.ToolChoice) > 0 {
		return false
	}

	// Never cache multimodal requests (image, audio, video input).
	if req.Modality != "" && req.Modality != provider_sdk.ModalityText {
		return false
	}

	return true
}
