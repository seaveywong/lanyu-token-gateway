import { apiClient } from './client';

export interface Channel {
  id: string;
  name: string;
  description?: string;
  status: string;
  created_at: string;
}

export interface ChannelWithSources extends Channel {
  sources: ChannelSourceItem[];
}

export interface ChannelSourceItem {
  id: string;
  name: string;
  source_type: string;
  priority: number;
  weight: number;
  health_state: string;
}

export interface ChannelListResponse {
  data: Channel[];
  total: number;
}

export interface RouteRule {
  id: string;
  org_id?: string;
  project_id?: string;
  model_pattern: string;
  channel_id: string;
  channel_name?: string;
  priority: number;
  weight: number;
  status: string;
  created_at: string;
}

export interface RouteRuleListResponse {
  data: RouteRule[];
  total: number;
}

export interface ModelMapping {
  id: string;
  external_model: string;
  native_model: string;
  cost_multiplier: number;
  status: string;
  created_at: string;
}

export interface ModelMappingListResponse {
  data: ModelMapping[];
  total: number;
}

export async function listChannels() {
  return apiClient<ChannelListResponse>('/admin-api/channels');
}

export async function getChannel(id: string) {
  return apiClient<ChannelWithSources>(`/admin-api/channels/${id}`);
}

export async function createChannel(data: { name: string; description?: string }) {
  return apiClient<Channel>('/admin-api/channels', {
    method: 'POST',
    body: data,
  });
}

export async function addSourceToChannel(channelId: string, sourceId: string) {
  return apiClient<{ success: boolean }>(`/admin-api/channels/${channelId}/sources`, {
    method: 'POST',
    body: { source_id: sourceId },
  });
}

export async function removeSourceFromChannel(channelId: string, sourceId: string) {
  return apiClient<{ success: boolean }>(`/admin-api/channels/${channelId}/sources/${sourceId}`, {
    method: 'DELETE',
  });
}

export async function listRouteRules(params?: { page?: number; page_size?: number }) {
  const query = new URLSearchParams();
  if (params?.page !== undefined) query.set('page', String(params.page));
  if (params?.page_size !== undefined) query.set('page_size', String(params.page_size));
  const qs = query.toString();
  return apiClient<RouteRuleListResponse>(
    `/admin-api/route-rules${qs ? '?' + qs : ''}`,
  );
}

export async function listModelMappings(params?: { page?: number; page_size?: number }) {
  const query = new URLSearchParams();
  if (params?.page !== undefined) query.set('page', String(params.page));
  if (params?.page_size !== undefined) query.set('page_size', String(params.page_size));
  const qs = query.toString();
  return apiClient<ModelMappingListResponse>(
    `/admin-api/model-mappings${qs ? '?' + qs : ''}`,
  );
}

export async function createModelMapping(data: {
  external_model: string;
  native_model: string;
  cost_multiplier: number;
}) {
  return apiClient<ModelMapping>('/admin-api/model-mappings', {
    method: 'POST',
    body: data,
  });
}

export async function createRouteRule(data: {
  org_id?: string;
  project_id?: string;
  model_pattern: string;
  channel_id: string;
  priority: number;
  weight: number;
}) {
  return apiClient<RouteRule>('/admin-api/route-rules', {
    method: 'POST',
    body: data,
  });
}
