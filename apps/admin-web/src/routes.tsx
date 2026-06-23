import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router';
import { lazy } from 'react';

const DashboardPage = lazy(() => import('@/pages/DashboardPage'));
const UsersPage = lazy(() => import('@/pages/UsersPage'));
const APIKeysPage = lazy(() => import('@/pages/APIKeysPage'));
const ChannelsPage = lazy(() => import('@/pages/ChannelsPage'));
const BillingPage = lazy(() => import('@/pages/BillingPage'));
const SecurityPage = lazy(() => import('@/pages/SecurityPage'));
const SupportPage = lazy(() => import('@/pages/SupportPage'));

const rootRoute = createRootRoute();

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: DashboardPage,
});

const usersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/users',
  component: UsersPage,
});

const apiKeysRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/api-keys',
  component: APIKeysPage,
});

const channelsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/channels',
  component: ChannelsPage,
});

const billingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/billing',
  component: BillingPage,
});

const securityRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/security',
  component: SecurityPage,
});

const supportRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/support',
  component: SupportPage,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  usersRoute,
  apiKeysRoute,
  channelsRoute,
  billingRoute,
  securityRoute,
  supportRoute,
]);

export const router = createRouter({ routeTree });

export const adminNavItems = [
  { path: '/', label: '数据概览' },
  { path: '/users', label: '用户与组织' },
  { path: '/api-keys', label: 'API 与模型' },
  { path: '/channels', label: '渠道管理' },
  { path: '/billing', label: '计费财务' },
  { path: '/security', label: '运营安全' },
  { path: '/support', label: '客服工单' },
] as const;
