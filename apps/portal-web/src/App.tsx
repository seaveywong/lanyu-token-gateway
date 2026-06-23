import { Suspense, useState, useEffect, useCallback } from 'react';
import { createRootRoute, createRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { lazy } from 'react';
import styles from './App.module.css';

// ── Routes ────────────────────────────────────────────

const DashboardPage = lazy(() => import('@/pages/DashboardPage'));
const ProjectsPage = lazy(() => import('@/pages/ProjectsPage'));
const APIKeysPage = lazy(() => import('@/pages/APIKeysPage'));
const UsagePage = lazy(() => import('@/pages/UsagePage'));
const BillingPage = lazy(() => import('@/pages/BillingPage'));
const DeveloperPage = lazy(() => import('@/pages/DeveloperPage'));
const SupportPage = lazy(() => import('@/pages/SupportPage'));

const rootRoute = createRootRoute();

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: DashboardPage,
});

const projectsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/projects',
  component: ProjectsPage,
});

const apiKeysRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/api-keys',
  component: APIKeysPage,
});

const usageRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/usage',
  component: UsagePage,
});

const billingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/billing',
  component: BillingPage,
});

const developerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/developer',
  component: DeveloperPage,
});

const supportRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/support',
  component: SupportPage,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  projectsRoute,
  apiKeysRoute,
  usageRoute,
  billingRoute,
  developerRoute,
  supportRoute,
]);

const router = createRouter({ routeTree });

const portalNavItems = [
  { path: '/', label: '首页' },
  { path: '/projects', label: '项目' },
  { path: '/api-keys', label: 'API Key' },
  { path: '/usage', label: '用量' },
  { path: '/billing', label: '账单' },
  { path: '/developer', label: '开发者' },
  { path: '/support', label: '支持' },
] as const;

// ── App ───────────────────────────────────────────────

function AppLayout() {
  const [currentPath, setCurrentPath] = useState(window.location.pathname);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  useEffect(() => {
    const unsubscribe = router.subscribe('onResolved', ({ pathname }) => {
      setCurrentPath(pathname);
    });
    return unsubscribe;
  }, []);

  const handleNavigate = useCallback((path: string) => {
    router.navigate({ to: path });
    setMobileMenuOpen(false);
  }, []);

  return (
    <div className={styles.layout}>
      <header className={styles.navbar}>
        <div className={styles.navInner}>
          <a
            href="/"
            className={styles.brand}
            onClick={(e) => {
              e.preventDefault();
              handleNavigate('/');
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
