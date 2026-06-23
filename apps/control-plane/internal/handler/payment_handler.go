package handler

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/service"
)

// ---------------------------------------------------------------------------
// PaymentHandler handles payment order, webhook callback, refund, and
// reconciliation HTTP endpoints for both the portal and admin APIs.
// ---------------------------------------------------------------------------

// PaymentHandler dispatches to the PaymentService and ReconciliationService.
type PaymentHandler struct {
	payments *service.PaymentService
	recon    *service.ReconciliationService
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(
	paymentSvc *service.PaymentService,
	reconSvc *service.ReconciliationService,
) *PaymentHandler {
	return &PaymentHandler{
		payments: paymentSvc,
		recon:    reconSvc,
	}
}

// ---------------------------------------------------------------------------
// Portal API — Payment Orders
// ---------------------------------------------------------------------------

// CreateOrder handles POST /portal-api/payments/orders.
// Creates a payment order for the authenticated user's organization.
func (h *PaymentHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "missing organization context", requestID(r))
		return
	}

	var req struct {
		PaymentMethod string `json:"payment_method"`
		AmountYuan    int    `json:"amount_yuan"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.PaymentMethod == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "payment_method is required", requestID(r))
		return
	}
	if req.AmountYuan <= 0 {
		respondError(w, http.StatusBadRequest, "invalid_request", "amount_yuan must be positive", requestID(r))
		return
	}

	order, err := h.payments.CreateOrder(r.Context(), userID, orgID, req.PaymentMethod, req.AmountYuan)
	if err != nil {
		slog.Error("create payment order failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, order)
}

// ListOrders handles GET /portal-api/payments/orders.
// Returns paginated payment orders for the authenticated user's organization.
func (h *PaymentHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "missing organization context", requestID(r))
		return
	}

	page, pageSize := getPageParams(r)
	orders, total, err := h.payments.ListOrders(r.Context(), orgID, page, pageSize)
	if err != nil {
		slog.Error("list payment orders failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      orders,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// GetOrder handles GET /portal-api/payments/orders/{orderNo}.
// Returns the status of a specific payment order.
func (h *PaymentHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderNo := chi.URLParam(r, "orderNo")
	if orderNo == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "orderNo is required", requestID(r))
		return
	}

	order, err := h.payments.GetOrderStatus(r.Context(), orderNo)
	if err != nil {
		slog.Error("get payment order failed", slog.String("error", err.Error()), slog.String("order_no", orderNo))
		respondError(w, http.StatusNotFound, "not_found", "payment order not found", requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, order)
}

// ---------------------------------------------------------------------------
// Public Callback API — Alipay & WeChat Pay Webhooks
// ---------------------------------------------------------------------------

// AlipayCallback handles POST /api/payments/callback/alipay.
// Public endpoint — no authentication required, but signature is verified.
func (h *PaymentHandler) AlipayCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20)) // 1 MB max
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "failed to read request body: "+err.Error(), requestID(r))
		return
	}
	rawBody := string(body)

	// Parse Alipay callback — stub: TODO real SDK integration.
	payload := parseAlipayCallbackBody(body)
	payload.RawBody = rawBody

	if err := h.payments.HandleCallback(r.Context(), "alipay", "payment.success", payload); err != nil {
		slog.Error("alipay callback processing failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", "callback processing failed", requestID(r))
		return
	}

	// Alipay expects "success" response; otherwise it will retry.
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("success"))
}

// WeChatCallback handles POST /api/payments/callback/wechat.
// Public endpoint — no authentication required, but signature is verified.
func (h *PaymentHandler) WeChatCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "failed to read request body: "+err.Error(), requestID(r))
		return
	}
	rawBody := string(body)

	// Parse WeChat callback — stub: TODO real SDK integration.
	payload := parseWeChatCallbackBody(body)
	payload.RawBody = rawBody

	if err := h.payments.HandleCallback(r.Context(), "wechat_pay", "payment.success", payload); err != nil {
		slog.Error("wechat callback processing failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", "callback processing failed", requestID(r))
		return
	}

	// WeChat expects a JSON response with "code": "SUCCESS".
	respondJSON(w, http.StatusOK, map[string]string{"code": "SUCCESS", "message": "OK"})
}

// ---------------------------------------------------------------------------
// Portal API — Refunds
// ---------------------------------------------------------------------------

// RequestRefund handles POST /portal-api/payments/refunds.
// Creates a refund request for a paid order.
func (h *PaymentHandler) RequestRefund(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		OrderNo        string `json:"order_no"`
		Reason         string `json:"reason"`
		AmountMicroUSD int64  `json:"amount_micro_usd"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.OrderNo == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "order_no is required", requestID(r))
		return
	}
	if req.Reason == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "reason is required", requestID(r))
		return
	}
	if req.AmountMicroUSD <= 0 {
		respondError(w, http.StatusBadRequest, "invalid_request", "amount_micro_usd must be positive", requestID(r))
		return
	}

	refund, err := h.payments.RequestRefund(r.Context(), userID, req.OrderNo, req.Reason, req.AmountMicroUSD)
	if err != nil {
		slog.Error("request refund failed", slog.String("error", err.Error()))
		respondError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, refund)
}

