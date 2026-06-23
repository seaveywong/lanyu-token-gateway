package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// PricingService provides business logic for pricing versions and cost calculation.
type PricingService struct {
	pricing *repository.PricingRepo
	audit   *repository.AuditRepo
}

// NewPricingService returns a PricingService with the given repositories.
func NewPricingService(pricing *repository.PricingRepo, audit *repository.AuditRepo) *PricingService {
	return &PricingService{pricing: pricing, audit: audit}
}

// CreateVersionParams holds the data needed to create a new pricing version
// with its associated rules.
type CreateVersionParams struct {
	Name        string
	Description *string
	CreatedBy   *string
	Rules       []CreateRuleItem
}

// CreateRuleItem is a single pricing rule within a version.
type CreateRuleItem struct {
	ModelName          string
	InputPriceMicroUSD   int64
	OutputPriceMicroUSD  int64
	CachedPriceMicroUSD  int64
	ImagePriceMicroUSD   int64
	AudioPriceMicroUSD   int64
}

// CreateVersion creates a new pricing version with rules and writes an audit
// log. The new version is NOT automatically activated — call ActivateVersion
// separately.
func (s *PricingService) CreateVersion(ctx context.Context, userID string, params CreateVersionParams) (*repository.PricingVersion, error) {
	if params.Name == "" {
		return nil, fmt.Errorf("create pricing version: name is required")
	}

	var createdBy *string
	if userID != "" {
		createdBy = &userID
	}
	if params.CreatedBy != nil {
		createdBy = params.CreatedBy
	}

	version, err := s.pricing.CreateVersion(ctx, repository.CreatePricingVersionParams{
		Name:        params.Name,
		Description: params.Description,
		CreatedBy:   createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create pricing version: %w", err)
	}

	// Insert all rules for this version
	for _, rule := range params.Rules {
		if _, err := s.pricing.CreateRule(ctx, version.ID, rule.ModelName,
			rule.InputPriceMicroUSD, rule.OutputPriceMicroUSD,
			rule.CachedPriceMicroUSD, rule.ImagePriceMicroUSD, rule.AudioPriceMicroUSD,
		); err != nil {
			return nil, fmt.Errorf("create pricing version: add rule for model %q: %w", rule.ModelName, err)
		}
	}

	// Audit log
	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			ActorID:      userID,
			Action:       "pricing.version_created",
			ResourceType: "pricing_version",
			ResourceID:   version.ID,
			MetadataJSON: fmt.Sprintf(`{"name":"%s","rule_count":%d}`, params.Name, len(params.Rules)),
		})
	}

	return version, nil
}

// CalculateCost estimates the cost for a request using the active pricing version.
//
// Parameters:
//   - model: the resolved (native) model name to look up pricing
//   - inputTokens, outputTokens, cachedTokens: token counts
//   - modalityUnits: per-modality usage (e.g. "image": 2, "audio_seconds": 30)
//   - costMultiplier: customer markup, e.g. 1.5 = 50% margin
//
// Returns:
//   - providerCost: what the upstream provider charges (in micro_usd)
//   - customerCharge: what the customer is billed (provider_cost * costMultiplier)
//   - pricingVersionID: the active version used for the calculation
func (s *PricingService) CalculateCost(ctx context.Context, model string, inputTokens, outputTokens, cachedTokens int, modalityUnits map[string]int, costMultiplier float64) (providerCost, customerCharge int64, pricingVersionID string, err error) {
	// Default multiplier
	if costMultiplier <= 0 {
		costMultiplier = 1.0
	}

	// Get the active pricing version
	version, err := s.pricing.GetCurrentVersion(ctx)
	if err != nil {
		return 0, 0, "", fmt.Errorf("calculate cost: get current version: %w", err)
	}
	if version == nil {
		return 0, 0, "", fmt.Errorf("calculate cost: no active pricing version")
	}

	// Find pricing rules for the model
	rules, err := s.pricing.FindRulesByModel(ctx, model)
	if err != nil {
		return 0, 0, "", fmt.Errorf("calculate cost: find rules for model %q: %w", model, err)
	}
	if len(rules) == 0 {
		return 0, 0, "", fmt.Errorf("calculate cost: no pricing rule for model %q in version %s", model, version.ID)
	}

	// Use the first (most recently effective) rule
	rule := rules[0]

	// Calculate provider cost
	var cost float64
	cost += float64(inputTokens) * float64(rule.InputPriceMicroUSD)
	cost += float64(outputTokens) * float64(rule.OutputPriceMicroUSD)
	cost += float64(cachedTokens) * float64(rule.CachedPriceMicroUSD)

	// Per-unit modality costs: images, audio, etc.
	if images, ok := modalityUnits["image"]; ok {
		cost += float64(images) * float64(rule.ImagePriceMicroUSD)
	}
	if audioSec, ok := modalityUnits["audio_seconds"]; ok {
		cost += float64(audioSec) * float64(rule.AudioPriceMicroUSD)
	}

	// Token costs are per-token — they can be fractional (e.g. 0.15 micro_usd per token).
	// But the spec says inputPrice/outputPrice are int64 per token, which means they
	// represent micro_usd per token already. So 1 token * 1500 micro_usd = 1500
	// micro_usd. That's per 1 token? No, typically pricing is per 1000 tokens or
	// per token with fractional amounts. Let's interpret:
	//
	// The prices in the rules are in micro_usd per unit (per token, per image, per
	// audio second). So:
	//   cost = sum(units * price_micro_usd_per_unit)
	//
	// This gives us cost in micro_usd.

	providerCost = int64(math.Round(cost))

	// Apply customer markup
	customerCharge = int64(math.Round(cost * costMultiplier))

	return providerCost, customerCharge, version.ID, nil
}

// GetCurrentVersion returns the currently active pricing version.
func (s *PricingService) GetCurrentVersion(ctx context.Context) (*repository.PricingVersion, error) {
	version, err := s.pricing.GetCurrentVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current pricing version: %w", err)
	}
	return version, nil
}

// ActivateVersion activates a pricing version (deactivates all others).
func (s *PricingService) ActivateVersion(ctx context.Context, userID, versionID string) error {
	if err := s.pricing.ActivateVersion(ctx, versionID); err != nil {
		return fmt.Errorf("activate pricing version: %w", err)
	}

	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			ActorID:      userID,
			Action:       "pricing.version_activated",
			ResourceType: "pricing_version",
			ResourceID:   versionID,
		})
	}

	return nil
}

// Ensure time is used (for future extensions like effective date checks).
var _ = time.Now
