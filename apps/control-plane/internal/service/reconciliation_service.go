package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// ---------------------------------------------------------------------------
// ReconciliationService
// ---------------------------------------------------------------------------

// ReconciliationService performs end-of-day reconciliation between provider
// billing, platform usage records, and ledger entries.
type ReconciliationService struct {
	payments *repository.PaymentRepo
	ledger   *repository.LedgerRepo
	audit    *repository.AuditRepo
}

// NewReconciliationService returns a new ReconciliationService.
func NewReconciliationService(
	payments *repository.PaymentRepo,
	ledger *repository.LedgerRepo,
	audit *repository.AuditRepo,
) *ReconciliationService {
	return &ReconciliationService{
		payments: payments,
		ledger:   ledger,
		audit:    audit,
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// DailyReport is a summary report for a given date.
type DailyReport struct {
	Date                  string                       `json:"date"`
	Run                   *repository.ReconciliationRun `json:"run"`
	TotalProviderCharges  int64                        `json:"total_provider_charges"`
	TotalPlatformRecords  int64                        `json:"total_platform_records"`
	TotalLedgerEntries    int64                        `json:"total_ledger_entries"`
	DiscrepancyCount      int                          `json:"discrepancy_count"`
	DiscrepancyMicroUSD   int64                        `json:"discrepancy_micro_usd"`
	Items                 []repository.ReconciliationItem `json:"items,omitempty"`
}

// ---------------------------------------------------------------------------
// RunDailyReconciliation
// ---------------------------------------------------------------------------

// Discrepancy threshold in micro USD (default: 100_000 = $0.10). Values exceeding
// this trigger an alert.
const defaultDiscrepancyThreshold int64 = 100_000

// RunDailyReconciliation performs end-of-day reconciliation:
//  1. Query upstream provider charges (stub — would call provider billing API)
//  2. Query platform usage records for the day
//  3. Query ledger entries for the day
//  4. Compare totals and identify discrepancies
//  5. Categorize each discrepancy
//  6. Create reconciliation_items for each discrepancy
//  7. Alert if discrepancy exceeds threshold
func (s *ReconciliationService) RunDailyReconciliation(ctx context.Context, date time.Time) (*repository.ReconciliationRun, error) {
	dateKey := date.Format("2006-01-02")

	slog.Info("starting daily reconciliation", slog.String("date", dateKey))

	// Create the reconciliation run
	run, err := s.payments.CreateReconciliationRun(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("create reconciliation run: %w", err)
	}

	// --- 1. Query upstream provider charges (STUB) ---
	// TODO: In production, this calls the provider's billing API to get all
	// charges for the given date.
	providerCharges := s.queryProviderChargesStub(ctx, date)

	// --- 2. Query platform usage records for the day ---
	// TODO: Query usage_events table for records on this date.
	// For now, stub returns 0 — will be wired when UsageRepo is available.
	platformRecords := s.queryPlatformUsageStub(ctx, date)

	// --- 3. Query ledger entries for the day ---
	// TODO: Query ledger_entries for the day. For now, stub returns 0.
	ledgerEntries := s.queryLedgerEntriesStub(ctx, date)

	// --- 4. Compare totals ---
	totalProvider := sumCharges(providerCharges)
	totalPlatform := sumCharges(platformRecords)
	totalLedger := sumCharges(ledgerEntries)

	discrepancyCount := 0
	var discrepancyTotal int64

	// --- 5. Identify and categorize discrepancies ---
	// Compare provider charges against platform records.
	// For any charge in provider that has no matching platform record, create a
	// discrepancy item.

	// Build lookup maps from each side and cross-reference.
	providerByRequest := make(map[string]providerChargeStub)
	for _, pc := range providerCharges {
		if pc.RequestID != nil {
			providerByRequest[*pc.RequestID] = pc
		}
	}
	platformByRequest := make(map[string]platformRecordStub)
	for _, pr := range platformRecords {
		if pr.RequestID != nil {
			platformByRequest[*pr.RequestID] = pr
		}
	}

	// Find charges that exist in provider but not in platform.
	for reqID, pc := range providerByRequest {
		if pr, ok := platformByRequest[reqID]; !ok {
			// Provider charged but no matching platform record → unknown_charge
			diff := pc.Amount
			if err := s.payments.CreateReconciliationItem(ctx, run.ID, "unknown_charge", &reqID,
				pc.Amount, 0, 0, diff); err != nil {
				return nil, fmt.Errorf("create unknown_charge item: %w", err)
			}
			discrepancyCount++
			discrepancyTotal += diff
		} else if pc.Amount != pr.Amount {
			// Both have the record but amounts differ → meter_diff
			diff := pc.Amount - pr.Amount
			if diff < 0 {
				diff = -diff
			}
			if err := s.payments.CreateReconciliationItem(ctx, run.ID, "meter_diff", &reqID,
				pc.Amount, pr.Amount, 0, diff); err != nil {
				return nil, fmt.Errorf("create meter_diff item: %w", err)
			}
			discrepancyCount++
			discrepancyTotal += diff
		}
	}

	// Find platform records with no matching provider charge.
	for reqID, pr := range platformByRequest {
		if _, ok := providerByRequest[reqID]; !ok {
			// Platform recorded but no provider charge → missing_charge (or delayed_bill)
			typ := "missing_charge"
			if pr.IsRecent {
				typ = "delayed_bill"
			}
			diff := pr.Amount
			if err := s.payments.CreateReconciliationItem(ctx, run.ID, typ, &reqID,
				0, pr.Amount, 0, diff); err != nil {
				return nil, fmt.Errorf("create missing_charge item: %w", err)
			}
			discrepancyCount++
			discrepancyTotal += diff
		}
	}

	// --- 6. Check for duplicates (by request_id in platform records) ---
	seen := make(map[string]int)
	for _, pr := range platformRecords {
		if pr.RequestID != nil {
			seen[*pr.RequestID]++
		}
	}
	for reqID, count := range seen {
		if count > 1 {
			firstPR := platformByRequest[reqID]
			if err := s.payments.CreateReconciliationItem(ctx, run.ID, "duplicate", &reqID,
				0, firstPR.Amount*int64(count-1), 0, firstPR.Amount*int64(count-1)); err != nil {
				return nil, fmt.Errorf("create duplicate item: %w", err)
			}
			discrepancyCount++
			discrepancyTotal += firstPR.Amount * int64(count-1)
		}
	}

	// --- 7. Complete the run ---
	if err := s.payments.CompleteReconciliationRun(ctx, run.ID,
		totalProvider, totalPlatform, totalLedger,
		discrepancyTotal, discrepancyCount,
	); err != nil {
		return nil, fmt.Errorf("complete reconciliation run: %w", err)
	}

	// Re-fetch the completed run
	completedRun, _ := s.payments.FindReconciliationRunByDate(ctx, date)

	// --- 8. Alert if discrepancy exceeds threshold ---
	if discrepancyTotal > defaultDiscrepancyThreshold {
		slog.Warn("reconciliation discrepancy exceeds threshold",
			slog.String("date", dateKey),
			slog.Int64("discrepancy_micro_usd", discrepancyTotal),
			slog.Int64("threshold_micro_usd", defaultDiscrepancyThreshold),
			slog.Int("discrepancy_count", discrepancyCount),
		)
	}

	slog.Info("daily reconciliation completed",
		slog.String("date", dateKey),
		slog.Int("discrepancy_count", discrepancyCount),
		slog.Int64("discrepancy_micro_usd", discrepancyTotal),
		slog.Int64("total_provider", totalProvider),
		slog.Int64("total_platform", totalPlatform),
	)

	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			Action:       "reconciliation.run_completed",
			ResourceType: "reconciliation_run",
			ResourceID:   run.ID,
			MetadataJSON: fmt.Sprintf(`{"date":"%s","discrepancy_count":%d,"discrepancy_micro_usd":%d}`, dateKey, discrepancyCount, discrepancyTotal),
		})
	}

	return completedRun, nil
}

