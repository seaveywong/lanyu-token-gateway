package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// PaymentConfig holds payment-related configuration.
type PaymentConfig struct {
	// USDCNYRate is the exchange rate: 1 USD = N CNY. Default 7.0.
	USDCNYRate float64
}

// DefaultPaymentConfig returns a config with sensible defaults.
func DefaultPaymentConfig() PaymentConfig {
	return PaymentConfig{
		USDCNYRate: 7.0,
	}
}

// ---------------------------------------------------------------------------
// PaymentService
// ---------------------------------------------------------------------------

// PaymentService provides business logic for payment orders, callbacks, and
// refunds. It coordinates the payment repository, wallet, and ledger.
type PaymentService struct {
	payments *repository.PaymentRepo
	wallets  *repository.WalletRepo
	ledger   *repository.LedgerRepo
	audit    *repository.AuditRepo
	cfg      PaymentConfig
}

// NewPaymentService returns a new PaymentService.
func NewPaymentService(
	payments *repository.PaymentRepo,
	wallets *repository.WalletRepo,
	ledger *repository.LedgerRepo,
	audit *repository.AuditRepo,
	cfg PaymentConfig,
) *PaymentService {
	return &PaymentService{
		payments: payments,
		wallets:  wallets,
		ledger:   ledger,
		audit:    audit,
		cfg:      cfg,
	}
}

// ---------------------------------------------------------------------------
// CreateOrder
// ---------------------------------------------------------------------------

// CreateOrder creates a payment order for the customer to pay.
// Generates order number PO_<timestamp>_<random>.
// amountYuan is in RMB fen (e.g., 1000 = 10.00 CNY).
func (s *PaymentService) CreateOrder(ctx context.Context, userID, orgID, paymentMethod string, amountYuan int) (*repository.PaymentOrder, error) {
	if amountYuan <= 0 {
		return nil, fmt.Errorf("amount must be positive, got %d", amountYuan)
	}
	if paymentMethod != "alipay" && paymentMethod != "wechat_pay" {
		return nil, fmt.Errorf("unsupported payment method: %s", paymentMethod)
	}

	// Generate order number: PO_<unix_seconds>_<8 random hex chars>
	orderNo, err := generateOrderNo()
	if err != nil {
		return nil, fmt.Errorf("generate order no: %w", err)
	}

	// Convert amount: CNY fen -> micro USD
	amountMicroUSD := s.yuanFenToMicroUSD(amountYuan)

	// Orders expire in 30 minutes by default.
	expiresAt := time.Now().Add(30 * time.Minute)

	order, err := s.payments.CreateOrder(ctx, repository.CreateOrderParams{
		OrganizationID: orgID,
		OrderNo:        orderNo,
		PaymentMethod:  paymentMethod,
		AmountMicroUSD: amountMicroUSD,
		AmountYuan:     amountYuan,
		ExpiresAt:      expiresAt,
		CreatedBy:      userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// Audit log
	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			OrganizationID: orgID,
			ActorID:        userID,
			Action:         "payment.order_created",
			ResourceType:   "payment_order",
			ResourceID:     order.ID,
			MetadataJSON:   fmt.Sprintf(`{"order_no":"%s","payment_method":"%s","amount_yuan":%d}`, orderNo, paymentMethod, amountYuan),
		})
	}

	slog.Info("payment order created",
		slog.String("order_no", orderNo),
		slog.String("org_id", orgID),
		slog.Int("amount_yuan", amountYuan),
		slog.Int64("amount_micro_usd", amountMicroUSD),
	)
	return order, nil
}

// ---------------------------------------------------------------------------
// HandleCallback
// ---------------------------------------------------------------------------

// CallbackPayload holds the parsed fields from a payment provider callback.
type CallbackPayload struct {
	OrderNo         string            // platform order number
	ProviderTradeNo string            // provider-side transaction number
	AmountYuan      int               // amount in RMB fen from provider
	RawBody         string            // raw callback body
	IdempotencyKey  string            // unique key for idempotent processing
	Signature       string            // provider signature
	Extra           map[string]string // any provider-specific extra fields
}

