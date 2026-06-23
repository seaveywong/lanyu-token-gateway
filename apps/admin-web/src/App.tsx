import { Suspense, useState, useEffect, useCallback } from 'react';
import { RouterProvider } from '@tanstack/react-router';
import { router, adminNavItems } from '@/routes';
import Sidebar from '@/components/Sidebar';
import styles from './App.module.css';

function AppLayout() {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [currentPath, setCurrentPath] = useState(window.location.pathname);

  useEffect(() => {
    const unsubscribe = router.subscribe('onResolved', ({ pathname }) => {
      setCurrentPath(pathname);
    });
    return unsubscribe;
  }, []);

  const handleNavigate = useCallback((path: string) => {
    router.navigate({ to: path });
    setSidebarOpen(false);
  }, []);

  const toggleSidebar = useCallback(() => {
    setSidebarOpen((prev) => !prev);
  }, []);

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
