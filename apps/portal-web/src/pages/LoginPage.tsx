import { useState, type FormEvent } from 'react';
import { useAuth } from '@lanyu/web-shared/auth/AuthContext';
import styles from './LoginPage.module.css';

function LoginPage() {
  const { user, loading, error, login, register } = useAuth();
  const [activeTab, setActiveTab] = useState<'login' | 'register'>('login');

  // Login form state
  const [loginEmail, setLoginEmail] = useState('');
  const [loginPassword, setLoginPassword] = useState('');

  // Register form state
  const [regEmail, setRegEmail] = useState('');
  const [regDisplayName, setRegDisplayName] = useState('');
  const [regPassword, setRegPassword] = useState('');
  const [regConfirmPassword, setRegConfirmPassword] = useState('');

  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);

  if (!loading && user) {
    window.location.href = '/dashboard';
    return null;
  }

  const handleLogin = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setLocalError(null);
    try {
      await login(loginEmail, loginPassword);
      window.location.href = '/dashboard';
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : '登录失败，请重试');
    } finally {
      setSubmitting(false);
    }
  };

  const handleRegister = async (e: FormEvent) => {
    e.preventDefault();
    setLocalError(null);

    if (regPassword !== regConfirmPassword) {
      setLocalError('两次输入的密码不一致');
      return;
    }

    if (regPassword.length < 8) {
      setLocalError('密码长度至少为 8 个字符');
      return;
    }

    setSubmitting(true);
    try {
      await register(regEmail, regPassword, regDisplayName);
      window.location.href = '/dashboard';
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : '注册失败，请重试');
    } finally {
      setSubmitting(false);
    }
  };

  const switchTab = (tab: 'login' | 'register') => {
    setActiveTab(tab);
    setLocalError(null);
  };

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <div className={styles.header}>
          <h1 className={styles.title}>蓝域 API 控制台</h1>
          <p className={styles.subtitle}>
            {activeTab === 'login' ? '登录您的账户以继续' : '创建您的账户'}
          </p>
        </div>

        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${activeTab === 'login' ? styles.tabActive : ''}`}
            onClick={() => switchTab('login')}
          >
            登录
          </button>
          <button
            className={`${styles.tab} ${activeTab === 'register' ? styles.tabActive : ''}`}
            onClick={() => switchTab('register')}
          >
            注册
          </button>
        </div>

        {(localError || error) && (
          <div className={styles.error}>{(localError || error) ?? ''}</div>
        )}

        {activeTab === 'login' ? (
          <form className={styles.form} onSubmit={handleLogin}>
            <div className={styles.field}>
              <label htmlFor="login-email" className={styles.label}>邮箱</label>
              <input
                id="login-email"
                type="email"
                className={styles.input}
                value={loginEmail}
                onChange={(e) => setLoginEmail(e.target.value)}
                placeholder="you@example.com"
                required
                autoComplete="email"
                autoFocus
              />
            </div>

            <div className={styles.field}>
              <label htmlFor="login-password" className={styles.label}>密码</label>
              <input
                id="login-password"
                type="password"
                className={styles.input}
                value={loginPassword}
                onChange={(e) => setLoginPassword(e.target.value)}
                placeholder="输入密码"
                required
                autoComplete="current-password"
              />
            </div>

            <button type="submit" className={styles.button} disabled={submitting}>
              {submitting ? '登录中...' : '登录'}
            </button>
          </form>
        ) : (
          <form className={styles.form} onSubmit={handleRegister}>
            <div className={styles.field}>
              <label htmlFor="reg-email" className={styles.label}>邮箱</label>
              <input
                id="reg-email"
                type="email"
                className={styles.input}
                value={regEmail}
                onChange={(e) => setRegEmail(e.target.value)}
                placeholder="you@example.com"
                required
                autoComplete="email"
                autoFocus
              />
            </div>

            <div className={styles.field}>
              <label htmlFor="reg-display-name" className={styles.label}>显示名称</label>
              <input
                id="reg-display-name"
                type="text"
                className={styles.input}
                value={regDisplayName}
                onChange={(e) => setRegDisplayName(e.target.value)}
                placeholder="您的名称"
                required
                autoComplete="name"
              />
            </div>

            <div className={styles.field}>
              <label htmlFor="reg-password" className={styles.label}>密码</label>
              <input
                id="reg-password"
                type="password"
                className={styles.input}
                value={regPassword}
                onChange={(e) => setRegPassword(e.target.value)}
                placeholder="至少 8 个字符"
                required
                autoComplete="new-password"
                minLength={8}
              />
            </div>

            <div className={styles.field}>
              <label htmlFor="reg-confirm-password" className={styles.label}>确认密码</label>
              <input
                id="reg-confirm-password"
                type="password"
                className={styles.input}
                value={regConfirmPassword}
                onChange={(e) => setRegConfirmPassword(e.target.value)}
                placeholder="再次输入密码"
                required
                autoComplete="new-password"
                minLength={8}
              />
            </div>

            <button type="submit" className={styles.button} disabled={submitting}>
              {submitting ? '注册中...' : '注册'}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}

export default LoginPage;