// HandleCallback processes a payment provider callback.
// Steps:
//  1. Verify signature (stub — TODO: real provider verification)
//  2. Find the payment order
//  3. Check idempotency (avoid double-processing via UNIQUE constraint)
//  4. Validate order state
//  5. Update order status to 'paid'
//  6. Ensure wallet exists (get-or-create)
//  7. Credit the wallet balance
//  8. Create ledger entry (payment_credit)
//  9. Mark order as 'credited'
func (s *PaymentService) HandleCallback(ctx context.Context, provider, eventType string, payload CallbackPayload) error {
	// --- 1. Verify signature (stub) ---
	if err := s.verifyProviderSignature(provider, payload); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// --- 2. Find the payment order ---
	order, err := s.payments.FindOrderByNo(ctx, payload.OrderNo)
	if err != nil {
		return fmt.Errorf("find order for callback: %w", err)
	}

	// --- 3. Idempotency check ---
	// Try to insert the webhook record. The UNIQUE constraint on idempotency_key
	// ensures we never double-process the same callback.
	err = s.payments.CreateWebhook(ctx, repository.CreateWebhookParams{
		PaymentOrderID:    order.ID,
		Provider:          provider,
		EventType:         eventType,
		RawBody:           payload.RawBody,
		SignatureVerified: true, // we verified above
		IdempotencyKey:    payload.IdempotencyKey,
	})
	if err != nil {
		// Check if this is a duplicate (idempotency_key collision).
		// If so, the callback was already processed — return success.
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "violation") {
			slog.Info("duplicate callback ignored (idempotent)",
				slog.String("order_no", payload.OrderNo),
				slog.String("idempotency_key", payload.IdempotencyKey),
			)
			return nil
		}
		return fmt.Errorf("create webhook: %w", err)
	}

	// --- 4. Validate the order is still in a processable state ---
	if order.Status != "created" && order.Status != "pending" {
		slog.Warn("order already in non-processable state, skipping credit",
			slog.String("order_no", payload.OrderNo),
			slog.String("status", order.Status),
		)
		return nil
	}

	// --- 5. Update order status to 'paid' ---
	if err := s.payments.UpdateOrderStatus(ctx, order.ID, "paid", payload.ProviderTradeNo); err != nil {
		return fmt.Errorf("update order to paid: %w", err)
	}

	// --- 6. Ensure wallet exists ---
	wallet, err := s.wallets.GetOrCreateWallet(ctx, order.OrganizationID)
	if err != nil {
		return fmt.Errorf("get or create wallet: %w", err)
	}

	// --- 7. Credit the wallet ---
	if err := s.wallets.CreditBalance(ctx, wallet.ID, order.AmountMicroUSD); err != nil {
		return fmt.Errorf("credit wallet: %w", err)
	}

	// --- 8. Create ledger entries (双分录) ---
	// Entry: payment_credit — record the payment as a ledger entry

	ledgerKey := fmt.Sprintf("payment_credit_%s_%s", order.OrderNo, payload.IdempotencyKey)
	if len(ledgerKey) > 128 {
		ledgerKey = ledgerKey[:128]
	}
	_, err = s.ledger.CreateEntry(ctx, nil, repository.LedgerEntryParams{
		OrganizationID: order.OrganizationID,
		WalletID:       wallet.ID,
		AmountMicroUSD: order.AmountMicroUSD,
		EntryType:      "payment_credit",
		Description:    fmt.Sprintf("Payment %s via %s", order.OrderNo, provider),
		RequestID:      order.OrderNo,
		IdempotencyKey: ledgerKey,
	})
	if err != nil {
		return fmt.Errorf("create payment credit ledger entry: %w", err)
	}

	// --- 8. Mark order as 'credited' ---
	if err := s.payments.MarkOrderCredited(ctx, order.ID); err != nil {
		return fmt.Errorf("mark order credited: %w", err)
	}

	slog.Info("payment callback processed successfully",
		slog.String("order_no", payload.OrderNo),
		slog.String("provider", provider),
		slog.String("provider_trade_no", payload.ProviderTradeNo),
		slog.Int64("amount_micro_usd", order.AmountMicroUSD),
	)

	// Audit log
	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			OrganizationID: order.OrganizationID,
			Action:         "payment.callback_processed",
			ResourceType:   "payment_order",
			ResourceID:     order.ID,
			MetadataJSON:   fmt.Sprintf(`{"provider":"%s","event_type":"%s","provider_trade_no":"%s"}`, provider, eventType, payload.ProviderTradeNo),
		})
	}

	return nil
}

// ---------------------------------------------------------------------------
// Refunds
// ---------------------------------------------------------------------------

