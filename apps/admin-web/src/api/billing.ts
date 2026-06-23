import { apiClient } from './client';

// ── Types ──────────────────────────────────────────────

export interface PricingRule {
  id: string;
  model_name: string;
  input_price_micro_usd: number;
  output_price_micro_usd: number;
  cached_price_micro_usd: number;
  image_price_micro_usd: number;
  audio_price_micro_usd: number;
}

export interface PricingVersion {
  id: string;
  name: string;
  is_active: boolean;
  effective_at: string;
  created_at?: string;
  rules?: PricingRule[];
}

export interface PricingVersionListResponse {
  data: PricingVersion[];
  total: number;
}

export interface WalletBalance {
  balance_micro_usd: number;
  frozen_micro_usd: number;
  available_micro_usd: number;
}

export interface LedgerEntry {
  id: string;
  wallet_id: string;
  amount_micro_usd: number;
  entry_type: string;
  description?: string;
  request_id?: string;
  created_at: string;
}

export interface LedgerListResponse {
  data: LedgerEntry[];
  total: number;
}

export interface PaymentOrder {
  id: string;
  order_no: string;
  org_id?: string;
  payment_method: string;
  amount_yuan: number;
  amount_micro_usd: number;
  status: string;
  created_at: string;
  completed_at?: string;
}

export interface PaymentOrderListResponse {
  data: PaymentOrder[];
  total: number;
}

export interface ReconciliationReport {
  id: string;
  date: string;
  total_expected_micro_usd: number;
  total_actual_micro_usd: number;
  discrepancy_count: number;
  status: string;
  created_at: string;
}

export interface ReconciliationDifference {
  id: string;
  type: string;
  expected_micro_usd: number;
  actual_micro_usd: number;
  difference_micro_usd: number;
  status: string;
  description?: string;
}

export interface ReconciliationDetail {
  report: ReconciliationReport;
  differences: ReconciliationDifference[];
}

// ── Pricing ────────────────────────────────────────────

export async function listPricingVersions(params?: {
  is_active?: boolean;
}): Promise<PricingVersionListResponse> {
  const query = new URLSearchParams();
  if (params?.is_active !== undefined) query.set('is_active', String(params.is_active));
  const qs = query.toString();
  return apiClient<PricingVersionListResponse>(
    `/admin-api/billing/pricing-versions${qs ? '?' + qs : ''}`,
  );
}

export async function getPricingVersion(
  id: string,
): Promise<PricingVersion> {
  return apiClient<PricingVersion>(`/admin-api/billing/pricing-versions/${id}`);
}

export async function createPricingVersion(data: {
  name: string;
  rules: {
    model_name: string;
    input_price_micro_usd: number;
    output_price_micro_usd: number;
    cached_price_micro_usd?: number;
    image_price_micro_usd?: number;
    audio_price_micro_usd?: number;
  }[];
}): Promise<PricingVersion> {
  return apiClient<PricingVersion>('/admin-api/billing/pricing-versions', {
    method: 'POST',
    body: data,
  });
}

// ── Wallet ─────────────────────────────────────────────

export async function getWalletBalance(
  orgId: string,
): Promise<WalletBalance> {
  return apiClient<WalletBalance>(`/admin-api/billing/wallets/${orgId}/balance`);
}

export async function listLedgerEntries(
  orgId: string,
  params?: { page?: number; page_size?: number },
): Promise<LedgerListResponse> {
  const query = new URLSearchParams();
  if (params?.page !== undefined) query.set('page', String(params.page));
  if (params?.page_size !== undefined) query.set('page_size', String(params.page_size));
  const qs = query.toString();
  return apiClient<LedgerListResponse>(
    `/admin-api/billing/wallets/${orgId}/ledger${qs ? '?' + qs : ''}`,
  );
}

// ── Payment Orders ─────────────────────────────────────

export async function createPaymentOrder(data: {
  org_id: string;
  payment_method: string;
  amount_yuan: number;
}): Promise<PaymentOrder> {
  return apiClient<PaymentOrder>('/admin-api/billing/payment-orders', {
    method: 'POST',
    body: data,
  });
}

export async function listPaymentOrders(params?: {
  org_id?: string;
  status?: string;
  page?: number;
  page_size?: number;
}): Promise<PaymentOrderListResponse> {
  const query = new URLSearchParams();
  if (params?.org_id) query.set('org_id', params.org_id);
  if (params?.status) query.set('status', params.status);
  if (params?.page !== undefined) query.set('page', String(params.page));
  if (params?.page_size !== undefined) query.set('page_size', String(params.page_size));
  const qs = query.toString();
  return apiClient<PaymentOrderListResponse>(
    `/admin-api/billing/payment-orders${qs ? '?' + qs : ''}`,
  );
}

export async function markPaymentOrderComplete(
  orderId: string,
): Promise<PaymentOrder> {
  return apiClient<PaymentOrder>(
    `/admin-api/billing/payment-orders/${orderId}/complete`,
    { method: 'POST' },
  );
}

// ── Reconciliation ─────────────────────────────────────

export async function getReconciliationReport(params: {
  date: string;
}): Promise<ReconciliationDetail> {
  return apiClient<ReconciliationDetail>(
    `/admin-api/billing/reconciliation?date=${encodeURIComponent(params.date)}`,
  );
}
