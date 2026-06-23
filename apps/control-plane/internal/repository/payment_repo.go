package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// PaymentOrder represents a row in the payment_orders table.
type PaymentOrder struct {
	ID                  string     `json:"id"`
	OrganizationID      string     `json:"organization_id"`
	OrderNo             string     `json:"order_no"`
	PaymentMethod       string     `json:"payment_method"`
	AmountMicroUSD      int64      `json:"amount_micro_usd"`
	AmountYuan          int        `json:"amount_yuan"`
	Status              string     `json:"status"`
	ProviderTradeNo     *string    `json:"provider_trade_no,omitempty"`
	ProviderCallbackRaw *string    `json:"provider_callback_raw,omitempty"`
	CreditedAt          *time.Time `json:"credited_at,omitempty"`
	ExpiresAt           time.Time  `json:"expires_at"`
	CreatedBy           *string    `json:"created_by,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// PaymentWebhook represents a row in the payment_webhooks table.
type PaymentWebhook struct {
	ID               string     `json:"id"`
	PaymentOrderID   *string    `json:"payment_order_id"`
	Provider         string     `json:"provider"`
	EventType        string     `json:"event_type"`
	RawBody          string     `json:"raw_body"`
	SignatureVerified bool      `json:"signature_verified"`
	IdempotencyKey   string     `json:"idempotency_key"`
	Processed        bool       `json:"processed"`
	ProcessedAt      *time.Time `json:"processed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// RefundRequest represents a row in the refund_requests table.
type RefundRequest struct {
	ID             string     `json:"id"`
	PaymentOrderID string     `json:"payment_order_id"`
	AmountMicroUSD int64      `json:"amount_micro_usd"`
	AmountYuan     int        `json:"amount_yuan"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	ApprovedBy     *string    `json:"approved_by,omitempty"`
	LedgerEntryID  *string    `json:"ledger_entry_id,omitempty"`
	CreatedBy      *string    `json:"created_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Params types
// ---------------------------------------------------------------------------

// CreateOrderParams holds the data needed to insert a payment order.
type CreateOrderParams struct {
	OrganizationID string
	OrderNo        string
	PaymentMethod  string
	AmountMicroUSD int64
	AmountYuan     int
	ExpiresAt      time.Time
	CreatedBy      string
}

// CreateWebhookParams holds the data needed to insert a payment webhook.
type CreateWebhookParams struct {
	PaymentOrderID   string
	Provider         string
	EventType        string
	RawBody          string
	SignatureVerified bool
	IdempotencyKey   string
}

// CreateRefundParams holds the data needed to insert a refund request.
type CreateRefundParams struct {
	PaymentOrderID string
	AmountMicroUSD int64
	AmountYuan     int
	Reason         string
	CreatedBy      string
}

// ---------------------------------------------------------------------------
// PaymentRepo
// ---------------------------------------------------------------------------

// PaymentRepo provides CRUD operations on payment-related tables.
type PaymentRepo struct {
	pool *pgxpool.Pool
}

// NewPaymentRepo returns a PaymentRepo backed by the given connection pool.
func NewPaymentRepo(pool *pgxpool.Pool) *PaymentRepo {
	return &PaymentRepo{pool: pool}
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

// CreateOrder inserts a new payment order.
func (r *PaymentRepo) CreateOrder(ctx context.Context, params CreateOrderParams) (*PaymentOrder, error) {
	var o PaymentOrder
	err := r.pool.QueryRow(ctx,
		`INSERT INTO payment_orders (organization_id, order_no, payment_method,
		                             amount_micro_usd, amount_yuan, expires_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, organization_id, order_no, payment_method,
		           amount_micro_usd, amount_yuan, status,
		           provider_trade_no, provider_callback_raw,
		           credited_at, expires_at, created_by, created_at, updated_at`,
		params.OrganizationID, params.OrderNo, params.PaymentMethod,
		params.AmountMicroUSD, params.AmountYuan, params.ExpiresAt, params.CreatedBy,
	).Scan(
		&o.ID, &o.OrganizationID, &o.OrderNo, &o.PaymentMethod,
		&o.AmountMicroUSD, &o.AmountYuan, &o.Status,
		&o.ProviderTradeNo, &o.ProviderCallbackRaw,
		&o.CreditedAt, &o.ExpiresAt, &o.CreatedBy, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create payment order: %w", err)
	}
	return &o, nil
}

// FindOrderByNo retrieves a payment order by its platform order number.
func (r *PaymentRepo) FindOrderByNo(ctx context.Context, orderNo string) (*PaymentOrder, error) {
	var o PaymentOrder
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, order_no, payment_method,
		        amount_micro_usd, amount_yuan, status,
		        provider_trade_no, provider_callback_raw,
		        credited_at, expires_at, created_by, created_at, updated_at
		 FROM payment_orders
		 WHERE order_no = $1`, orderNo,
	).Scan(
		&o.ID, &o.OrganizationID, &o.OrderNo, &o.PaymentMethod,
		&o.AmountMicroUSD, &o.AmountYuan, &o.Status,
		&o.ProviderTradeNo, &o.ProviderCallbackRaw,
		&o.CreditedAt, &o.ExpiresAt, &o.CreatedBy, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("payment order %q not found: %w", orderNo, err)
		}
		return nil, fmt.Errorf("find payment order by no: %w", err)
	}
	return &o, nil
}

// FindOrderByID retrieves a payment order by its UUID.
func (r *PaymentRepo) FindOrderByID(ctx context.Context, id string) (*PaymentOrder, error) {
	var o PaymentOrder
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, order_no, payment_method,
		        amount_micro_usd, amount_yuan, status,
		        provider_trade_no, provider_callback_raw,
		        credited_at, expires_at, created_by, created_at, updated_at
		 FROM payment_orders
		 WHERE id = $1`, id,
	).Scan(
		&o.ID, &o.OrganizationID, &o.OrderNo, &o.PaymentMethod,
		&o.AmountMicroUSD, &o.AmountYuan, &o.Status,
		&o.ProviderTradeNo, &o.ProviderCallbackRaw,
		&o.CreditedAt, &o.ExpiresAt, &o.CreatedBy, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("payment order %q not found: %w", id, err)
		}
		return nil, fmt.Errorf("find payment order by id: %w", err)
	}
	return &o, nil
}

// UpdateOrderStatus updates the status and optional provider trade number of a
// payment order. It also sets updated_at to NOW().
func (r *PaymentRepo) UpdateOrderStatus(ctx context.Context, id, status, providerTradeNo string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE payment_orders
		 SET status = $2, provider_trade_no = $3, updated_at = NOW()
		 WHERE id = $1`, id, status, providerTradeNo,
	)
	if err != nil {
		return fmt.Errorf("update payment order status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("payment order %q not found for status update", id)
	}
	return nil
}

// MarkOrderCredited sets the order status to 'credited' and records the
// credited_at timestamp.
func (r *PaymentRepo) MarkOrderCredited(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE payment_orders
		 SET status = 'credited', credited_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND status = 'paid'`, id,
	)
	if err != nil {
		return fmt.Errorf("mark order credited: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("payment order %q not in 'paid' status for crediting", id)
	}
	return nil
}

// ListOrdersByOrg returns paginated payment orders for an organization.
// Returns the orders and the total count.
func (r *PaymentRepo) ListOrdersByOrg(ctx context.Context, orgID string, page, pageSize int) ([]PaymentOrder, int, error) {
	offset := (page - 1) * pageSize

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payment_orders WHERE organization_id = $1`, orgID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count payment orders: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, order_no, payment_method,
		        amount_micro_usd, amount_yuan, status,
		        provider_trade_no, provider_callback_raw,
		        credited_at, expires_at, created_by, created_at, updated_at
		 FROM payment_orders
		 WHERE organization_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, orgID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list payment orders: %w", err)
	}
	defer rows.Close()

	return scanPaymentOrders(rows, total)
}

// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

// CreateWebhook inserts a new payment webhook record.
// The idempotency_key UNIQUE constraint prevents duplicate processing.
func (r *PaymentRepo) CreateWebhook(ctx context.Context, params CreateWebhookParams) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO payment_webhooks (payment_order_id, provider, event_type,
		                               raw_body, signature_verified, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		params.PaymentOrderID, params.Provider, params.EventType,
		params.RawBody, params.SignatureVerified, params.IdempotencyKey,
	)
	if err != nil {
		return fmt.Errorf("create payment webhook: %w", err)
	}
	return nil
}

// MarkWebhookProcessed marks a webhook as processed.
func (r *PaymentRepo) MarkWebhookProcessed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payment_webhooks
		 SET processed = TRUE, processed_at = NOW()
		 WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("mark webhook processed: %w", err)
	}
	return nil
}

// FindWebhookByOrderAndKey finds a webhook for a given order and idempotency key.
// Used during callback handling to check for duplicates.
func (r *PaymentRepo) FindWebhookByOrderAndKey(ctx context.Context, paymentOrderID, idempotencyKey string) (*PaymentWebhook, error) {
	var wh PaymentWebhook
	err := r.pool.QueryRow(ctx,
		`SELECT id, payment_order_id, provider, event_type, raw_body,
		        signature_verified, idempotency_key, processed, processed_at, created_at
		 FROM payment_webhooks
		 WHERE payment_order_id = $1 AND idempotency_key = $2`,
		paymentOrderID, idempotencyKey,
	).Scan(
		&wh.ID, &wh.PaymentOrderID, &wh.Provider, &wh.EventType, &wh.RawBody,
		&wh.SignatureVerified, &wh.IdempotencyKey, &wh.Processed, &wh.ProcessedAt, &wh.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // not found is not an error
		}
		return nil, fmt.Errorf("find webhook by order and key: %w", err)
	}
	return &wh, nil
}

// ---------------------------------------------------------------------------
// Refunds
// ---------------------------------------------------------------------------

// CreateRefundRequest inserts a new refund request.
func (r *PaymentRepo) CreateRefundRequest(ctx context.Context, params CreateRefundParams) (*RefundRequest, error) {
	var ref RefundRequest
	err := r.pool.QueryRow(ctx,
		`INSERT INTO refund_requests (payment_order_id, amount_micro_usd,
		                              amount_yuan, reason, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, payment_order_id, amount_micro_usd, amount_yuan,
		           reason, status, approved_by, ledger_entry_id,
		           created_by, created_at, updated_at`,
		params.PaymentOrderID, params.AmountMicroUSD,
		params.AmountYuan, params.Reason, params.CreatedBy,
	).Scan(
		&ref.ID, &ref.PaymentOrderID, &ref.AmountMicroUSD, &ref.AmountYuan,
		&ref.Reason, &ref.Status, &ref.ApprovedBy, &ref.LedgerEntryID,
		&ref.CreatedBy, &ref.CreatedAt, &ref.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create refund request: %w", err)
	}
	return &ref, nil
}

// UpdateRefundStatus updates the status and optional ledger entry of a refund.
func (r *PaymentRepo) UpdateRefundStatus(ctx context.Context, id, status string, ledgerEntryID *string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE refund_requests
		 SET status = $2, ledger_entry_id = $3, updated_at = NOW()
		 WHERE id = $1`, id, status, ledgerEntryID,
	)
	if err != nil {
		return fmt.Errorf("update refund status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("refund request %q not found for status update", id)
	}
	return nil
}

// FindRefundByID retrieves a refund request by its UUID.
func (r *PaymentRepo) FindRefundByID(ctx context.Context, id string) (*RefundRequest, error) {
	var ref RefundRequest
	err := r.pool.QueryRow(ctx,
		`SELECT id, payment_order_id, amount_micro_usd, amount_yuan,
		        reason, status, approved_by, ledger_entry_id,
		        created_by, created_at, updated_at
		 FROM refund_requests
		 WHERE id = $1`, id,
	).Scan(
		&ref.ID, &ref.PaymentOrderID, &ref.AmountMicroUSD, &ref.AmountYuan,
		&ref.Reason, &ref.Status, &ref.ApprovedBy, &ref.LedgerEntryID,
		&ref.CreatedBy, &ref.CreatedAt, &ref.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("refund request %q not found: %w", id, err)
		}
		return nil, fmt.Errorf("find refund by id: %w", err)
	}
	return &ref, nil
}

// ListRefundsByOrg returns paginated refund requests for an organization.
func (r *PaymentRepo) ListRefundsByOrg(ctx context.Context, orgID string, page, pageSize int) ([]RefundRequest, int, error) {
	offset := (page - 1) * pageSize

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM refund_requests rf
		 JOIN payment_orders po ON rf.payment_order_id = po.id
		 WHERE po.organization_id = $1`, orgID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count refund requests: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT rf.id, rf.payment_order_id, rf.amount_micro_usd, rf.amount_yuan,
		        rf.reason, rf.status, rf.approved_by, rf.ledger_entry_id,
		        rf.created_by, rf.created_at, rf.updated_at
		 FROM refund_requests rf
		 JOIN payment_orders po ON rf.payment_order_id = po.id
		 WHERE po.organization_id = $1
		 ORDER BY rf.created_at DESC
		 LIMIT $2 OFFSET $3`, orgID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list refund requests: %w", err)
	}
	defer rows.Close()

	var refs []RefundRequest
	for rows.Next() {
		var ref RefundRequest
		if err := rows.Scan(
			&ref.ID, &ref.PaymentOrderID, &ref.AmountMicroUSD, &ref.AmountYuan,
			&ref.Reason, &ref.Status, &ref.ApprovedBy, &ref.LedgerEntryID,
			&ref.CreatedBy, &ref.CreatedAt, &ref.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan refund request: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter refund requests: %w", err)
	}
	if refs == nil {
		refs = []RefundRequest{}
	}

	return refs, total, nil
}

// ---------------------------------------------------------------------------
// Reconciliation
// ---------------------------------------------------------------------------

// ReconciliationRun represents a row in the reconciliation_runs table.
type ReconciliationRun struct {
	ID                    string     `json:"id"`
	RunDate               time.Time  `json:"run_date"`
	Status                string     `json:"status"`
	TotalProviderCharges  int64      `json:"total_provider_charges"`
	TotalPlatformRecords  int64      `json:"total_platform_records"`
	TotalLedgerEntries    int64      `json:"total_ledger_entries"`
	DiscrepancyCount      int        `json:"discrepancy_count"`
	DiscrepancyMicroUSD   int64      `json:"discrepancy_micro_usd"`
	StartedAt             time.Time  `json:"started_at"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
}

// ReconciliationItem represents a row in the reconciliation_items table.
type ReconciliationItem struct {
	ID               string    `json:"id"`
	RunID            string    `json:"run_id"`
	DiscrepancyType  string    `json:"discrepancy_type"`
	RequestID        *string   `json:"request_id,omitempty"`
	ProviderAmount   *int64    `json:"provider_amount,omitempty"`
	PlatformAmount   *int64    `json:"platform_amount,omitempty"`
	LedgerAmount     *int64    `json:"ledger_amount,omitempty"`
	DiffMicroUSD     int64     `json:"diff_micro_usd"`
	ResolutionStatus string    `json:"resolution_status"`
	Notes            *string   `json:"notes,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// CreateReconciliationRun inserts a new reconciliation run.
func (r *PaymentRepo) CreateReconciliationRun(ctx context.Context, runDate time.Time) (*ReconciliationRun, error) {
	var run ReconciliationRun
	err := r.pool.QueryRow(ctx,
		`INSERT INTO reconciliation_runs (run_date)
		 VALUES ($1)
		 RETURNING id, run_date, status, total_provider_charges,
		           total_platform_records, total_ledger_entries,
		           discrepancy_count, discrepancy_micro_usd,
		           started_at, completed_at`, runDate,
	).Scan(
		&run.ID, &run.RunDate, &run.Status, &run.TotalProviderCharges,
		&run.TotalPlatformRecords, &run.TotalLedgerEntries,
		&run.DiscrepancyCount, &run.DiscrepancyMicroUSD,
		&run.StartedAt, &run.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create reconciliation run: %w", err)
	}
	return &run, nil
}

// CompleteReconciliationRun marks a reconciliation run as completed with summary
// statistics.
func (r *PaymentRepo) CompleteReconciliationRun(ctx context.Context, id string, providerCharges, platformRecords, ledgerEntries, discrepancyMicroUSD int64, discrepancyCount int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE reconciliation_runs
		 SET status = 'completed',
		     total_provider_charges = $2,
		     total_platform_records = $3,
		     total_ledger_entries = $4,
		     discrepancy_count = $5,
		     discrepancy_micro_usd = $6,
		     completed_at = NOW()
		 WHERE id = $1`, id, providerCharges, platformRecords, ledgerEntries, discrepancyCount, discrepancyMicroUSD,
	)
	if err != nil {
		return fmt.Errorf("complete reconciliation run: %w", err)
	}
	return nil
}

// FailReconciliationRun marks a reconciliation run as failed.
func (r *PaymentRepo) FailReconciliationRun(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE reconciliation_runs
		 SET status = 'failed', completed_at = NOW()
		 WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("fail reconciliation run: %w", err)
	}
	return nil
}

// CreateReconciliationItem inserts a new discrepancy item.
func (r *PaymentRepo) CreateReconciliationItem(ctx context.Context, runID, discrepancyType string, requestID *string, providerAmount, platformAmount, ledgerAmount, diffMicroUSD int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO reconciliation_items (run_id, discrepancy_type, request_id,
		                                   provider_amount, platform_amount,
		                                   ledger_amount, diff_micro_usd)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		runID, discrepancyType, requestID,
		providerAmount, platformAmount, ledgerAmount, diffMicroUSD,
	)
	if err != nil {
		return fmt.Errorf("create reconciliation item: %w", err)
	}
	return nil
}

// ListReconciliationItems returns all items for a given run.
func (r *PaymentRepo) ListReconciliationItems(ctx context.Context, runID string) ([]ReconciliationItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, run_id, discrepancy_type, request_id,
		        provider_amount, platform_amount, ledger_amount,
		        diff_micro_usd, resolution_status, notes, created_at
		 FROM reconciliation_items
		 WHERE run_id = $1
		 ORDER BY created_at`, runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list reconciliation items: %w", err)
	}
	defer rows.Close()

	var items []ReconciliationItem
	for rows.Next() {
		var item ReconciliationItem
		if err := rows.Scan(
			&item.ID, &item.RunID, &item.DiscrepancyType, &item.RequestID,
			&item.ProviderAmount, &item.PlatformAmount, &item.LedgerAmount,
			&item.DiffMicroUSD, &item.ResolutionStatus, &item.Notes, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan reconciliation item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter reconciliation items: %w", err)
	}
	if items == nil {
		items = []ReconciliationItem{}
	}
	return items, nil
}

// UpdateReconciliationItem updates the resolution status and notes for an item.
func (r *PaymentRepo) UpdateReconciliationItem(ctx context.Context, id, resolution, notes string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE reconciliation_items
		 SET resolution_status = $2, notes = $3
		 WHERE id = $1`, id, resolution, notes,
	)
	if err != nil {
		return fmt.Errorf("update reconciliation item: %w", err)
	}
	return nil
}

// FindReconciliationRunByDate finds a reconciliation run for a given date.
func (r *PaymentRepo) FindReconciliationRunByDate(ctx context.Context, runDate time.Time) (*ReconciliationRun, error) {
	var run ReconciliationRun
	err := r.pool.QueryRow(ctx,
		`SELECT id, run_date, status, total_provider_charges,
		        total_platform_records, total_ledger_entries,
		        discrepancy_count, discrepancy_micro_usd,
		        started_at, completed_at
		 FROM reconciliation_runs
		 WHERE run_date = $1
		 ORDER BY started_at DESC
		 LIMIT 1`, runDate,
	).Scan(
		&run.ID, &run.RunDate, &run.Status, &run.TotalProviderCharges,
		&run.TotalPlatformRecords, &run.TotalLedgerEntries,
		&run.DiscrepancyCount, &run.DiscrepancyMicroUSD,
		&run.StartedAt, &run.CompletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find reconciliation run by date: %w", err)
	}
	return &run, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// scanPaymentOrders reads payment_order rows into a slice.
func scanPaymentOrders(rows pgx.Rows, total int) ([]PaymentOrder, int, error) {
	var orders []PaymentOrder
	for rows.Next() {
		var o PaymentOrder
		if err := rows.Scan(
			&o.ID, &o.OrganizationID, &o.OrderNo, &o.PaymentMethod,
			&o.AmountMicroUSD, &o.AmountYuan, &o.Status,
			&o.ProviderTradeNo, &o.ProviderCallbackRaw,
			&o.CreditedAt, &o.ExpiresAt, &o.CreatedBy, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan payment order: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter payment orders: %w", err)
	}
	if orders == nil {
		orders = []PaymentOrder{}
	}
	return orders, total, nil
}