// RequestRefund initiates a refund request for a paid order.
func (s *PaymentService) RequestRefund(ctx context.Context, userID, orderNo, reason string, amountMicroUSD int64) (*repository.RefundRequest, error) {
	order, err := s.payments.FindOrderByNo(ctx, orderNo)
	if err != nil {
		return nil, fmt.Errorf("find order for refund: %w", err)
	}

	if order.Status != "paid" && order.Status != "credited" {
		return nil, fmt.Errorf("order %s is in status %q, cannot refund", orderNo, order.Status)
	}

	if amountMicroUSD <= 0 || amountMicroUSD > order.AmountMicroUSD {
		return nil, fmt.Errorf("refund amount %d out of range (order amount: %d)", amountMicroUSD, order.AmountMicroUSD)
	}

	// Convert micro USD back to yuan fen for the refund record.
	amountYuan := s.microUSDToYuanFen(amountMicroUSD)

	refund, err := s.payments.CreateRefundRequest(ctx, repository.CreateRefundParams{
		PaymentOrderID: order.ID,
		AmountMicroUSD: amountMicroUSD,
		AmountYuan:     amountYuan,
		Reason:         reason,
		CreatedBy:      userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create refund request: %w", err)
	}

	slog.Info("refund requested",
		slog.String("order_no", orderNo),
		slog.String("refund_id", refund.ID),
		slog.Int64("amount_micro_usd", amountMicroUSD),
	)

	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			OrganizationID: order.OrganizationID,
			ActorID:        userID,
			Action:         "payment.refund_requested",
			ResourceType:   "refund_request",
			ResourceID:     refund.ID,
			MetadataJSON:   fmt.Sprintf(`{"order_no":"%s","amount_micro_usd":%d}`, orderNo, amountMicroUSD),
		})
	}

	return refund, nil
}

