package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// WalletService provides business logic for wallet balance management, fund
// reservation (freeze), settlement, and release. It coordinates with the
// double-entry ledger to ensure every money movement is auditable.
type WalletService struct {
	wallets       *repository.WalletRepo
	ledger        *repository.LedgerRepo
	audit         *repository.AuditRepo
	platformOrgID string // org whose org-level wallet receives platform revenue
}

// NewWalletService returns a WalletService with the given repositories.
// platformOrgID identifies the platform organization whose wallet collects
// revenue (the credit side of settlement double-entries).
func NewWalletService(wallets *repository.WalletRepo, ledger *repository.LedgerRepo, audit *repository.AuditRepo, platformOrgID string) *WalletService {
	return &WalletService{
		wallets:       wallets,
		ledger:        ledger,
		audit:         audit,
		platformOrgID: platformOrgID,
	}
}

// WalletBalance represents the current balance state of a wallet.
type WalletBalance struct {
	BalanceMicroUSD   int64 `json:"balance_micro_usd"`
	FrozenMicroUSD    int64 `json:"frozen_micro_usd"`
	AvailableMicroUSD int64 `json:"available_micro_usd"` // balance - frozen
}

// GetBalance returns the current balance for an org or org+project wallet.
func (s *WalletService) GetBalance(ctx context.Context, orgID string, projectID *string) (*WalletBalance, error) {
	wallet, err := s.wallets.GetOrCreate(ctx, orgID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	return &WalletBalance{
		BalanceMicroUSD:   wallet.BalanceMicroUSD,
		FrozenMicroUSD:    wallet.FrozenMicroUSD,
		AvailableMicroUSD: wallet.BalanceMicroUSD - wallet.FrozenMicroUSD,
	}, nil
}

// ReserveFunds freezes funds before an API call.
//
// Steps:
//  1. Get or create the customer wallet
//  2. Atomically freeze the estimated cost (balance -> frozen)
//  3. Create a reservation ledger entry for the customer wallet
//
// The reservation is a single-entry memo — it does NOT require a double-entry
// counterpart because no money has actually moved yet. The actual double-entry
// happens at settlement time.
func (s *WalletService) ReserveFunds(ctx context.Context, orgID, projectID, requestID string, estimatedCostMicroUSD int64) (*repository.Wallet, error) {
	if estimatedCostMicroUSD <= 0 {
		return nil, fmt.Errorf("reserve funds: estimated cost must be positive, got %d", estimatedCostMicroUSD)
	}

	wallet, err := s.wallets.GetOrCreate(ctx, orgID, &projectID)
	if err != nil {
		return nil, fmt.Errorf("reserve funds: get wallet: %w", err)
	}

	if err := s.wallets.FreezeBalance(ctx, wallet.ID, estimatedCostMicroUSD); err != nil {
		return nil, fmt.Errorf("reserve funds: freeze: %w", err)
	}

	// Create a reservation ledger entry (memo, not double-entry)
	// The idempotency key ensures we don't double-freeze.
	reservationKey := fmt.Sprintf("reserve_%s_%s", requestID, wallet.ID)
	if len(reservationKey) > 128 {
		reservationKey = reservationKey[:128]
	}
	_, err = s.ledger.CreateEntry(ctx, nil, repository.LedgerEntryParams{
		OrganizationID: orgID,
		WalletID:       wallet.ID,
		AmountMicroUSD: -estimatedCostMicroUSD, // credit = hold on available balance
		EntryType:      "reservation",
		Description:    fmt.Sprintf("Reserve %d micro_usd for request %s", estimatedCostMicroUSD, requestID),
		RequestID:      requestID,
		IdempotencyKey: reservationKey,
	})
	if err != nil {
		// Best-effort rollback: unfreeze
		if unfreezeErr := s.wallets.UnfreezeBalance(ctx, wallet.ID, estimatedCostMicroUSD); unfreezeErr != nil {
			slog.Error("reserve funds: failed to rollback freeze after ledger error",
				slog.String("wallet_id", wallet.ID),
				slog.String("error", unfreezeErr.Error()),
			)
		}
		return nil, fmt.Errorf("reserve funds: create reservation entry: %w", err)
	}

	// Return fresh wallet state
	return s.wallets.FindByID(ctx, wallet.ID)
}

// SettleCharge finalizes a charge after an API call completes.
//
// Steps:
//  1. Idempotency check via ledger entries
//  2. Get the customer wallet
//  3. Deduct the actual cost from frozen balance
//  4. If there is excess frozen (estimated > actual), release the difference
//  5. Create double-entry ledger: customer wallet credit (-actualCost),
//     platform wallet debit (+actualCost)
//
// The double-entry ensures: customer_paid + platform_earned = 0.
func (s *WalletService) SettleCharge(ctx context.Context, orgID, projectID, requestID string, actualCostMicroUSD int64) error {
	if actualCostMicroUSD <= 0 {
		return fmt.Errorf("settle charge: actual cost must be positive, got %d", actualCostMicroUSD)
	}

	// --- Idempotency check ---
	settleKey := "settle_" + requestID
	existing, err := s.ledger.FindByRequestID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("settle charge: idempotency check: %w", err)
	}
	for _, e := range existing {
		if e.EntryType == "settlement" {
			slog.Info("settle charge: request already settled (idempotent)",
				slog.String("request_id", requestID),
				slog.String("ledger_entry_id", e.ID),
			)
			return nil
		}
	}

	// --- Get customer wallet ---
	customerWallet, err := s.wallets.GetOrCreate(ctx, orgID, &projectID)
	if err != nil {
		return fmt.Errorf("settle charge: get customer wallet: %w", err)
	}

	// --- Get platform revenue wallet ---
	platformWallet, err := s.wallets.GetOrCreate(ctx, s.platformOrgID, nil)
	if err != nil {
		return fmt.Errorf("settle charge: get platform wallet: %w", err)
	}

	// --- Check that we have enough frozen ---
	if customerWallet.FrozenMicroUSD < actualCostMicroUSD {
		// Release whatever is frozen and report the shortfall
		if customerWallet.FrozenMicroUSD > 0 {
			_ = s.wallets.UnfreezeBalance(ctx, customerWallet.ID, customerWallet.FrozenMicroUSD)
		}
		return fmt.Errorf("settle charge: insufficient frozen: have %d, need %d",
			customerWallet.FrozenMicroUSD, actualCostMicroUSD)
	}

	// --- Deduct actual cost from frozen ---
	if err := s.wallets.DeductFrozen(ctx, customerWallet.ID, actualCostMicroUSD); err != nil {
		return fmt.Errorf("settle charge: deduct frozen: %w", err)
	}

	// --- Release excess frozen (estimated > actual) ---
	excess := customerWallet.FrozenMicroUSD - actualCostMicroUSD
	if excess > 0 {
		if err := s.wallets.UnfreezeBalance(ctx, customerWallet.ID, excess); err != nil {
			slog.Error("settle charge: failed to release excess frozen",
				slog.String("wallet_id", customerWallet.ID),
				slog.Int64("excess", excess),
				slog.String("error", err.Error()),
			)
		}
	}

	// --- Double-entry ledger: customer pays, platform earns ---
	debit := repository.LedgerEntryParams{
		OrganizationID: s.platformOrgID,
		WalletID:       platformWallet.ID,
		AmountMicroUSD: +actualCostMicroUSD, // platform earns (debit increases platform balance)
		EntryType:      "settlement",
		Description:    fmt.Sprintf("Revenue from request %s (org %s)", requestID, orgID),
		RequestID:      requestID,
		IdempotencyKey: "settle_platform_" + requestID,
	}
	credit := repository.LedgerEntryParams{
		OrganizationID: orgID,
		WalletID:       customerWallet.ID,
		AmountMicroUSD: -actualCostMicroUSD, // customer pays (credit decreases customer balance)
		EntryType:      "settlement",
		Description:    fmt.Sprintf("Charge for request %s: %d micro_usd", requestID, actualCostMicroUSD),
		RequestID:      requestID,
		IdempotencyKey: settleKey,
	}

	_, _, err = s.ledger.CreateDoubleEntry(ctx, debit, credit)
	if err != nil {
		return fmt.Errorf("settle charge: create double entry: %w", err)
	}

	// Audit log
	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			OrganizationID: orgID,
			Action:         "wallet.charge_settled",
			ResourceType:   "wallet",
			ResourceID:     customerWallet.ID,
			MetadataJSON:   fmt.Sprintf(`{"request_id":"%s","amount_micro_usd":%d}`, requestID, actualCostMicroUSD),
		})
	}

	slog.Info("charge settled",
		slog.String("request_id", requestID),
		slog.String("org_id", orgID),
		slog.String("customer_wallet", customerWallet.ID),
		slog.String("platform_wallet", platformWallet.ID),
		slog.Int64("amount_micro_usd", actualCostMicroUSD),
	)

	return nil
}

