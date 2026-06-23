import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router';
import { lazy } from 'react';

const DashboardPage = lazy(() => import('@/pages/DashboardPage'));
const ProjectsPage = lazy(() => import('@/pages/ProjectsPage'));
const APIKeysPage = lazy(() => import('@/pages/APIKeysPage'));
const UsagePage = lazy(() => import('@/pages/UsagePage'));
const BillingPage = lazy(() => import('@/pages/BillingPage'));
const DeveloperPage = lazy(() => import('@/pages/DeveloperPage'));
const SupportPage = lazy(() => import('@/pages/SupportPage'));
const LoginPage = lazy(() => import('@/pages/LoginPage'));
const RegisterPage = lazy(() => import('@/pages/RegisterPage'));

const rootRoute = createRootRoute();

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
});

const registerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/register',
  component: RegisterPage,
});

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/dashboard',
  component: DashboardPage,
});

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
  loginRoute,
  registerRoute,
  dashboardRoute,
  indexRoute,
  projectsRoute,
  apiKeysRoute,
  usageRoute,
  billingRoute,
  developerRoute,
  supportRoute,
]);

export const router = createRouter({ routeTree });

export const portalNavItems = [
  { path: '/dashboard', label: '首页' },
  { path: '/projects', label: '项目' },
  { path: '/api-keys', label: 'API Key' },
  { path: '/usage', label: '用量' },
  { path: '/billing', label: '账单' },
  { path: '/developer', label: '开发者' },
  { path: '/support', label: '支持' },
] as const;