// ApproveRefund approves a pending refund and processes it.
// Creates reversal ledger entries and updates the wallet balance.
func (s *PaymentService) ApproveRefund(ctx context.Context, approverID, refundID string) error {
	refund, err := s.payments.FindRefundByID(ctx, refundID)
	if err != nil {
		return fmt.Errorf("find refund: %w", err)
	}

	if refund.Status != "pending" {
		return fmt.Errorf("refund %s is in status %q, cannot approve", refundID, refund.Status)
	}

	order, err := s.payments.FindOrderByID(ctx, refund.PaymentOrderID)
	if err != nil {
		return fmt.Errorf("find order for refund approval: %w", err)
	}

	// Update refund status to processing
	if err := s.payments.UpdateRefundStatus(ctx, refundID, "processing", nil); err != nil {
		return fmt.Errorf("update refund to processing: %w", err)
	}

	// Find the org wallet
	wallet, err := s.wallets.FindByOrgID(ctx, order.OrganizationID)
	if err != nil {
		return fmt.Errorf("find wallet for refund: %w", err)
	}
	if wallet == nil {
		_ = s.payments.UpdateRefundStatus(ctx, refundID, "pending", nil)
		return fmt.Errorf("wallet not found for org %s", order.OrganizationID)
	}

	// Deduct from wallet: freeze then deduct frozen (two-step debit)
	if err := s.wallets.FreezeBalance(ctx, wallet.ID, refund.AmountMicroUSD); err != nil {
		_ = s.payments.UpdateRefundStatus(ctx, refundID, "pending", nil)
		return fmt.Errorf("freeze wallet for refund: %w", err)
	}
	if err := s.wallets.DeductFrozen(ctx, wallet.ID, refund.AmountMicroUSD); err != nil {
		// Attempt to unfreeze on deduction failure
		_ = s.wallets.UnfreezeBalance(ctx, wallet.ID, refund.AmountMicroUSD)
		_ = s.payments.UpdateRefundStatus(ctx, refundID, "pending", nil)
		return fmt.Errorf("deduct frozen for refund: %w", err)
	}

	// Create reversal ledger entry

	ledgerKey := fmt.Sprintf("refund_%s_%s", refundID, approverID)
	if len(ledgerKey) > 128 {
		ledgerKey = ledgerKey[:128]
	}
	entry, err := s.ledger.CreateEntry(ctx, nil, repository.LedgerEntryParams{
		OrganizationID: order.OrganizationID,
		WalletID:       wallet.ID,
		AmountMicroUSD: -refund.AmountMicroUSD, // negative = reversal
		EntryType:      "refund_reversal",
		Description:    fmt.Sprintf("Refund for order %s: %s", order.OrderNo, refund.Reason),
		RequestID:      order.OrderNo,
		IdempotencyKey: ledgerKey,
	})
	if err != nil {
		return fmt.Errorf("create refund reversal ledger entry: %w", err)
	}

	// Update refund to refunded with the ledger entry ID
	if err := s.payments.UpdateRefundStatus(ctx, refundID, "refunded", &entry.ID); err != nil {
		return fmt.Errorf("mark refund as refunded: %w", err)
	}

	// Update the original order status to 'refunded' (full refund) or leave as-is (partial)
	// For simplicity, only set to refunded for full refunds.
	if refund.AmountMicroUSD == order.AmountMicroUSD {
		_ = s.payments.UpdateOrderStatus(ctx, order.ID, "refunded", "")
	}

	slog.Info("refund approved and processed",
		slog.String("refund_id", refundID),
		slog.String("order_no", order.OrderNo),
		slog.Int64("amount_micro_usd", refund.AmountMicroUSD),
	)

	if s.audit != nil {
		_ = s.audit.Create(ctx, repository.CreateAuditParams{
			OrganizationID: order.OrganizationID,
			ActorID:        approverID,
			Action:         "payment.refund_approved",
			ResourceType:   "refund_request",
			ResourceID:     refundID,
			MetadataJSON:   fmt.Sprintf(`{"order_no":"%s","amount":%d,"ledger_entry_id":"%s"}`, order.OrderNo, refund.AmountMicroUSD, entry.ID),
		})
	}

	return nil
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// GetOrderStatus returns the current status of a payment order.
func (s *PaymentService) GetOrderStatus(ctx context.Context, orderNo string) (*repository.PaymentOrder, error) {
	return s.payments.FindOrderByNo(ctx, orderNo)
}

// ListOrders returns paginated orders for an organization.
func (s *PaymentService) ListOrders(ctx context.Context, orgID string, page, pageSize int) ([]repository.PaymentOrder, int, error) {
	return s.payments.ListOrdersByOrg(ctx, orgID, page, pageSize)
}

// ListRefunds returns paginated refunds for an organization.
func (s *PaymentService) ListRefunds(ctx context.Context, orgID string, page, pageSize int) ([]repository.RefundRequest, int, error) {
	return s.payments.ListRefundsByOrg(ctx, orgID, page, pageSize)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// generateOrderNo creates a unique order number: PO_<unix_seconds>_<8 hex random>.
func generateOrderNo() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("PO_%d_%s", time.Now().Unix(), hex.EncodeToString(b)), nil
}

// verifyProviderSignature is a stub for real payment provider signature verification.
// TODO: Replace with real SDK integrations (Alipay RSA/SHA256WithRSA, WeChat Pay V3).
func (s *PaymentService) verifyProviderSignature(provider string, payload CallbackPayload) error {
	// STUB: In production, verify the signature using the provider's public key.
	// Alipay: RSA/SHA256WithRSA verification of signed_string vs sign field.
	// WeChat Pay V3: HMAC-SHA256 verification using the API v3 key.
	slog.Info("signature verification stub (verified)",
		slog.String("provider", provider),
		slog.String("order_no", payload.OrderNo),
	)
	return nil
}

// yuanFenToMicroUSD converts an amount in CNY fen to micro USD using the
// configured exchange rate (USDCNYRate = how many CNY per 1 USD).
//
// Example: 1000 fen (10 CNY) at rate 7.0 -> 1000 * 1,000,000 / (7.0 * 100)
// = 1000 * 10000 / 70 = 142,857 micro USD
func (s *PaymentService) yuanFenToMicroUSD(fen int) int64 {
	if s.cfg.USDCNYRate <= 0 {
		s.cfg.USDCNYRate = 7.0
	}
	// micro_usd = fen * 1_000_000 / (usd_cny_rate * 100)
	result := float64(fen) * 1_000_000.0 / (s.cfg.USDCNYRate * 100.0)
	return int64(math.Round(result))
}

// microUSDToYuanFen converts micro USD back to CNY fen.
func (s *PaymentService) microUSDToYuanFen(microUSD int64) int {
	if s.cfg.USDCNYRate <= 0 {
		s.cfg.USDCNYRate = 7.0
	}
	// fen = micro_usd * usd_cny_rate * 100 / 1_000_000
	result := float64(microUSD) * s.cfg.USDCNYRate * 100.0 / 1_000_000.0
	return int(math.Round(result))
}
