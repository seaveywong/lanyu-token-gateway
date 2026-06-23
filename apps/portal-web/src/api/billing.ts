import { apiClient } from './client';

// ── Types ──────────────────────────────────────────────

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
  payment_method: string;
  amount_yuan: number;
  amount_micro_usd: number;
  status: string;
  created_at: string;
  completed_at?: string;
  pay_url?: string;
  qr_code_url?: string;
}

export interface PaymentOrderListResponse {
  data: PaymentOrder[];
  total: number;
}

// ── APIs ───────────────────────────────────────────────

export async function getWalletBalance(): Promise<WalletBalance> {
  return apiClient<WalletBalance>('/portal-api/payments/wallet/balance');
}

export async function listLedgerEntries(params?: {
  page?: number;
  page_size?: number;
}): Promise<LedgerListResponse> {
  const query = new URLSearchParams();
  if (params?.page !== undefined) query.set('page', String(params.page));
  if (params?.page_size !== undefined) query.set('page_size', String(params.page_size));
  const qs = query.toString();
  return apiClient<LedgerListResponse>(
    `/portal-api/payments/ledger${qs ? '?' + qs : ''}`,
  );
}

export async function createPaymentOrder(data: {
  payment_method: string;
  amount_yuan: number;
}): Promise<PaymentOrder> {
  return apiClient<PaymentOrder>('/portal-api/payments/orders', {
    method: 'POST',
    body: data,
  });
}

export async function listPaymentOrders(params?: {
  status?: string;
  page?: number;
  page_size?: number;
}): Promise<PaymentOrderListResponse> {
  const query = new URLSearchParams();
  if (params?.status) query.set('status', params.status);
  if (params?.page !== undefined) query.set('page', String(params.page));
  if (params?.page_size !== undefined) query.set('page_size', String(params.page_size));
  const qs = query.toString();
  return apiClient<PaymentOrderListResponse>(
    `/portal-api/payments/orders${qs ? '?' + qs : ''}`,
  );
}

export async function getPaymentOrder(orderId: string): Promise<PaymentOrder> {
  return apiClient<PaymentOrder>(`/portal-api/payments/orders/${orderId}`);
}
