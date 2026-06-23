package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// RoutingService resolves external model names to native models and selects
// the best available account source to serve a request. It implements the
// routing algorithm defined in §5.3 of the full implementation spec.
type RoutingService struct {
	sources      *repository.AccountSourceRepo
	models       *repository.ModelRepo
	redis        *redis.Client
	roundRobinCtr uint64 // atomic counter for weighted round-robin
}

// ResolvedRoute contains the result of model resolution.
type ResolvedRoute struct {
	ExternalModel string                      `json:"external_model"`
	NativeModel   string                      `json:"native_model"`
	Candidates    []repository.AccountSource  `json:"candidates,omitempty"`
}

// NewRoutingService returns a RoutingService with the given dependencies.
func NewRoutingService(sources *repository.AccountSourceRepo, models *repository.ModelRepo, redis *redis.Client) *RoutingService {
	return &RoutingService{
		sources: sources,
		models:  models,
		redis:   redis,
	}
}

// ResolveModel maps an external model name to the native model and finds
// available sources. It returns a ResolvedRoute with candidates sorted by
// routing priority.
func (s *RoutingService) ResolveModel(ctx context.Context, externalModel string) (*ResolvedRoute, error) {
	if externalModel == "" {
		return nil, fmt.Errorf("resolve model: external_model is required")
	}

	// Find the model mapping for this external model.
	mappings, err := s.models.FindMapping(ctx, externalModel)
	if err != nil {
		return nil, fmt.Errorf("resolve model: find mappings: %w", err)
	}
	if len(mappings) == 0 {
		return nil, fmt.Errorf("resolve model: no mapping found for model %s", externalModel)
	}

	// Use the first mapping's native model as the canonical native model.
	nativeModel := mappings[0].NativeModel

	// Find routing candidates from the account_sources that support this model.
	candidates, err := s.sources.ListRoutingCandidates(ctx, externalModel)
	if err != nil {
		return nil, fmt.Errorf("resolve model: list candidates: %w", err)
	}

	// Sort candidates by priority (ascending) then weight (descending).
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority < candidates[j].Priority
		}
		return candidates[i].Weight > candidates[j].Weight
	})

	// Apply source_type ordering: official_api_key > official_oauth >
	// subscription_pool > upstream_api within the same priority band.
	sort.SliceStable(candidates, func(i, j int) bool {
		return sourceTypeRank(candidates[i].SourceType) < sourceTypeRank(candidates[j].SourceType)
	})
	// Re-sort with priority as primary key after the stable sort.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Priority < candidates[j].Priority
	})

	return &ResolvedRoute{
		ExternalModel: externalModel,
		NativeModel:   nativeModel,
		Candidates:    candidates,
	}, nil
}

// SelectSource picks the best source from candidates considering:
// 1. Priority (lower number = higher priority)
// 2. Weight (weighted round-robin within same priority)
// 3. Concurrency (skip if at max concurrency -- tracked in Redis)
// 4. Budget (skip if daily budget exceeded)
// 5. Health (skip if circuit open or dead)
//
// Returns nil if no candidate is available.
func (s *RoutingService) SelectSource(ctx context.Context, candidates []repository.AccountSource) (*repository.AccountSource, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("select source: no candidates available")
	}

	type eligible struct {
		source      repository.AccountSource
		totalWeight int
		weightSum   int
	}

	var eligibleList []eligible
	totalWeight := 0

	for _, c := range candidates {
		// Skip sources with zero or negative weight.
		if c.Weight <= 0 {
			continue
		}

		// Check health: skip dead or circuit_open sources.
		if c.HealthState == "dead" || c.HealthState == "circuit_open" {
			continue
		}

		// Check concurrency against max_concurrency via Redis.
		if s.redis != nil && c.MaxConcurrency > 0 {
			active, err := s.getConcurrency(ctx, c.ID)
			if err == nil && active >= c.MaxConcurrency {
				continue
			}
		}

		// Check daily budget. A value of 0 means no budget limit.
		if c.DailyBudgetMicroUSD > 0 {
			spent, err := s.getDailySpend(ctx, c.ID)
			if err == nil && spent >= c.DailyBudgetMicroUSD {
				continue
			}
		}

		totalWeight += c.Weight
		eligibleList = append(eligibleList, eligible{
			source:      c,
			totalWeight: c.Weight,
			weightSum:   0, // filled below
		})
	}

	if len(eligibleList) == 0 {
		return nil, fmt.Errorf("select source: no eligible source available")
	}

	// Stable-sort by priority (ascending), keep weight info.
	sort.SliceStable(eligibleList, func(i, j int) bool {
		return eligibleList[i].source.Priority < eligibleList[j].source.Priority
	})

	// Pick the highest priority band (lowest priority number).
	bestPriority := eligibleList[0].source.Priority
	samePriority := eligibleList[:0]
	for _, e := range eligibleList {
		if e.source.Priority == bestPriority {
			samePriority = append(samePriority, e)
		}
	}

	// Weighted round-robin within the same priority band.
	selected := s.weightedSelect(samePriority)

	// Increment concurrency counter in Redis.
	if s.redis != nil {
		_ = s.incrConcurrency(ctx, selected.source.ID)
	}

	// Update last used timestamp (best-effort).
	_ = s.sources.UpdateLastUsed(ctx, selected.source.ID)

	return &selected.source, nil
}

