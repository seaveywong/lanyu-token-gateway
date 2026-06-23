import { getAccessToken, clearTokens, tryRefreshToken } from '../auth/tokens';

const API_BASE = '';

interface RequestOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
}

export async function apiClient<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...options.headers,
  };

  const token = getAccessToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, {
    method: options.method || 'GET',
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (!res.ok) {
    if (res.status === 401) {
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        headers['Authorization'] = `Bearer ${getAccessToken()}`;
        const retryRes = await fetch(`${API_BASE}${path}`, {
          method: options.method || 'GET',
          headers,
          body: options.body ? JSON.stringify(options.body) : undefined,
        });
        if (retryRes.ok) return retryRes.json();
      }
      clearTokens();
      window.location.href = '/login';
      throw new Error('Unauthorized');
    }
    const err = await res.json();
    throw new Error(err.error?.message || 'Request failed');
  }

  return res.json();
}
