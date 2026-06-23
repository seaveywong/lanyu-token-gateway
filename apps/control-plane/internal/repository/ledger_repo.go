package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LedgerEntry represents a row in the ledger_entries table.
type LedgerEntry struct {
	ID               string    `json:"id"`
	OrganizationID   string    `json:"organization_id"`
	WalletID         string    `json:"wallet_id"`
	AmountMicroUSD   int64     `json:"amount_micro_usd"`   // positive = debit (increase), negative = credit (decrease)
	EntryType        string    `json:"entry_type"`          // reservation, settlement, release, payment, refund, adjustment
	Description      *string   `json:"description,omitempty"`
	RequestID        *string   `json:"request_id,omitempty"`
	IdempotencyKey   string    `json:"idempotency_key"`
	CreatedAt        time.Time `json:"created_at"`
}

// LedgerEntryParams holds the data needed to insert a single ledger entry.
type LedgerEntryParams struct {
	OrganizationID   string
	WalletID         string
	AmountMicroUSD   int64  // debit: positive, credit: negative
	EntryType        string
	Description      string
	RequestID        string
	IdempotencyKey   string
}

// CreateLedgerParams is an alias for backward compatibility.
// Deprecated: use LedgerEntryParams.
type CreateLedgerParams = LedgerEntryParams

// LedgerRepo provides write and read operations on the ledger_entries table.
// Implements double-entry accounting: every business transaction produces a
// matched pair of entries that sum to zero.
type LedgerRepo struct {
	pool *pgxpool.Pool
}

// NewLedgerRepo returns a LedgerRepo backed by the given connection pool.
func NewLedgerRepo(pool *pgxpool.Pool) *LedgerRepo {
	return &LedgerRepo{pool: pool}
}

// CreateEntry inserts a single ledger entry and returns it.
// When tx is nil, the insert uses the pool directly (for standalone entries).
// When tx is non-nil, the insert participates in the given transaction (for
// double-entry pairs).
func (r *LedgerRepo) CreateEntry(ctx context.Context, tx pgx.Tx, params LedgerEntryParams) (*LedgerEntry, error) {
	var desc *string
	if params.Description != "" {
		desc = &params.Description
	}

	var reqID *string
	if params.RequestID != "" {
		reqID = &params.RequestID
	}

	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx,
			`INSERT INTO ledger_entries (organization_id, wallet_id, amount_micro_usd,
			                             entry_type, description, request_id, idempotency_key)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id, organization_id, wallet_id, amount_micro_usd,
			           entry_type, description, request_id, idempotency_key, created_at`,
			params.OrganizationID, params.WalletID, params.AmountMicroUSD,
			params.EntryType, desc, reqID, params.IdempotencyKey,
		)
	} else {
		row = r.pool.QueryRow(ctx,
			`INSERT INTO ledger_entries (organization_id, wallet_id, amount_micro_usd,
			                             entry_type, description, request_id, idempotency_key)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id, organization_id, wallet_id, amount_micro_usd,
			           entry_type, description, request_id, idempotency_key, created_at`,
			params.OrganizationID, params.WalletID, params.AmountMicroUSD,
			params.EntryType, desc, reqID, params.IdempotencyKey,
		)
	}

	var entry LedgerEntry
	err := row.Scan(
		&entry.ID, &entry.OrganizationID, &entry.WalletID, &entry.AmountMicroUSD,
		&entry.EntryType, &entry.Description, &entry.RequestID, &entry.IdempotencyKey, &entry.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("create ledger entry: duplicate idempotency_key %q: %w", params.IdempotencyKey, err)
		}
		return nil, fmt.Errorf("create ledger entry: %w", err)
	}
	return &entry, nil
}

// CreateDoubleEntry creates a balanced pair (debit + credit) in a single
// database transaction. The two entries MUST sum to zero.
// Returns the two created entries in order: debit, credit.
func (r *LedgerRepo) CreateDoubleEntry(ctx context.Context, debit, credit LedgerEntryParams) (*LedgerEntry, *LedgerEntry, error) {
	// Validate that the pair sums to zero
	if debit.AmountMicroUSD+credit.AmountMicroUSD != 0 {
		return nil, nil, fmt.Errorf("create double entry: unbalanced pair (%d + %d = %d, want 0)",
			debit.AmountMicroUSD, credit.AmountMicroUSD, debit.AmountMicroUSD+credit.AmountMicroUSD)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create double entry: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	debitEntry, err := r.CreateEntry(ctx, tx, debit)
	if err != nil {
		return nil, nil, fmt.Errorf("create double entry: debit: %w", err)
	}

	creditEntry, err := r.CreateEntry(ctx, tx, credit)
	if err != nil {
		return nil, nil, fmt.Errorf("create double entry: credit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("create double entry: commit: %w", err)
	}

	return debitEntry, creditEntry, nil
}

// GetBalance returns the sum of all ledger entries for a wallet.
// This is the canonical balance — the wallets.balance_micro_usd column is a
// materialized cache. The ledger is the source of truth.
func (r *LedgerRepo) GetBalance(ctx context.Context, walletID string) (int64, error) {
	var sum *int64
	err := r.pool.QueryRow(ctx,
		`SELECT SUM(amount_micro_usd)
		 FROM ledger_entries
		 WHERE wallet_id = $1`,
		walletID,
	).Scan(&sum)
	if err != nil {
		return 0, fmt.Errorf("get ledger balance: %w", err)
	}
	if sum == nil {
		return 0, nil
	}
	return *sum, nil
}

// ListByWallet returns paginated ledger entries for a wallet, ordered by
// creation time descending (newest first). page is 1-based.
// Returns the entries and total count.
func (r *LedgerRepo) ListByWallet(ctx context.Context, walletID string, page, pageSize int) ([]LedgerEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_entries WHERE wallet_id = $1`, walletID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count ledger entries: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, wallet_id, amount_micro_usd,
		        entry_type, description, request_id, idempotency_key, created_at
		 FROM ledger_entries
		 WHERE wallet_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, walletID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.ID, &e.OrganizationID, &e.WalletID, &e.AmountMicroUSD,
			&e.EntryType, &e.Description, &e.RequestID, &e.IdempotencyKey, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan ledger entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter ledger entries: %w", err)
	}
	if entries == nil {
		entries = []LedgerEntry{}
	}

	return entries, total, nil
}

// FindByRequestID returns all ledger entries associated with a given request ID.
func (r *LedgerRepo) FindByRequestID(ctx context.Context, requestID string) ([]LedgerEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, wallet_id, amount_micro_usd,
		        entry_type, description, request_id, idempotency_key, created_at
		 FROM ledger_entries
		 WHERE request_id = $1
		 ORDER BY created_at`, requestID,
	)
	if err != nil {
		return nil, fmt.Errorf("find ledger by request: %w", err)
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.ID, &e.OrganizationID, &e.WalletID, &e.AmountMicroUSD,
			&e.EntryType, &e.Description, &e.RequestID, &e.IdempotencyKey, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter ledger entries: %w", err)
	}
	return entries, nil
}
