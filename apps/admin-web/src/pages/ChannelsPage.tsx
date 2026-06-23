import { useState, useCallback, type ReactNode } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import PageHeader from '@/components/PageHeader';
import AddSourceDialog from '@/components/AddSourceDialog';
import SourceDetailDrawer from '@/components/SourceDetailDrawer';
import {
  listAccountSources,
  disableAccountSource,
  validateAccountSource,
} from '@/api/sources';
import {
  listChannels,
  createChannel,
  listRouteRules,
  listModelMappings,
  createModelMapping,
} from '@/api/channels';
import type { AccountSource, Channel, RouteRule, ModelMapping } from '@/api';
import styles from './ChannelsPage.module.css';

// ---- Types ----

type TabKey = 'sources' | 'channels' | 'mappings' | 'rules';

interface Tab {
  key: TabKey;
  label: string;
}

const TABS: Tab[] = [
  { key: 'sources', label: '账号来源' },
  { key: 'channels', label: '渠道' },
  { key: 'mappings', label: '模型映射' },
  { key: 'rules', label: '路由规则' },
];

const SOURCE_TYPE_LABELS: Record<string, string> = {
  official_api_key: '官方 API Key',
  official_oauth: '官方 OAuth',
  upstream_api: '上游 API',
  subscription_pool: '订阅池',
};

function getTypeBadgeClass(type: string): string {
  switch (type) {
    case 'official_oauth': return styles.typeOAuth;
    case 'upstream_api': return styles.typeUpstream;
    case 'subscription_pool': return styles.typePool;
    default: return styles.typeOfficial;
  }
}

function getHealthClass(state: string): string {
  switch (state) {
    case 'healthy': return styles.healthHealthy;
    case 'degraded': return styles.healthDegraded;
    default: return styles.healthUnhealthy;
  }
}

// ---- Toast ----

interface ToastState {
  message: string;
  type: 'success' | 'error';
}

function Toast({ toast }: { toast: ToastState | null }) {
  if (!toast) return null;
  return (
    <div className={toast.type === 'success' ? styles.toastSuccess : styles.toastError}>
      {toast.message}
    </div>
  );
}

// ---- Sub-components ----

