import { createContext, useContext, useState, useEffect, type ReactNode } from 'react';
import type { User } from '../api/auth';
import { login as loginApi, register as registerApi, logout as logoutApi, getMe } from '../api/auth';
import { getAccessToken, setTokens } from './tokens';

interface AuthState {
  user: User | null;
  loading: boolean;
  error: string | null;
  login: (email: string, password: string, totpCode?: string) => Promise<void>;
  register: (email: string, password: string, displayName: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (getAccessToken()) {
      getMe()
        .then(setUser)
        .catch(() => {
          // Token invalid, ignore
        })
        .finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, []);

  const loginFn = async (email: string, password: string, totpCode?: string) => {
    setError(null);
    try {
      const result = await loginApi({ email, password, totp_code: totpCode });
      setTokens(result.access_token, result.refresh_token);
      setUser(result.user);
    } catch (e) {
      setError(e instanceof Error ? e.message : '登录失败');
      throw e;
    }
  };

  const registerFn = async (email: string, password: string, displayName: string) => {
    setError(null);
    try {
      const result = await registerApi({ email, password, display_name: displayName });
      setTokens(result.access_token, result.refresh_token);
      setUser(result.user);
    } catch (e) {
      setError(e instanceof Error ? e.message : '注册失败');
      throw e;
    }
  };

  const logoutFn = async () => {
    await logoutApi();
    setUser(null);
  };

  const refreshFn = async () => {
    try {
      setUser(await getMe());
    } catch {
      // User session expired
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        error,
        login: loginFn,
        register: registerFn,
        logout: logoutFn,
        refreshUser: refreshFn,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
