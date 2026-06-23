import { Suspense, useState, useEffect, useCallback } from 'react';
import { RouterProvider } from '@tanstack/react-router';
import { router, portalNavItems } from '@/routes';
import { useAuth } from '@lanyu/web-shared/auth/AuthContext';
import styles from './App.module.css';

function AppLayout() {
  const { user, loading, logout } = useAuth();
  const [currentPath, setCurrentPath] = useState(window.location.pathname);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const isAuthPage = currentPath === '/login' || currentPath === '/register';

  useEffect(() => {
    const handlePopState = () => {
      setCurrentPath(window.location.pathname);
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const handleNavigate = useCallback((path: string) => {
    router.navigate({ to: path });
    setMobileMenuOpen(false);
  }, []);

  // Auth guard: redirect to /login if not authenticated
  useEffect(() => {
    if (!loading && !user && !isAuthPage) {
      router.navigate({ to: '/login' });
    }
  }, [loading, user, isAuthPage]);

  const handleLogout = async () => {
    await logout();
    router.navigate({ to: '/login' });
  };

  // Auth pages (login/register): minimal layout (no header nav / footer)
  if (isAuthPage) {
    return (
      <div className={styles.layout}>
        <main className={styles.mainFull}>
          <Suspense fallback={<div className={styles.loading}>加载中...</div>}>
            <RouterProvider router={router} />
          </Suspense>
        </main>
      </div>
    );
  }

  // Loading state
  if (loading) {
    return (
      <div className={styles.layout}>
        <main className={styles.mainFull}>
          <div className={styles.loading}>加载中...</div>
        </main>
      </div>
    );
  }

  // Not authenticated (shouldn't reach here due to redirect, but fallback)
  if (!user) {
    return null;
  }

  return (
    <div className={styles.layout}>
      <header className={styles.navbar}>
        <div className={styles.navInner}>
          <a
            href="/dashboard"
            className={styles.brand}
            onClick={(e) => {
              e.preventDefault();
              handleNavigate('/dashboard');
            }}
          >
            🔑 兰语 Token
          </a>

          <button
            className={styles.hamburger}
            onClick={() => setMobileMenuOpen((prev) => !prev)}
            aria-label="切换菜单"
          >
            <span className={styles.hamburgerLine} />
            <span className={styles.hamburgerLine} />
            <span className={styles.hamburgerLine} />
          </button>

          <nav className={`${styles.navLinks} ${mobileMenuOpen ? styles.navOpen : ''}`}>
            {portalNavItems.map((item) => (
              <button
                key={item.path}
                className={`${styles.navLink} ${currentPath === item.path ? styles.active : ''}`}
                onClick={() => handleNavigate(item.path)}
              >
                {item.label}
              </button>
            ))}
          </nav>

          <div className={styles.userInfo}>
            <span className={styles.userEmail}>{user.email}</span>
            <button className={styles.logoutButton} onClick={handleLogout}>
              退出
            </button>
          </div>
        </div>
      </header>

      <main className={styles.main}>
        <Suspense fallback={<div className={styles.loading}>加载中...</div>}>
          <RouterProvider router={router} />
        </Suspense>
      </main>

      <footer className={styles.footer}>
        <span>&copy; 2026 兰语 Token 网关</span>
      </footer>
    </div>
  );
}

function App() {
  return <AppLayout />;
}

export default App;