// ListRefunds handles GET /portal-api/payments/refunds.
func (h *PaymentHandler) ListRefunds(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "missing organization context", requestID(r))
		return
	}

	page, pageSize := getPageParams(r)
	refunds, total, err := h.payments.ListRefunds(r.Context(), orgID, page, pageSize)
	if err != nil {
		slog.Error("list refunds failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      refunds,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// ---------------------------------------------------------------------------
// Admin API — Refund Approval
// ---------------------------------------------------------------------------

// ApproveRefund handles POST /admin-api/payments/refunds/{id}/approve.
// Platform admins approve a pending refund.
func (h *PaymentHandler) ApproveRefund(w http.ResponseWriter, r *http.Request) {
	approverID := middleware.UserIDFromContext(r.Context())
	refundID := chi.URLParam(r, "id")

	if err := h.payments.ApproveRefund(r.Context(), approverID, refundID); err != nil {
		slog.Error("approve refund failed", slog.String("error", err.Error()))
		respondError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "refunded"})
}

// ---------------------------------------------------------------------------
// Admin API — Reconciliation
// ---------------------------------------------------------------------------

// TriggerReconciliation handles POST /admin-api/reconciliation/run.
// Manually triggers a reconciliation run for today (or a given date).
func (h *PaymentHandler) TriggerReconciliation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date string `json:"date"` // optional, format: "2006-01-02", defaults to today
	}
	// Empty body is acceptable — defaults to today.
	if r.Body != nil && r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
			return
		}
	}

	var date time.Time
	if req.Date == "" {
		date = time.Now()
	} else {
		var err error
		date, err = time.Parse("2006-01-02", req.Date)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_request", "invalid date format, use YYYY-MM-DD", requestID(r))
			return
		}
	}
	// Truncate to day boundary
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	run, err := h.recon.RunDailyReconciliation(r.Context(), date)
	if err != nil {
		slog.Error("reconciliation run failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, run)
}

// GetReconciliationReport handles GET /admin-api/reconciliation/report/{date}.
// Returns the daily reconciliation report for a given date.
func (h *PaymentHandler) GetReconciliationReport(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid date format, use YYYY-MM-DD", requestID(r))
		return
	}

	report, err := h.recon.GetDailyReport(r.Context(), date)
	if err != nil {
		slog.Error("get reconciliation report failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// ResolveDiscrepancy handles POST /admin-api/reconciliation/items/{id}/resolve.
func (h *PaymentHandler) ResolveDiscrepancy(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")

	var req struct {
		Resolution       string `json:"resolution"`
		Notes            string `json:"notes"`
		CorrectionAmount int64  `json:"correction_amount"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Resolution == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "resolution is required", requestID(r))
		return
	}

	if err := h.recon.ResolveDiscrepancy(r.Context(), itemID, req.Resolution, req.Notes, req.CorrectionAmount); err != nil {
		slog.Error("resolve discrepancy failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

// ---------------------------------------------------------------------------
// Callback parsing helpers (stubs — TODO: real provider SDK integration)
// ---------------------------------------------------------------------------

// parseAlipayCallbackBody parses an Alipay callback body (stub).
// In production, this would use Alipay's SDK to verify the RSA signature and
// parse the signed fields.
//
// Expected Alipay form fields: out_trade_no, trade_no, total_amount, sign, notify_id.
func parseAlipayCallbackBody(body []byte) service.CallbackPayload {
	// STUB: Return empty payload. In production:
	//  1. Parse form-encoded body
	//  2. Verify RSA/SHA256WithRSA signature using Alipay public key
	//  3. Map out_trade_no→OrderNo, trade_no→ProviderTradeNo,
	//     total_amount→AmountYuan (yuan * 100 → fen), notify_id→IdempotencyKey
	return service.CallbackPayload{
		Extra: map[string]string{"raw": string(body)},
	}
}

// parseWeChatCallbackBody parses a WeChat Pay V3 callback body (stub).
// WeChat Pay V3 sends an encrypted JSON callback:
//
//	{"id":"...","create_time":"...","resource_type":"encrypt-resource",
//	 "event_type":"TRANSACTION.SUCCESS","resource":{"algorithm":"AEAD_AES_256_GCM",
//	 "ciphertext":"...","nonce":"..."}}
func parseWeChatCallbackBody(body []byte) service.CallbackPayload {
	// STUB: Return empty payload. In production:
	//  1. Parse JSON envelope
	//  2. Decrypt resource.ciphertext using AEAD_AES_256_GCM with APIv3 key
	//  3. Parse decrypted transaction data
	//  4. Map out_trade_no→OrderNo, transaction_id→ProviderTradeNo,
	//     amount.total→AmountYuan, id→IdempotencyKey
	return service.CallbackPayload{
		Extra: map[string]string{"raw": string(body)},
	}
}

