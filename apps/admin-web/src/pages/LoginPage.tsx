import { useState, type FormEvent } from 'react';
import { useAuth } from '@lanyu/web-shared/auth/AuthContext';
import styles from './LoginPage.module.css';

function LoginPage() {
  const { user, loading, error, login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);

  // If already logged in, redirect to home
  if (!loading && user) {
    window.location.href = '/';
    return null;
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setLocalError(null);
    try {
      await login(email, password);
      window.location.href = '/';
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : '登录失败，请重试');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <div className={styles.header}>
          <h1 className={styles.title}>蓝域 Token Gateway 管理后台</h1>
          <p className={styles.subtitle}>请登录以继续</p>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          {(localError || error) && (
            <div className={styles.error}>{(localError || error) ?? ''}</div>
          )}

          <div className={styles.field}>
            <label htmlFor="email" className={styles.label}>邮箱</label>
            <input
              id="email"
              type="email"
              className={styles.input}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="admin@example.com"
              required
              autoComplete="email"
              autoFocus
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="password" className={styles.label}>密码</label>
            <input
              id="password"
              type="password"
              className={styles.input}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="输入密码"
              required
              autoComplete="current-password"
            />
          </div>

          <button type="submit" className={styles.button} disabled={submitting}>
            {submitting ? '登录中...' : '登录'}
          </button>
        </form>

        <div className={styles.divider}>
          <span className={styles.dividerText}>或</span>
        </div>

        <button
          className={styles.ssoButton}
          onClick={() => { window.location.href = '/portal-api/auth/oidc/login'; }}
        >
          使用企业账号登录
        </button>

        <p className={styles.footer}>仅限授权管理员使用</p>
      </div>
    </div>
  );
}

export default LoginPage;
