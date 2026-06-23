package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wallet represents a row in the wallets table.
type Wallet struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organization_id"`
	ProjectID       *string   `json:"project_id,omitempty"`
	BalanceMicroUSD int64     `json:"balance_micro_usd"`
	FrozenMicroUSD  int64     `json:"frozen_micro_usd"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// WalletRepo provides atomic wallet operations backed by PostgreSQL.
type WalletRepo struct {
	pool *pgxpool.Pool
}

// NewWalletRepo returns a WalletRepo backed by the given connection pool.
func NewWalletRepo(pool *pgxpool.Pool) *WalletRepo {
	return &WalletRepo{pool: pool}
}

// GetOrCreate returns the wallet for an org (and optional project).
// Creates the wallet if it does not already exist. Handles concurrent
// creation races by catching unique violations and retrying the lookup.
func (r *WalletRepo) GetOrCreate(ctx context.Context, orgID string, projectID *string) (*Wallet, error) {
	w, err := r.findByOrgAndProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	if w != nil {
		return w, nil
	}

	var newWallet Wallet
	err = r.pool.QueryRow(ctx,
		`INSERT INTO wallets (organization_id, project_id)
		 VALUES ($1, $2)
		 RETURNING id, organization_id, project_id,
		           balance_micro_usd, frozen_micro_usd, created_at, updated_at`,
		orgID, projectID,
	).Scan(&newWallet.ID, &newWallet.OrganizationID, &newWallet.ProjectID,
		&newWallet.BalanceMicroUSD, &newWallet.FrozenMicroUSD,
		&newWallet.CreatedAt, &newWallet.UpdatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			w, findErr := r.findByOrgAndProject(ctx, orgID, projectID)
			if findErr != nil {
				return nil, fmt.Errorf("get or create wallet after race: %w", findErr)
			}
			if w == nil {
				return nil, fmt.Errorf("get or create wallet: race resolved but wallet not found")
			}
			return w, nil
		}
		return nil, fmt.Errorf("get or create wallet: %w", err)
	}
	return &newWallet, nil
}

// GetByOrgID returns the org-level wallet (project_id IS NULL).
func (r *WalletRepo) GetByOrgID(ctx context.Context, orgID string) (*Wallet, error) {
	return r.findByOrgAndProject(ctx, orgID, nil)
}

// GetOrCreateWallet returns the organization-level wallet, creating it if it
// does not exist. Convenience wrapper over GetOrCreate for org-level wallets.
// Deprecated: prefer GetOrCreate for full org+project support.
func (r *WalletRepo) GetOrCreateWallet(ctx context.Context, orgID string) (*Wallet, error) {
	return r.GetOrCreate(ctx, orgID, nil)
}

// FindByOrgID returns the org-level wallet. Returns nil if not found.
func (r *WalletRepo) FindByOrgID(ctx context.Context, orgID string) (*Wallet, error) {
	return r.findByOrgAndProject(ctx, orgID, nil)
}

// FindByID returns a wallet by its primary key. Returns nil if not found.
func (r *WalletRepo) FindByID(ctx context.Context, id string) (*Wallet, error) {
	var w Wallet
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, project_id,
		        balance_micro_usd, frozen_micro_usd, created_at, updated_at
		 FROM wallets WHERE id = $1`, id,
	).Scan(&w.ID, &w.OrganizationID, &w.ProjectID,
		&w.BalanceMicroUSD, &w.FrozenMicroUSD, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, handleNoRows(err)
	}
	return &w, nil
}

// findByOrgAndProject looks up a wallet by org and optional project.
// Uses IS NOT DISTINCT FROM for NULL-safe comparison.
func (r *WalletRepo) findByOrgAndProject(ctx context.Context, orgID string, projectID *string) (*Wallet, error) {
	var w Wallet
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, project_id,
		        balance_micro_usd, frozen_micro_usd, created_at, updated_at
		 FROM wallets
		 WHERE organization_id = $1 AND project_id IS NOT DISTINCT FROM $2`,
		orgID, projectID,
	).Scan(&w.ID, &w.OrganizationID, &w.ProjectID,
		&w.BalanceMicroUSD, &w.FrozenMicroUSD, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, handleNoRows(err)
	}
	return &w, nil
}

// FreezeBalance atomically moves amount from balance to frozen.
// Returns an error if the wallet does not exist or balance is insufficient.
func (r *WalletRepo) FreezeBalance(ctx context.Context, id string, amountMicroUSD int64) error {
	if amountMicroUSD <= 0 {
		return fmt.Errorf("freeze balance: amount must be positive, got %d", amountMicroUSD)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE wallets
		 SET balance_micro_usd = balance_micro_usd - $2,
		     frozen_micro_usd  = frozen_micro_usd + $2,
		     updated_at = NOW()
		 WHERE id = $1 AND balance_micro_usd >= $2`,
		id, amountMicroUSD,
	)
	if err != nil {
		return fmt.Errorf("freeze balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("freeze balance: insufficient balance or wallet %s not found", id)
	}
	return nil
}