// weightedSelect picks one source from a same-priority list using weighted
// round-robin selection.
func (s *RoutingService) weightedSelect(eligible []eligible) eligible {
	if len(eligible) == 1 {
		return eligible[0]
	}

	// Compute running weight sums.
	totalW := 0
	for i := range eligible {
		totalW += eligible[i].totalWeight
		eligible[i].weightSum = totalW
	}

	// Atomic counter for round-robin across calls.
	counter := atomic.AddUint64(&s.roundRobinCtr, 1)
	point := int(counter % uint64(totalW))

	// Binary search for the source whose weight range covers the point.
	for _, e := range eligible {
		if point < e.weightSum {
			return e
		}
	}

	// Fallback: return the first eligible source.
	return eligible[0]
}

// ReleaseSource decrements the concurrency counter for a source after a
// request completes. Should be called in a defer block after SelectSource.
func (s *RoutingService) ReleaseSource(ctx context.Context, sourceID string) {
	if s.redis != nil {
		_ = s.decrConcurrency(ctx, sourceID)
	}
}

// --- Redis helpers ---

const (
	concurrencyKeyPrefix = "routing:concurrency:"
	dailySpendKeyPrefix  = "routing:daily_spend:"
)

func (s *RoutingService) getConcurrency(ctx context.Context, sourceID string) (int, error) {
	val, err := s.redis.Get(ctx, concurrencyKeyPrefix+sourceID).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get concurrency: %w", err)
	}
	return val, nil
}

func (s *RoutingService) incrConcurrency(ctx context.Context, sourceID string) error {
	key := concurrencyKeyPrefix + sourceID
	pipe := s.redis.Pipeline()
	incr := pipe.Incr(ctx, key)
	// Set a TTL to prevent stale keys from accumulating. The TTL is generous
	// enough to cover the max expected request duration.
	pipe.Expire(ctx, key, 5*time.Minute)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("incr concurrency: %w", err)
	}
	_ = incr.Val()
	return nil
}

func (s *RoutingService) decrConcurrency(ctx context.Context, sourceID string) error {
	key := concurrencyKeyPrefix + sourceID
	val, err := s.redis.Decr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("decr concurrency: %w", err)
	}
	// Clean up if count drops to 0 or below (safety).
	if val <= 0 {
		s.redis.Del(ctx, key)
	}
	return nil
}

func (s *RoutingService) getDailySpend(ctx context.Context, sourceID string) (int64, error) {
	key := dailySpendKeyPrefix + sourceID + ":" + time.Now().UTC().Format("2006-01-02")
	val, err := s.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get daily spend: %w", err)
	}
	return val, nil
}

// sourceTypeRank returns an ordering rank for source types. Lower values have
// higher preference in routing.
func sourceTypeRank(sourceType string) int {
	switch sourceType {
	case "official_api_key":
		return 0
	case "official_oauth":
		return 1
	case "subscription_pool":
		return 2
	case "upstream_api":
		return 3
	default:
		return math.MaxInt
	}
}