// ---------------------------------------------------------------------------
// GetDiscrepancies
// ---------------------------------------------------------------------------

// GetDiscrepancies returns all (including resolved) reconciliation items for
// a given run.
func (s *ReconciliationService) GetDiscrepancies(ctx context.Context, runID string) ([]repository.ReconciliationItem, error) {
	return s.payments.ListReconciliationItems(ctx, runID)
}

// ---------------------------------------------------------------------------
// ResolveDiscrepancy
// ---------------------------------------------------------------------------

// ResolveDiscrepancy marks a discrepancy as addressed.
// Corrected items may include a follow-up ledger entry to fix the balance (done
// elsewhere by the operator).
func (s *ReconciliationService) ResolveDiscrepancy(ctx context.Context, itemID, resolution, notes string, correctionAmount int64) error {
	if resolution == "" {
		return fmt.Errorf("resolution is required")
	}

	if err := s.payments.UpdateReconciliationItem(ctx, itemID, resolution, notes); err != nil {
		return fmt.Errorf("resolve discrepancy: %w", err)
	}

	slog.Info("discrepancy resolved",
		slog.String("item_id", itemID),
		slog.String("resolution", resolution),
		slog.Int64("correction_amount", correctionAmount),
	)

	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			Action:       "reconciliation.discrepancy_resolved",
			ResourceType: "reconciliation_item",
			ResourceID:   itemID,
			MetadataJSON: fmt.Sprintf(`{"resolution":"%s","correction_amount":%d}`, resolution, correctionAmount),
		})
	}

	return nil
}

