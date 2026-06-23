import { apiClient } from './client';

// ── Types ──────────────────────────────────────────────

export interface UsageByModel {
  model_name: string;
  request_count: number;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  cost_micro_usd: number;
}

export interface UsageSummary {
  total_requests: number;
  total_input_tokens: number;
  total_output_tokens: number;
  total_cost_micro_usd: number;
  by_model: UsageByModel[];
}

export interface UsageQueryParams {
  start_date?: string;
  end_date?: string;
  project_id?: string;
  group_by?: 'model' | 'day' | 'project';
}

export interface UsageByDay {
  date: string;
  request_count: number;
  input_tokens: number;
  output_tokens: number;
  cost_micro_usd: number;
}

export interface UsageTrend {
  data: UsageByDay[];
}

export interface DashboardSummary {
  balance: {
    available_micro_usd: number;
    frozen_micro_usd: number;
  };
  today: {
    requests: number;
    tokens: number;
    cost_micro_usd: number;
  };
  recent_errors: {
    count: number;
    rate: number;
  };
  projects: {
    id: string;
    name: string;
    daily_budget_micro_usd: number;
    monthly_budget_micro_usd: number;
    daily_used_micro_usd: number;
    monthly_used_micro_usd: number;
  }[];
}

// ── APIs ───────────────────────────────────────────────

export async function getUsageSummary(
  params?: UsageQueryParams,
): Promise<UsageSummary> {
  const query = new URLSearchParams();
  if (params?.start_date) query.set('start_date', params.start_date);
  if (params?.end_date) query.set('end_date', params.end_date);
  if (params?.project_id) query.set('project_id', params.project_id);
  if (params?.group_by) query.set('group_by', params.group_by);
  const qs = query.toString();
  return apiClient<UsageSummary>(
    `/portal-api/usage${qs ? '?' + qs : ''}`,
  );
}

export async function getUsageTrend(
  params?: UsageQueryParams,
): Promise<UsageTrend> {
  const query = new URLSearchParams();
  if (params?.start_date) query.set('start_date', params.start_date);
  if (params?.end_date) query.set('end_date', params.end_date);
  if (params?.project_id) query.set('project_id', params.project_id);
  query.set('group_by', 'day');
  const qs = query.toString();
  return apiClient<UsageTrend>(
    `/portal-api/usage/trend${qs ? '?' + qs : ''}`,
  );
}

export async function getDashboardSummary(): Promise<DashboardSummary> {
  return apiClient<DashboardSummary>('/portal-api/dashboard/summary');
}
