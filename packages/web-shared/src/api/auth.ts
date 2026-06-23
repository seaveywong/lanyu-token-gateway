import { apiClient } from './client';
import { clearTokens } from '../auth/tokens';

export interface User {
  id: string;
  email: string;
  display_name: string;
  mfa_enabled: boolean;
}

export interface LoginRequest {
  email: string;
  password: string;
  totp_code?: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  display_name: string;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: User;
}

export function login(data: LoginRequest): Promise<AuthResponse> {
  return apiClient<AuthResponse>('/portal-api/auth/login', { method: 'POST', body: data });
}

export function register(data: RegisterRequest): Promise<AuthResponse> {
  return apiClient<AuthResponse>('/portal-api/auth/register', { method: 'POST', body: data });
}

export function getMe(): Promise<User> {
  return apiClient<User>('/portal-api/auth/me');
}

export async function logout(): Promise<void> {
  try {
    await apiClient('/portal-api/auth/logout', { method: 'POST' });
  } catch {
    // Ignore errors on logout
  }
  clearTokens();
}

export function setupMFA(): Promise<{ secret: string; qr_code_url: string }> {
  return apiClient('/portal-api/auth/mfa/setup', { method: 'POST' });
}

export function enableMFA(code: string): Promise<{ recovery_codes: string[] }> {
  return apiClient('/portal-api/auth/mfa/enable', { method: 'POST', body: { code } });
}
