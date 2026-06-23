import { apiClient } from './client';

export interface AccountSource {
  id: string;
  name: string;
  source_type: 'official_api_key' | 'official_oauth' | 'upstream_api' | 'subscription_pool';
  provider_id?: string;
  priority: number;
  weight: number;
  max_concurrency: number;
  daily_budget_micro_usd: number;
  status: string;
  health_state: string;
  subscription_accounts_count: number;
  last_validated_at?: string;
  created_at: string;
}

export interface AccountSourceListResponse {
  data: AccountSource[];
  total: number;
  page: number;
  page_size: number;
}

export interface CreateAccountSourceParams {
  name: string;
  source_type: string;
  provider_id?: string;
  credential: string;
  priority: number;
  weight: number;
  max_concurrency?: number;
  daily_budget_micro_usd?: number;
}

export async function listAccountSources(params?: {
  page?: number;
  page_size?: number;
  source_type?: string;
}) {
  const query = new URLSearchParams();
  if (params?.page !== undefined) query.set('page', String(params.page));
  if (params?.page_size !== undefined) query.set('page_size', String(params.page_size));
  if (params?.source_type) query.set('source_type', params.source_type);
  const qs = query.toString();
  return apiClient<AccountSourceListResponse>(
    `/admin-api/account-sources${qs ? '?' + qs : ''}`,
  );
}

export async function getAccountSource(id: string) {
  return apiClient<AccountSource>(`/admin-api/account-sources/${id}`);
}

export async function createAccountSource(data: CreateAccountSourceParams) {
  return apiClient<AccountSource>('/admin-api/account-sources', {
    method: 'POST',
    body: data,
  });
}

export async function disableAccountSource(id: string) {
  return apiClient<{ success: boolean }>(`/admin-api/account-sources/${id}/disable`, {
    method: 'POST',
  });
}

export async function validateAccountSource(id: string) {
  return apiClient<{ success: boolean }>(`/admin-api/account-sources/${id}/validate`, {
    method: 'POST',
  });
}