// ---------------------------------------------------------------------------
// GetDailyReport
// ---------------------------------------------------------------------------

// GetDailyReport returns a summary report for a date.
func (s *ReconciliationService) GetDailyReport(ctx context.Context, date time.Time) (*DailyReport, error) {
	dateKey := date.Format("2006-01-02")

	run, err := s.payments.FindReconciliationRunByDate(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("find recon run: %w", err)
	}
	if run == nil {
		return &DailyReport{
			Date: dateKey,
		}, nil
	}

	items, err := s.payments.ListReconciliationItems(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("list recon items: %w", err)
	}

	return &DailyReport{
		Date:                 dateKey,
		Run:                  run,
		TotalProviderCharges: run.TotalProviderCharges,
		TotalPlatformRecords: run.TotalPlatformRecords,
		TotalLedgerEntries:   run.TotalLedgerEntries,
		DiscrepancyCount:     run.DiscrepancyCount,
		DiscrepancyMicroUSD:  run.DiscrepancyMicroUSD,
		Items:                items,
	}, nil
}

// ---------------------------------------------------------------------------
// Stub types and helpers for reconciliation
// ---------------------------------------------------------------------------

// providerChargeStub represents a charge from an upstream provider.
// TODO: Replace with real provider billing API integration.
type providerChargeStub struct {
	RequestID *string
	Amount    int64 // micro USD
	Timestamp time.Time
}

// platformRecordStub represents a usage record from the platform.
// TODO: Replace with actual usage_events query.
type platformRecordStub struct {
	RequestID *string
	Amount    int64  // micro USD
	IsRecent  bool   // true if the record was created recently (might be delayed_bill)
}

// queryProviderChargesStub returns stub provider charges for a date.
// TODO: Call provider billing API (Alipay bill, WeChat Pay statement, etc.).
func (s *ReconciliationService) queryProviderChargesStub(ctx context.Context, date time.Time) []providerChargeStub {
	slog.Info("queryProviderCharges: STUB — no real provider API integration yet",
		slog.String("date", date.Format("2006-01-02")),
	)
	return nil
}

// queryPlatformUsageStub returns stub platform usage records for a date.
// TODO: Query usage_events table for the given date.
func (s *ReconciliationService) queryPlatformUsageStub(ctx context.Context, date time.Time) []platformRecordStub {
	slog.Info("queryPlatformUsage: STUB — no usage_events query yet",
		slog.String("date", date.Format("2006-01-02")),
	)
	return nil
}

// queryLedgerEntriesStub returns stub ledger entries for a date.
// TODO: Query ledger_entries for the given date.
func (s *ReconciliationService) queryLedgerEntriesStub(ctx context.Context, date time.Time) []providerChargeStub {
	slog.Info("queryLedgerEntries: STUB — no ledger query yet",
		slog.String("date", date.Format("2006-01-02")),
	)
	return nil
}

// sumCharges sums the amounts from a slice of providerChargeStub.
func sumCharges(charges []providerChargeStub) int64 {
	var total int64
	for _, c := range charges {
		total += c.Amount
	}
	return total
}