// UnfreezeBalance atomically moves amount from frozen back to balance.
func (r *WalletRepo) UnfreezeBalance(ctx context.Context, id string, amountMicroUSD int64) error {
	if amountMicroUSD <= 0 {
		return fmt.Errorf("unfreeze balance: amount must be positive, got %d", amountMicroUSD)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE wallets
		 SET balance_micro_usd = balance_micro_usd + $2,
		     frozen_micro_usd  = frozen_micro_usd - $2,
		     updated_at = NOW()
		 WHERE id = $1 AND frozen_micro_usd >= $2`,
		id, amountMicroUSD,
	)
	if err != nil {
		return fmt.Errorf("unfreeze balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("unfreeze balance: insufficient frozen or wallet %s not found", id)
	}
	return nil
}

// DeductFrozen atomically reduces frozen balance (and thus total balance).
// Used after a request completes to settle the actual charge.
func (r *WalletRepo) DeductFrozen(ctx context.Context, id string, amountMicroUSD int64) error {
	if amountMicroUSD <= 0 {
		return fmt.Errorf("deduct frozen: amount must be positive, got %d", amountMicroUSD)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE wallets
		 SET frozen_micro_usd = frozen_micro_usd - $2,
		     updated_at = NOW()
		 WHERE id = $1 AND frozen_micro_usd >= $2`,
		id, amountMicroUSD,
	)
	if err != nil {
		return fmt.Errorf("deduct frozen: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("deduct frozen: insufficient frozen or wallet %s not found", id)
	}
	return nil
}

// CreditBalance adds funds to balance (from payment, refund, or admin adjustment).
// Takes wallet ID (not org ID) for precise targeting.
func (r *WalletRepo) CreditBalance(ctx context.Context, id string, amountMicroUSD int64) error {
	if amountMicroUSD <= 0 {
		return fmt.Errorf("credit balance: amount must be positive, got %d", amountMicroUSD)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE wallets
		 SET balance_micro_usd = balance_micro_usd + $2,
		     updated_at = NOW()
		 WHERE id = $1`,
		id, amountMicroUSD,
	)
	if err != nil {
		return fmt.Errorf("credit balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("credit balance: wallet %s not found", id)
	}
	return nil
}

// CreditBalanceByOrg adds funds to the org-level wallet by organization ID.
// Convenience method for payment processing where only org ID is available.
func (r *WalletRepo) CreditBalanceByOrg(ctx context.Context, orgID string, amountMicroUSD int64) (int64, error) {
	var newBalance int64
	err := r.pool.QueryRow(ctx,
		`UPDATE wallets
		 SET balance_micro_usd = balance_micro_usd + $2,
		     updated_at = NOW()
		 WHERE organization_id = $1 AND project_id IS NULL
		 RETURNING balance_micro_usd`,
		orgID, amountMicroUSD,
	).Scan(&newBalance)
	if err != nil {
		return 0, fmt.Errorf("credit wallet balance by org: %w", err)
	}
	return newBalance, nil
}

// GetBalance returns the current balance for an organization's org-level wallet.
func (r *WalletRepo) GetBalance(ctx context.Context, orgID string) (int64, error) {
	var balance int64
	err := r.pool.QueryRow(ctx,
		`SELECT balance_micro_usd
		 FROM wallets
		 WHERE organization_id = $1 AND project_id IS NULL`,
		orgID,
	).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("get wallet balance: %w", err)
	}
	return balance, nil
}

// handleNoRows converts a "no rows" error to a nil return.
func handleNoRows(err error) error {
	if err == pgx.ErrNoRows {
		return nil
	}
	return err
}

// Compile-time check that pgx is imported (used for ErrNoRows sentinel).
var _ = pgx.ErrNoRows
