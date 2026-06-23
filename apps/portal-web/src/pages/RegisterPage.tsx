import { useState, type FormEvent } from 'react';
import { useAuth } from '@lanyu/web-shared/auth/AuthContext';
import styles from './RegisterPage.module.css';

function RegisterPage() {
  const { user, loading, error, register } = useAuth();
  const [email, setEmail] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);

  if (!loading && user) {
    window.location.href = '/dashboard';
    return null;
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLocalError(null);

    if (password !== confirmPassword) {
      setLocalError('两次输入的密码不一致');
      return;
    }

    if (password.length < 8) {
      setLocalError('密码长度至少为 8 个字符');
      return;
    }

    setSubmitting(true);
    try {
      await register(email, password, displayName);
      window.location.href = '/dashboard';
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : '注册失败，请重试');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <div className={styles.header}>
          <h1 className={styles.title}>创建账户</h1>
          <p className={styles.subtitle}>注册蓝域 API 控制台账户</p>
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
              placeholder="you@example.com"
              required
              autoComplete="email"
              autoFocus
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="displayName" className={styles.label}>显示名称</label>
            <input
              id="displayName"
              type="text"
              className={styles.input}
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="您的名称"
              required
              autoComplete="name"
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
              placeholder="至少 8 个字符"
              required
              autoComplete="new-password"
              minLength={8}
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="confirmPassword" className={styles.label}>确认密码</label>
            <input
              id="confirmPassword"
              type="password"
              className={styles.input}
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
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

        <p className={styles.switchHint}>
          已有账户？{' '}
          <a href="/login" className={styles.switchLink}>
            立即登录
          </a>
        </p>
      </div>
    </div>
  );
}

export default RegisterPage;
