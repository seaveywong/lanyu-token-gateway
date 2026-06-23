import { Suspense, useState, useEffect, useCallback } from 'react';
import { RouterProvider } from '@tanstack/react-router';
import { router, adminNavItems } from '@/routes';
import { useAuth } from '@lanyu/web-shared/auth/AuthContext';
import Sidebar from '@/components/Sidebar';
import styles from './App.module.css';

function AppLayout() {
  const { user, loading, logout } = useAuth();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [currentPath, setCurrentPath] = useState(window.location.pathname);

  const isLoginPage = currentPath === '/login';

  useEffect(() => {
    const handlePopState = () => {
      setCurrentPath(window.location.pathname);
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const handleNavigate = useCallback((path: string) => {
    router.navigate({ to: path });
    setSidebarOpen(false);
  }, []);

  const toggleSidebar = useCallback(() => {
    setSidebarOpen((prev) => !prev);
  }, []);

  // Auth guard: redirect to /login if not authenticated
  useEffect(() => {
    if (!loading && !user && !isLoginPage) {
      router.navigate({ to: '/login' });
    }
  }, [loading, user, isLoginPage]);

  // Login page: minimal layout (no sidebar)
  if (isLoginPage) {
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

  const currentLabel = adminNavItems.find((item) => item.path === currentPath)?.label ?? '';

  return (
    <div className={styles.layout}>
      {sidebarOpen && (
        <div
          className={styles.overlay}
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}

      <Sidebar
        navItems={adminNavItems}
        currentPath={currentPath}
        onNavigate={handleNavigate}
        isOpen={sidebarOpen}
        userEmail={user.email}
        onLogout={async () => {
          await logout();
          router.navigate({ to: '/login' });
        }}
      />

      <div className={styles.main}>
        <header className={styles.topbar}>
          <button
            className={styles.hamburger}
            onClick={toggleSidebar}
            aria-label="切换菜单"
          >
            <span className={styles.hamburgerLine} />
            <span className={styles.hamburgerLine} />
            <span className={styles.hamburgerLine} />
          </button>
          <h1 className={styles.pageTitle}>{currentLabel}</h1>
        </header>
        <main className={styles.content}>
          <Suspense fallback={<div className={styles.loading}>加载中...</div>}>
            <RouterProvider router={router} />
          </Suspense>
        </main>
      </div>
    </div>
  );
}

function App() {
  return <AppLayout />;
}

export default App;
