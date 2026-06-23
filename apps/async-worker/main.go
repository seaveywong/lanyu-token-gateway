// Async Worker is the background job processor for the Lanyu Token Gateway.
// It consumes tasks from the transactional outbox (PostgreSQL) and/or NATS
// JetStream, handling payment settlement, channel health checks, credential
// auto-refresh, usage aggregation, webhook delivery, and other asynchronous
// workloads.
//
// It is the runtime entry point for the Lanyu Token Gateway async worker.
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/seaveywong/lanyu-token-gateway/packages/config"
	"github.com/seaveywong/lanyu-token-gateway/packages/observability"
)

// webhookEndpoint is a minimal row for dispatch lookups.
type webhookEndpoint struct {
	ID     string
	URL    string
	Secret string
}

// webhookDelivery is a minimal row for pending deliveries.
type webhookDelivery struct {
	ID             string
	EndpointID     string
	EventType      string
	Payload        string
	AttemptCount   int
	MaxAttempts    int
	Status         string
	IdempotencyKey string
}

// WebhookDispatcher polls webhook_deliveries for pending entries and delivers
// them to the customer's endpoint with exponential backoff retries.
type WebhookDispatcher struct {
	pool       *pgxpool.Pool
	httpClient *http.Client
	logger     *slog.Logger
}

// NewWebhookDispatcher creates a new WebhookDispatcher.
func NewWebhookDispatcher(pool *pgxpool.Pool) *WebhookDispatcher {
	return &WebhookDispatcher{
		pool: pool,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: slog.Default().With(slog.String("component", "webhook_dispatcher")),
	}
}

