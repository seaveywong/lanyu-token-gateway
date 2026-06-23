import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router';
import { lazy } from 'react';

const DashboardPage = lazy(() => import('@/pages/DashboardPage'));
const UsersPage = lazy(() => import('@/pages/UsersPage'));
const APIKeysPage = lazy(() => import('@/pages/APIKeysPage'));
const ChannelsPage = lazy(() => import('@/pages/ChannelsPage'));
const ModelsPage = lazy(() => import('@/pages/ModelsPage'));
const BillingPage = lazy(() => import('@/pages/BillingPage'));
const SecurityPage = lazy(() => import('@/pages/SecurityPage'));
const SupportPage = lazy(() => import('@/pages/SupportPage'));
const ApprovalsPage = lazy(() => import('@/pages/ApprovalsPage'));
const SettingsPage = lazy(() => import('@/pages/SettingsPage'));
const LoginPage = lazy(() => import('@/pages/LoginPage'));

const rootRoute = createRootRoute();

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
});

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

const modelsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/models',
  component: ModelsPage,
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

const approvalsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/approvals',
  component: ApprovalsPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings',
  component: SettingsPage,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  indexRoute,
  usersRoute,
  apiKeysRoute,
  channelsRoute,
  modelsRoute,
  billingRoute,
  securityRoute,
  supportRoute,
  approvalsRoute,
  settingsRoute,
]);

export const router = createRouter({ routeTree });

export const adminNavItems = [
  { path: '/', label: '数据概览' },
  { path: '/users', label: '用户与组织' },
  { path: '/api-keys', label: 'API 与模型' },
  { path: '/channels', label: '渠道管理' },
  { path: '/models', label: '模型管理' },
  { path: '/billing', label: '计费财务' },
  { path: '/approvals', label: '审批管理' },
  { path: '/security', label: '运营安全' },
  { path: '/settings', label: '系统设置' },
  { path: '/support', label: '客服工单' },
] as const;