// ReleaseFunds releases frozen funds when a request fails or is cancelled
// (the upstream never accepted the request, or the request was rejected).
//
// Steps:
//  1. Get the customer wallet
//  2. Unfreeze the amount
//  3. Create a release ledger entry
func (s *WalletService) ReleaseFunds(ctx context.Context, orgID, projectID, requestID string) error {
	// --- Idempotency check ---
	releaseKey := "release_" + requestID
	existing, err := s.ledger.FindByRequestID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("release funds: idempotency check: %w", err)
	}
	for _, e := range existing {
		if e.EntryType == "release" {
			slog.Info("release funds: already released (idempotent)",
				slog.String("request_id", requestID),
			)
			return nil
		}
	}

	// --- Get customer wallet ---
	wallet, err := s.wallets.GetOrCreate(ctx, orgID, &projectID)
	if err != nil {
		return fmt.Errorf("release funds: get wallet: %w", err)
	}

	// --- Determine the amount to release ---
	// We look at the reservation entry to find how much was frozen.
	frozenAmount := wallet.FrozenMicroUSD
	if frozenAmount == 0 {
		// Nothing to release — may have already been settled or released
		return nil
	}

	reservationEntries, err := s.ledger.FindByRequestID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("release funds: find reservation: %w", err)
	}
	for _, e := range reservationEntries {
		if e.EntryType == "reservation" {
			// The reservation entry records a negative amount (credit)
			// The absolute value is the frozen amount
			if e.AmountMicroUSD < 0 {
				frozenAmount = -e.AmountMicroUSD
			}
			break
		}
	}

	// --- Unfreeze ---
	if err := s.wallets.UnfreezeBalance(ctx, wallet.ID, frozenAmount); err != nil {
		return fmt.Errorf("release funds: unfreeze: %w", err)
	}

	// --- Create release ledger entry ---
	_, err = s.ledger.CreateEntry(ctx, nil, repository.LedgerEntryParams{
		OrganizationID: orgID,
		WalletID:       wallet.ID,
		AmountMicroUSD: 0, // release is a zero-sum memo entry (unfreeze handled at wallet level)
		EntryType:      "release",
		Description:    fmt.Sprintf("Release frozen funds for cancelled request %s", requestID),
		RequestID:      requestID,
		IdempotencyKey: releaseKey,
	})
	if err != nil {
		// Non-fatal: the funds are already unfrozen at wallet level
		slog.Error("release funds: failed to create ledger entry after unfreeze",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
	}

	// Audit log
	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			OrganizationID: orgID,
			Action:         "wallet.funds_released",
			ResourceType:   "wallet",
			ResourceID:     wallet.ID,
			MetadataJSON:   fmt.Sprintf(`{"request_id":"%s","amount_micro_usd":%d}`, requestID, frozenAmount),
		})
	}

	slog.Info("funds released",
		slog.String("request_id", requestID),
		slog.String("org_id", orgID),
		slog.String("wallet_id", wallet.ID),
		slog.Int64("amount_micro_usd", frozenAmount),
	)

	return nil
}