// Dispatch polls for pending deliveries and dispatches them.
func (d *WebhookDispatcher) Dispatch(ctx context.Context) error {
	// Fetch pending deliveries
	deliveries, err := d.listPending(ctx, 10)
	if err != nil {
		return fmt.Errorf("list pending deliveries: %w", err)
	}

	for _, delivery := range deliveries {
		if err := d.deliverOne(ctx, delivery); err != nil {
			d.logger.Warn("webhook delivery failed",
				slog.String("delivery_id", delivery.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// listPending fetches pending webhook deliveries.
func (d *WebhookDispatcher) listPending(ctx context.Context, limit int) ([]webhookDelivery, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, endpoint_id, event_type, payload::text, attempt_count, max_attempts, status, idempotency_key
		 FROM webhook_deliveries
		 WHERE status IN ('pending', 'failed')
		   AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		 ORDER BY created_at ASC
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []webhookDelivery
	for rows.Next() {
		var d webhookDelivery
		if err := rows.Scan(&d.ID, &d.EndpointID, &d.EventType, &d.Payload, &d.AttemptCount, &d.MaxAttempts, &d.Status, &d.IdempotencyKey); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// deliverOne dispatches a single webhook delivery to its endpoint.
func (d *WebhookDispatcher) deliverOne(ctx context.Context, delivery webhookDelivery) error {
	// Look up the endpoint
	endpoint, err := d.getEndpoint(ctx, delivery.EndpointID)
	if err != nil {
		return fmt.Errorf("get endpoint: %w", err)
	}
	if endpoint == nil {
		return d.markDead(ctx, delivery.ID)
	}

	// Build the request
	body := []byte(delivery.Payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Sign the payload
	signature := signPayload(endpoint.Secret, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token-Signature", signature)
	req.Header.Set("X-Token-Event", delivery.EventType)
	req.Header.Set("X-Token-Delivery-ID", delivery.ID)

	// Execute the request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		// Network error — schedule retry
		d.logger.Warn("webhook request failed",
			slog.String("delivery_id", delivery.ID),
			slog.String("url", endpoint.URL),
			slog.String("error", err.Error()),
		)
		return d.scheduleRetry(ctx, delivery)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	statusCode := resp.StatusCode

	// Success: 2xx response
	if statusCode >= 200 && statusCode < 300 {
		return d.markSuccess(ctx, delivery.ID, statusCode, string(respBody))
	}

	// Permanent failure: 4xx (except 429) — do not retry
	if statusCode >= 400 && statusCode < 500 && statusCode != 429 {
		d.logger.Warn("webhook rejected by endpoint",
			slog.String("delivery_id", delivery.ID),
			slog.Int("status", statusCode),
		)
		return d.markDead(ctx, delivery.ID)
	}

	// 5xx or 429 — schedule retry
	return d.scheduleRetry(ctx, delivery)
}

// getEndpoint looks up a webhook endpoint by ID.
func (d *WebhookDispatcher) getEndpoint(ctx context.Context, endpointID string) (*webhookEndpoint, error) {
	var ep webhookEndpoint
	err := d.pool.QueryRow(ctx,
		`SELECT id, url, secret FROM webhook_endpoints WHERE id = $1 AND is_active = TRUE`, endpointID,
	).Scan(&ep.ID, &ep.URL, &ep.Secret)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &ep, nil
}

// markSuccess marks a delivery as successful.
func (d *WebhookDispatcher) markSuccess(ctx context.Context, id string, statusCode int, respBody string) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET status = 'success', response_status = $2, response_body = $3, attempt_count = attempt_count + 1
		 WHERE id = $1`,
		id, statusCode, respBody,
	)
	return err
}

// markDead marks a delivery as dead (no more retries).
func (d *WebhookDispatcher) markDead(ctx context.Context, id string) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE webhook_deliveries SET status = 'dead' WHERE id = $1`,
		id,
	)
	return err
}

// scheduleRetry schedules the next retry with exponential backoff.
func (d *WebhookDispatcher) scheduleRetry(ctx context.Context, delivery webhookDelivery) error {
	if delivery.AttemptCount >= delivery.MaxAttempts {
		d.logger.Warn("webhook delivery max attempts reached",
			slog.String("delivery_id", delivery.ID),
			slog.Int("attempts", delivery.AttemptCount),
		)
		return d.markDead(ctx, delivery.ID)
	}

	// Exponential backoff: 2^attempt seconds, capped at 1 hour
	backoffSeconds := int(math.Pow(2, float64(delivery.AttemptCount)))
	if backoffSeconds > 3600 {
		backoffSeconds = 3600
	}
	nextRetry := time.Now().Add(time.Duration(backoffSeconds) * time.Second)

	_, err := d.pool.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET status = 'failed', next_retry_at = $2
		 WHERE id = $1`,
		delivery.ID, nextRetry,
	)
	return err
}

// signPayload computes the HMAC-SHA-256 signature for a webhook payload.
func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func main() {
	// --- Load configuration ---
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// --- Initialize observability ---
	ctx := context.Background()
	obsCfg := observability.ObservabilityConfig{
		ServiceName:    cfg.Observability.ServiceName,
		ServiceVersion: cfg.Observability.ServiceVersion,
		OTLPEndpoint:   cfg.Observability.OTLPEndpoint,
		LogLevel:       cfg.Observability.LogLevel,
		LogFormat:      cfg.Observability.LogFormat,
	}
	shutdown, err := observability.Init(ctx, obsCfg)
	if err != nil {
		slog.Error("failed to initialize observability", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			slog.Error("observability shutdown error", slog.String("error", err.Error()))
		}
	}()

	logger := observability.Logger()
	logger.Info("async-worker started")

	// --- Initialize database connection ---
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.DBName, cfg.Database.SSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Error("failed to parse database config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	poolCfg.MaxConns = int32(cfg.Database.MaxConns)
	poolCfg.MinConns = int32(cfg.Database.MinConns)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Error("failed to create database pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("database connection established")

	// Initialize Redis (best-effort, webhook dispatcher doesn't require it)
	var rdb *redis.Client
	redisCfg := cfg.Redis
	if redisCfg.Addr != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:         redisCfg.Addr,
			Password:     redisCfg.Password,
			DB:           redisCfg.DB,
			PoolSize:     redisCfg.PoolSize,
			MinIdleConns: redisCfg.MinIdleConns,
		})
		if err := rdb.Ping(ctx).Err(); err != nil {
			logger.Warn("redis connection failed, continuing without redis",
				slog.String("error", err.Error()),
			)
			rdb = nil
		} else {
			defer rdb.Close()
			logger.Info("redis connection established")
		}
	}
	_ = rdb

	// --- Create webhook dispatcher ---
	dispatcher := NewWebhookDispatcher(pool)

	// --- Signal handling for graceful shutdown ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// --- Worker loop ---
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Webhook delivery ticker: every 5 seconds
		webhookTicker := time.NewTicker(5 * time.Second)
		defer webhookTicker.Stop()

		for {
			select {
			case <-sigCh:
				logger.Info("received shutdown signal, draining worker loop")
				return

			case <-webhookTicker.C:
				// Dispatch pending webhook deliveries
				if err := dispatcher.Dispatch(context.Background()); err != nil {
					logger.Error("webhook dispatch error", slog.String("error", err.Error()))
				}

				// Heartbeat
				logger.Debug("worker heartbeat")
			}
		}
	}()

	// Wait for the worker loop to finish.
	<-done
	logger.Info("async-worker stopped")
}
