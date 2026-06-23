package handler

import (
	"context"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// ---------------------------------------------------------------------------
// Service interfaces — defined in the handler package to decouple handlers
// from concrete service implementations. These interfaces mirror the actual
// service method signatures in apps/control-plane/internal/service/.
// ---------------------------------------------------------------------------

// UserService defines the user-facing operations required by auth and user
// HTTP handlers.
type UserService interface {
	Register(ctx context.Context, email, password, displayName string) (*repository.User, error)
	Login(ctx context.Context, email, password string) (*repository.User, error)
	SetupMFA(ctx context.Context, userID string) (secret, qrURL string, err error)
	EnableMFA(ctx context.Context, userID, code string) (recoveryCodes []string, err error)
	VerifyMFA(ctx context.Context, userID, code string) (bool, error)
	// TODO: GetByID and ListAll will be added to the service layer in a follow-up.
	GetByID(ctx context.Context, id string) (*repository.User, error)
	ListAll(ctx context.Context, page, pageSize int) ([]repository.User, int, error)
}

// OrgService defines organization-level operations.
type OrgService interface {
	Create(ctx context.Context, userID, name string) (*repository.Organization, error)
	GetByID(ctx context.Context, id string) (*repository.Organization, error)
	ListByUser(ctx context.Context, userID string) ([]repository.Organization, error)
	// TODO: ListAll will be added to the service layer in a follow-up.
	ListAll(ctx context.Context, page, pageSize int) ([]repository.Organization, int, error)
}

// ProjectService defines project-level operations.
type ProjectService interface {
	Create(ctx context.Context, orgID, userID, name, description string) (*repository.Project, error)
	GetByID(ctx context.Context, id string) (*repository.Project, error)
	ListByOrg(ctx context.Context, orgID, userID string) ([]repository.Project, error)
}

// APIKeyService defines API key lifecycle operations.
type APIKeyService interface {
	Create(ctx context.Context, projectID, userID, name, env string) (*APIKeyCreateResult, error)
	ListByProject(ctx context.Context, projectID, userID string) ([]repository.APIKey, error)
	Revoke(ctx context.Context, keyID, userID string) error
}

// APIKeyCreateResult wraps the plaintext key returned at creation time along
// with the stored API key metadata. The raw key is only available once.
type APIKeyCreateResult struct {
	ID        string  `json:"id"`
	RawKey    string  `json:"key"`
	Prefix    string  `json:"key_prefix"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

// AuditService defines audit log query operations.
type AuditService interface {
	ListByOrg(ctx context.Context, orgID string, page, pageSize int) ([]repository.AuditEntry, int, error)
}

// AccountSourceService defines account source management operations.
type AccountSourceService interface {
	List(ctx context.Context, page, pageSize int) ([]AccountSourceResponse, int, error)
	Create(ctx context.Context, name, sourceType, providerID string, credentialCiphertext []byte, createdBy string) (*AccountSourceResponse, error)
}

// AccountSourceResponse is a single account source entry.
type AccountSourceResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
	Status     string `json:"status"`
}

// ---------------------------------------------------------------------------
// Payment & Reconciliation service interfaces
// ---------------------------------------------------------------------------

// PaymentService defines the payment operations required by HTTP handlers.
type PaymentService interface {
	CreateOrder(ctx context.Context, userID, orgID, paymentMethod string, amountYuan int) (*repository.PaymentOrder, error)
	HandleCallback(ctx context.Context, provider, eventType string, payload interface{}) error
	RequestRefund(ctx context.Context, userID, orderNo, reason string, amountMicroUSD int64) (*repository.RefundRequest, error)
	ApproveRefund(ctx context.Context, approverID, refundID string) error
	GetOrderStatus(ctx context.Context, orderNo string) (*repository.PaymentOrder, error)
	ListOrders(ctx context.Context, orgID string, page, pageSize int) ([]repository.PaymentOrder, int, error)
	ListRefunds(ctx context.Context, orgID string, page, pageSize int) ([]repository.RefundRequest, int, error)
}

// ReconciliationService defines the reconciliation operations required by HTTP handlers.
type ReconciliationService interface {
	RunDailyReconciliation(ctx context.Context, date time.Time) (*repository.ReconciliationRun, error)
	GetDiscrepancies(ctx context.Context, runID string) ([]repository.ReconciliationItem, error)
	ResolveDiscrepancy(ctx context.Context, itemID, resolution, notes string, correctionAmount int64) error
	GetDailyReport(ctx context.Context, date time.Time) (interface{}, error)
}