function SourcesTab() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [sourceType, setSourceType] = useState('');
  const [viewMode, setViewMode] = useState<'table' | 'card'>('table');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [detailSourceId, setDetailSourceId] = useState<string | null>(null);
  const [toast, setToast] = useState<ToastState | null>(null);
  const pageSize = 20;

  const showToast = useCallback((message: string, type: 'success' | 'error') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['accountSources', { page, pageSize, sourceType }],
    queryFn: () => listAccountSources({ page, page_size: pageSize, source_type: sourceType || undefined }),
  });

  const disableMutation = useMutation({
    mutationFn: disableAccountSource,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accountSources'] });
      showToast('已禁用', 'success');
    },
    onError: (err: Error) => showToast(err.message || '操作失败', 'error'),
  });

  const validateMutation = useMutation({
    mutationFn: validateAccountSource,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accountSources'] });
      showToast('验证任务已触发', 'success');
    },
    onError: (err: Error) => showToast(err.message || '操作失败', 'error'),
  });

  const sources: AccountSource[] = data?.data ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <select
            className={styles.filterSelect}
            value={sourceType}
            onChange={(e) => {
              setSourceType(e.target.value);
              setPage(1);
            }}
          >
            <option value="">全部类型</option>
            <option value="official_api_key">官方 API Key</option>
            <option value="official_oauth">官方 OAuth</option>
            <option value="upstream_api">上游 API</option>
            <option value="subscription_pool">订阅池</option>
          </select>

          <div className={styles.viewToggle}>
            <button
              className={viewMode === 'table' ? styles.viewToggleBtnActive : styles.viewToggleBtn}
              onClick={() => setViewMode('table')}
            >
              表格
            </button>
            <button
              className={viewMode === 'card' ? styles.viewToggleBtnActive : styles.viewToggleBtn}
              onClick={() => setViewMode('card')}
            >
              卡片
            </button>
          </div>
        </div>

        <button className={styles.addButton} onClick={() => setDialogOpen(true)}>
          + 添加来源
        </button>
      </div>

      {error && (
        <div className={styles.errorBanner}>
          <span>{(error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => refetch()}>重试</button>
        </div>
      )}

      {isLoading && <div className={styles.loading}>加载中...</div>}

      {!isLoading && !error && sources.length === 0 && (
        <div className={styles.empty}>暂无账号来源，点击「添加来源」创建第一个</div>
      )}

      {!isLoading && sources.length > 0 && (
        <>
          {viewMode === 'table' ? (
            <div className={styles.tableWrapper}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>类型</th>
                    <th>优先级</th>
                    <th>权重</th>
                    <th>健康</th>
                    <th>并发</th>
                    <th>日预算</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {sources.map((src) => (
                    <tr key={src.id}>
                      <td className={styles.tableName}>{src.name}</td>
                      <td>
                        <span className={`${styles.typeBadge} ${getTypeBadgeClass(src.source_type)}`}>
                          {SOURCE_TYPE_LABELS[src.source_type] || src.source_type}
                        </span>
                      </td>
                      <td>{src.priority}</td>
                      <td>{src.weight}</td>
                      <td>
                        <span className={`${styles.healthDot} ${getHealthClass(src.health_state)}`} />
                        {src.health_state === 'healthy' ? '正常' : src.health_state === 'degraded' ? '降级' : '异常'}
                      </td>
                      <td>{src.max_concurrency}</td>
                      <td>
                        {src.daily_budget_micro_usd > 0
                          ? `$${(src.daily_budget_micro_usd / 1_000_000).toFixed(2)}`
                          : '-'}
                      </td>
                      <td>
                        <div className={styles.actionBtns}>
                          <button className={styles.actionBtn} onClick={() => setDetailSourceId(src.id)}>
                            详情
                          </button>
                          <button
                            className={styles.actionBtn}
                            onClick={() => validateMutation.mutate(src.id)}
                            disabled={validateMutation.isPending}
                          >
                            验证
                          </button>
                          <button
                            className={styles.actionBtnDanger}
                            onClick={() => {
                              if (window.confirm(`确定禁用「${src.name}」？`)) {
                                disableMutation.mutate(src.id);
                              }
                            }}
                            disabled={disableMutation.isPending}
                          >
                            禁用
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className={styles.cardGrid}>
              {sources.map((src) => (
                <div key={src.id} className={styles.card}>
                  <div className={styles.cardHeader}>
                    <span className={styles.cardName}>{src.name}</span>
                    <span className={`${styles.typeBadge} ${getTypeBadgeClass(src.source_type)}`}>
                      {SOURCE_TYPE_LABELS[src.source_type] || src.source_type}
                    </span>
                  </div>

                  <div className={styles.cardMeta}>
                    <div className={styles.cardField}>
                      优先级
                      <span className={styles.cardFieldValue}>{src.priority}</span>
                    </div>
                    <div className={styles.cardField}>
                      权重
                      <span className={styles.cardFieldValue}>{src.weight}</span>
                    </div>
                    <div className={styles.cardField}>
                      健康
                      <span className={styles.cardFieldValue}>
                        <span className={`${styles.healthDot} ${getHealthClass(src.health_state)}`} />
                        {src.health_state === 'healthy' ? '正常' : src.health_state === 'degraded' ? '降级' : '异常'}
                      </span>
                    </div>
                    <div className={styles.cardField}>
                      并发
                      <span className={styles.cardFieldValue}>{src.max_concurrency}</span>
                    </div>
                    {src.daily_budget_micro_usd > 0 && (
                      <div className={styles.cardField}>
                        日预算
                        <span className={styles.cardFieldValue}>
                          ${(src.daily_budget_micro_usd / 1_000_000).toFixed(2)}
                        </span>
                      </div>
                    )}
                  </div>

                  <div className={styles.cardActions}>
                    <button className={styles.actionBtn} onClick={() => setDetailSourceId(src.id)}>
                      详情
                    </button>
                    <button
                      className={styles.actionBtn}
                      onClick={() => validateMutation.mutate(src.id)}
                    >
                      验证
                    </button>
                    <button
                      className={styles.actionBtnDanger}
                      onClick={() => {
                        if (window.confirm(`确定禁用「${src.name}」？`)) {
                          disableMutation.mutate(src.id);
                        }
                      }}
                    >
                      禁用
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}

          {totalPages > 1 && (
            <div className={styles.pagination}>
              <button
                className={styles.pageBtn}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
              >
                上一页
              </button>
              <span className={styles.pageInfo}>
                第 {page} / {totalPages} 页，共 {total} 条
              </span>
              <button
                className={styles.pageBtn}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
              >
                下一页
              </button>
            </div>
          )}
        </>
      )}

      <AddSourceDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
      <SourceDetailDrawer sourceId={detailSourceId} onClose={() => setDetailSourceId(null)} />
      <Toast toast={toast} />
    </div>
  );
}

function ChannelsTab() {
  const queryClient = useQueryClient();
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [toast, setToast] = useState<ToastState | null>(null);

  const showToast = useCallback((message: string, type: 'success' | 'error') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['channels'],
    queryFn: listChannels,
  });

  const createMutation = useMutation({
    mutationFn: createChannel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] });
      setShowAddForm(false);
      setNewName('');
      setNewDesc('');
      showToast('渠道已创建', 'success');
    },
    onError: (err: Error) => showToast(err.message || '创建失败', 'error'),
  });

  const channels: Channel[] = data?.data ?? [];

  const handleCreate = () => {
    if (!newName.trim()) {
      showToast('请输入渠道名称', 'error');
      return;
    }
    createMutation.mutate({
      name: newName.trim(),
      description: newDesc.trim() || undefined,
    });
  };

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <span style={{ fontSize: 13, color: '#6b7280' }}>
          {channels.length} 个渠道
        </span>
        <button className={styles.addButton} onClick={() => setShowAddForm(true)}>
          + 添加渠道
        </button>
      </div>

      {showAddForm && (
        <div style={{
          background: '#fff',
          border: '1px solid #e5e7eb',
          borderRadius: 8,
          padding: '12px 16px',
        }}>
          <div className={styles.inlineForm}>
            <input
              className={styles.inlineInput}
              placeholder="渠道名称"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <input
              className={styles.inlineInput}
              placeholder="描述（可选）"
              value={newDesc}
              onChange={(e) => setNewDesc(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <button
              className={styles.inlineConfirm}
              onClick={handleCreate}
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? '创建中...' : '确认'}
            </button>
            <button
              className={styles.actionBtn}
              onClick={() => setShowAddForm(false)}
            >
              取消
            </button>
          </div>
        </div>
      )}

      {error && (
        <div className={styles.errorBanner}>
          <span>{(error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => refetch()}>重试</button>
        </div>
      )}

      {isLoading && <div className={styles.loading}>加载中...</div>}

      {!isLoading && !error && channels.length === 0 && (
        <div className={styles.empty}>暂无渠道，点击「添加渠道」创建</div>
      )}

      {!isLoading && channels.length > 0 && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th style={{ width: 40 }} />
                <th>名称</th>
                <th>描述</th>
                <th>状态</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {channels.map((ch) => (
                <>
                  <tr
                    key={ch.id}
                    className={styles.channelRow}
                    onClick={() => setExpandedId(expandedId === ch.id ? null : ch.id)}
                  >
                    <td style={{ textAlign: 'center', color: '#9ca3af' }}>
                      {expandedId === ch.id ? '▼' : '▶'}
                    </td>
                    <td className={styles.tableName}>{ch.name}</td>
                    <td style={{ color: '#6b7280', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {ch.description || '-'}
                    </td>
                    <td>{ch.status === 'active' ? '活跃' : ch.status}</td>
                    <td style={{ fontSize: 13, color: '#6b7280' }}>
                      {new Date(ch.created_at).toLocaleDateString('zh-CN')}
                    </td>
                    <td>
                      <button className={styles.actionBtn}>管理来源</button>
                    </td>
                  </tr>
                  {expandedId === ch.id && (
                    <tr className={styles.expandedRow}>
                      <td colSpan={6}>
                        <div className={styles.expandedContent}>
                          <div className={styles.expandedTitle}>关联的来源</div>
                          <div className={styles.noSources}>
                            后端接口开发中（/admin-api/channels/{ch.id}），届时将展示关联的账号来源列表。
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Toast toast={toast} />
    </div>
  );
}

function MappingsTab() {
  const [toast, setToast] = useState<ToastState | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [extModel, setExtModel] = useState('');
  const [natModel, setNatModel] = useState('');
  const [multiplier, setMultiplier] = useState('1.0');
  const queryClient = useQueryClient();

  const showToast = useCallback((message: string, type: 'success' | 'error') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['modelMappings'],
    queryFn: () => listModelMappings({ page_size: 100 }),
  });

  const createMappingMutation = useMutation({
    mutationFn: createModelMapping,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['modelMappings'] });
      setShowAddForm(false);
      setExtModel('');
      setNatModel('');
      setMultiplier('1.0');
      showToast('模型映射已创建', 'success');
    },
    onError: (err: Error) => showToast(err.message || '创建失败', 'error'),
  });

  const mappings: ModelMapping[] = data?.data ?? [];

  const handleCreate = () => {
    if (!extModel.trim() || !natModel.trim()) {
      showToast('请填写外部模型和原生模型', 'error');
      return;
    }
    const cost = parseFloat(multiplier);
    if (isNaN(cost) || cost <= 0) {
      showToast('成本倍率必须大于 0', 'error');
      return;
    }
    createMappingMutation.mutate({
      external_model: extModel.trim(),
      native_model: natModel.trim(),
      cost_multiplier: cost,
    });
  };

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <span style={{ fontSize: 13, color: '#6b7280' }}>
          {mappings.length} 条映射
        </span>
        <button className={styles.addButton} onClick={() => setShowAddForm(true)}>
          + 添加映射
        </button>
      </div>

      {showAddForm && (
        <div style={{
          background: '#fff',
          border: '1px solid #e5e7eb',
          borderRadius: 8,
          padding: '12px 16px',
        }}>
          <div className={styles.inlineForm}>
            <input
              className={styles.inlineInput}
              placeholder="外部模型 (如 gpt-4)"
              value={extModel}
              onChange={(e) => setExtModel(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <span style={{ color: '#9ca3af' }}>&rarr;</span>
            <input
              className={styles.inlineInput}
              placeholder="原生模型 (如 gpt-4o)"
              value={natModel}
              onChange={(e) => setNatModel(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <input
              className={styles.inlineInput}
              placeholder="成本倍率"
              value={multiplier}
              onChange={(e) => setMultiplier(e.target.value)}
              style={{ maxWidth: 80 }}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <button
              className={styles.inlineConfirm}
              onClick={handleCreate}
              disabled={createMappingMutation.isPending}
            >
              {createMappingMutation.isPending ? '...' : '确认'}
            </button>
            <button
              className={styles.actionBtn}
              onClick={() => setShowAddForm(false)}
            >
              取消
            </button>
          </div>
        </div>
      )}

      {error && (
        <div className={styles.errorBanner}>
          <span>{(error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => refetch()}>重试</button>
        </div>
      )}

      {isLoading && <div className={styles.loading}>加载中...</div>}

      {!isLoading && !error && mappings.length === 0 && (
        <div className={styles.empty}>暂无模型映射</div>
      )}

      {!isLoading && mappings.length > 0 && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>外部模型</th>
                <th>原生模型</th>
                <th>成本倍率</th>
                <th>状态</th>
                <th>创建时间</th>
              </tr>
            </thead>
            <tbody>
              {mappings.map((m) => (
                <tr key={m.id}>
                  <td className={styles.tableName}>{m.external_model}</td>
                  <td>{m.native_model}</td>
                  <td>x{m.cost_multiplier}</td>
                  <td>{m.status === 'active' ? '活跃' : m.status}</td>
                  <td style={{ fontSize: 13, color: '#6b7280' }}>
                    {new Date(m.created_at).toLocaleDateString('zh-CN')}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Toast toast={toast} />
    </div>
  );
}

function RulesTab() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['routeRules'],
    queryFn: () => listRouteRules({ page_size: 100 }),
  });

  const rules: RouteRule[] = data?.data ?? [];

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <span style={{ fontSize: 13, color: '#6b7280' }}>
          {rules.length} 条规则
        </span>
      </div>

      {error && (
        <div className={styles.errorBanner}>
          <span>{(error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => refetch()}>重试</button>
        </div>
      )}

      {isLoading && <div className={styles.loading}>加载中...</div>}

      {!isLoading && !error && rules.length === 0 && (
        <div className={styles.empty}>暂无路由规则</div>
      )}

      {!isLoading && rules.length > 0 && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>组织/项目</th>
                <th>模型匹配</th>
                <th>渠道</th>
                <th>优先级</th>
                <th>权重</th>
                <th>状态</th>
                <th>创建时间</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((r) => (
                <tr key={r.id}>
                  <td>
                    {r.org_id ? (
                      <span style={{ fontSize: 13 }}>{r.org_id}{r.project_id ? ` / ${r.project_id}` : ''}</span>
                    ) : (
                      <span style={{ color: '#9ca3af', fontSize: 13 }}>全局</span>
                    )}
                  </td>
                  <td className={styles.tableName}>{r.model_pattern}</td>
                  <td>{r.channel_name || r.channel_id}</td>
                  <td>{r.priority}</td>
                  <td>{r.weight}</td>
                  <td>{r.status === 'active' ? '活跃' : r.status}</td>
                  <td style={{ fontSize: 13, color: '#6b7280' }}>
                    {new Date(r.created_at).toLocaleDateString('zh-CN')}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {isLoading && <div className={styles.loading}>加载中...</div>}
      {!isLoading && !error && rules.length === 0 && (
        <div className={styles.empty}>暂无路由规则，后端接口开发中</div>
      )}
    </div>
  );
}

// ---- Main ChannelsPage ----

function ChannelsPage() {
  const [activeTab, setActiveTab] = useState<TabKey>('sources');

  const tabContent: Record<TabKey, ReactNode> = {
    sources: <SourcesTab />,
    channels: <ChannelsTab />,
    mappings: <MappingsTab />,
    rules: <RulesTab />,
  };

  return (
    <div className={styles.page}>
      <PageHeader
        title="渠道管理"
        breadcrumbs={[{ label: '渠道管理' }]}
      />

      <div className={styles.tabs}>
        {TABS.map((tab) => (
          <button
            key={tab.key}
            className={activeTab === tab.key ? styles.tabActive : styles.tab}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {tabContent[activeTab]}
    </div>
  );
}

export default ChannelsPage;
