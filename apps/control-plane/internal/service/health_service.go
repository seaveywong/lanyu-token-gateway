package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// HealthService performs periodic health checks on account sources and manages
// circuit breaker state in Redis.
type HealthService struct {
	sources *repository.AccountSourceRepo
	redis   *redis.Client
}

// CircuitState tracks the circuit breaker state for an account source in Redis.
type CircuitState struct {
	FailureCount  int       `json:"failure_count"`
	LastFailure   time.Time `json:"last_failure"`
	State         string    `json:"state"` // closed, open, half_open
	CooldownUntil time.Time `json:"cooldown_until"`
}

const (
	// Circuit breaker constants.
	defaultFailureThreshold = 5
	defaultCooldownSeconds  = 120
	circuitStateKeyPrefix   = "circuit:state:"

	// Health check defaults.
	defaultPeriodSeconds = 60
)

// NewHealthService returns a HealthService with the given dependencies.
func NewHealthService(sources *repository.AccountSourceRepo, redis *redis.Client) *HealthService {
	return &HealthService{sources: sources, redis: redis}
}

// CheckSource validates a source's credential with its provider.
// This is a stub — full provider adapter integration will be added when the
// provider SDK is wired in.
//
// For now, it performs a basic liveness check and updates the health state.
func (s *HealthService) CheckSource(ctx context.Context, sourceID string) error {
	source, err := s.sources.FindByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("health check source: %w", err)
	}
	if source == nil {
		return fmt.Errorf("health check source: source %s not found", sourceID)
	}

	// State machine: inspect current circuit state before probing.
	circuit, err := s.getCircuitState(ctx, sourceID)
	if err == nil && circuit != nil {
		switch circuit.State {
		case "open":
			// Check if the cooldown period has elapsed.
			if time.Now().UTC().Before(circuit.CooldownUntil) {
				return nil // skip, still cooling down
			}
			// Cooldown elapsed — transition to half_open and allow a probe.
			slog.Info("circuit transitioning to half_open",
				slog.String("source_id", sourceID),
			)
			circuit.State = "half_open"
			_ = s.setCircuitState(ctx, sourceID, circuit)
		case "half_open":
			// Only allow one probe request in half_open state.
		}
	}

	// TODO: integrate with provider adapter to actually validate the credential.
	// For now, mark as healthy after basic existence check.
	healthState := "healthy"
	_ = s.sources.UpdateHealth(ctx, sourceID, healthState)

	// On successful check, reset the circuit breaker.
	if circuit != nil && circuit.State != "closed" {
		slog.Info("circuit reset to closed after successful health check",
			slog.String("source_id", sourceID),
		)
		_ = s.setCircuitState(ctx, sourceID, &CircuitState{
			State:         "closed",
			LastFailure:   time.Time{},
			CooldownUntil: time.Time{},
		})
	}

	return nil
}

// StartHealthCheckLoop runs periodic health checks on all active sources.
// This should be called in a goroutine.
func (s *HealthService) StartHealthCheckLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Duration(defaultPeriodSeconds) * time.Second
	}

	slog.Info("health check loop started",
		slog.Duration("interval", interval),
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run an immediate check on startup.
	s.runAllChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("health check loop stopped")
			return
		case <-ticker.C:
			s.runAllChecks(ctx)
		}
	}
}

// runAllChecks fetches all active sources and checks each one.
func (s *HealthService) runAllChecks(ctx context.Context) {
	// Get first page of active sources. In production we would paginate
	// through all pages.
	sources, _, err := s.sources.List(ctx, "", 1, 200)
	if err != nil {
		slog.Error("health check: failed to list sources",
			slog.String("error", err.Error()),
		)
		return
	}

	for _, src := range sources {
		if src.Status != "active" {
			continue
		}
		srcID := src.ID
		if err := s.CheckSource(ctx, srcID); err != nil {
			slog.Warn("health check: source check failed",
				slog.String("source_id", srcID),
				slog.String("error", err.Error()),
			)
			// Record the failure for circuit breaker.
			_ = s.recordFailure(ctx, srcID)
		}
	}
}

// RecordSuccess records a successful request against a source for passive
// health tracking. This resets the failure counter.
func (s *HealthService) RecordSuccess(ctx context.Context, sourceID string) {
	if s.redis == nil {
		return
	}
	circuit, _ := s.getCircuitState(ctx, sourceID)
	if circuit != nil && circuit.State != "closed" {
		// Reset to closed on sustained success.
		_ = s.setCircuitState(ctx, sourceID, &CircuitState{
			State:         "closed",
			LastFailure:   time.Time{},
			CooldownUntil: time.Time{},
		})
	}
}

// RecordFailure records a failed request against a source. If the failure
// threshold is exceeded, the circuit opens.
func (s *HealthService) RecordFailure(ctx context.Context, sourceID string) {
	if s.redis == nil {
		return
	}
	_ = s.recordFailure(ctx, sourceID)
}

// recordFailure increments the failure counter and opens the circuit if the
// threshold is exceeded.
func (s *HealthService) recordFailure(ctx context.Context, sourceID string) error {
	circuit, err := s.getCircuitState(ctx, sourceID)
	if err != nil {
		return err
	}
	if circuit == nil {
		circuit = &CircuitState{State: "closed"}
	}

	circuit.FailureCount++
	circuit.LastFailure = time.Now().UTC()

	if circuit.FailureCount >= defaultFailureThreshold {
		circuit.State = "open"
		circuit.CooldownUntil = time.Now().UTC().Add(time.Duration(defaultCooldownSeconds) * time.Second)
		slog.Warn("circuit opened due to consecutive failures",
			slog.String("source_id", sourceID),
			slog.Int("failure_count", circuit.FailureCount),
			slog.Time("cooldown_until", circuit.CooldownUntil),
		)
		// Update the source health state in PostgreSQL.
		_ = s.sources.UpdateHealth(ctx, sourceID, "circuit_open")
	}

	return s.setCircuitState(ctx, sourceID, circuit)
}

// getCircuitState retrieves the circuit breaker state from Redis.
func (s *HealthService) getCircuitState(ctx context.Context, sourceID string) (*CircuitState, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}
	key := circuitStateKeyPrefix + sourceID
	data, err := s.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get circuit state: %w", err)
	}

	var state CircuitState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal circuit state: %w", err)
	}
	return &state, nil
}

// setCircuitState persists the circuit breaker state to Redis.
func (s *HealthService) setCircuitState(ctx context.Context, sourceID string, state *CircuitState) error {
	if s.redis == nil {
		return fmt.Errorf("redis not available")
	}
	key := circuitStateKeyPrefix + sourceID
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal circuit state: %w", err)
	}
	// Keep circuit state for 24 hours.
	if err := s.redis.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("set circuit state: %w", err)
	}
	return nil
}

// GetCircuitState is a convenience method to expose the circuit state for
// admin dashboards and monitoring.
func (s *HealthService) GetCircuitState(ctx context.Context, sourceID string) (*CircuitState, error) {
	return s.getCircuitState(ctx, sourceID)
}
