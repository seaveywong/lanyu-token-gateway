import { useState, useCallback, type ReactNode } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import PageHeader from '@/components/PageHeader';
import {
  listPricingVersions,
  getPricingVersion,
  createPricingVersion,
  getWalletBalance,
  listLedgerEntries,
  listPaymentOrders,
  markPaymentOrderComplete,
  getReconciliationReport,
} from '@/api/billing';
import type {
  PricingVersion,
  PricingRule,
  WalletBalance,
  LedgerEntry,
  PaymentOrder,
  ReconciliationDifference,
} from '@/api/billing';
import { formatMicroUSD, formatYuan } from '@/utils/format';
import styles from './BillingPage.module.css';

// ── Types ──────────────────────────────────────────────

type TabKey = 'pricing' | 'wallet' | 'orders' | 'reconciliation';

interface Tab {
  key: TabKey;
  label: string;
}

const TABS: Tab[] = [
  { key: 'pricing', label: '价格管理' },
  { key: 'wallet', label: '钱包管理' },
  { key: 'orders', label: '支付订单' },
  { key: 'reconciliation', label: '对账' },
];

// ── Helpers ────────────────────────────────────────────

function getStatusClass(status: string): string {
  switch (status) {
    case 'active':
    case 'completed':
    case 'paid':
      return styles.statusActive;
    case 'pending':
    case 'processing':
      return styles.statusPending;
    case 'failed':
    case 'cancelled':
      return styles.statusFailed;
    default:
      return styles.statusCancelled;
  }
}

function getStatusLabel(status: string): string {
  switch (status) {
    case 'active': return '活跃';
    case 'completed': return '已完成';
    case 'paid': return '已支付';
    case 'pending': return '待处理';
    case 'processing': return '处理中';
    case 'failed': return '失败';
    case 'cancelled': return '已取消';
    default: return status;
  }
}

function getAmountClass(amountMicroUSD: number): string {
  if (amountMicroUSD > 0) return styles.amountPositive;
  if (amountMicroUSD < 0) return styles.amountNegative;
  return styles.amountZero;
}

// ── Toast ──────────────────────────────────────────────

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

// ── Pricing Tab ────────────────────────────────────────

function PricingTab() {
  const queryClient = useQueryClient();
  const [selectedVersionId, setSelectedVersionId] = useState<string>('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newVersionName, setNewVersionName] = useState('');
  const [newModelName, setNewModelName] = useState('');
  const [newInputPrice, setNewInputPrice] = useState('');
  const [newOutputPrice, setNewOutputPrice] = useState('');
  const [newCachedPrice, setNewCachedPrice] = useState('');
  const [toast, setToast] = useState<ToastState | null>(null);

  const showToast = useCallback((message: string, type: 'success' | 'error') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const versionsQuery = useQuery({
    queryKey: ['pricingVersions'],
    queryFn: () => listPricingVersions(),
  });

  const detailQuery = useQuery({
    queryKey: ['pricingVersion', selectedVersionId],
    queryFn: () => getPricingVersion(selectedVersionId),
    enabled: selectedVersionId !== '',
  });

  const createMutation = useMutation({
    mutationFn: createPricingVersion,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['pricingVersions'] });
      setShowCreateForm(false);
      setNewVersionName('');
      setNewModelName('');
      setNewInputPrice('');
      setNewOutputPrice('');
      setNewCachedPrice('');
      showToast('价格版本已创建', 'success');
    },
    onError: (err: Error) => showToast(err.message || '创建失败', 'error'),
  });

  const versions: PricingVersion[] = versionsQuery.data?.data ?? [];
  const rules: PricingRule[] = detailQuery.data?.rules ?? [];

  // Auto-select first version
  if (!selectedVersionId && versions.length > 0 && !versionsQuery.isLoading) {
    // Use microtask to avoid setState during render
    queueMicrotask(() => setSelectedVersionId(versions[0].id));
  }

  const handleCreate = () => {
    if (!newVersionName.trim() || !newModelName.trim() || !newInputPrice || !newOutputPrice) {
      showToast('请填写完整信息', 'error');
      return;
    }
    const inputPrice = parseFloat(newInputPrice);
    const outputPrice = parseFloat(newOutputPrice);
    const cachedPrice = newCachedPrice ? parseFloat(newCachedPrice) : 0;
    if (isNaN(inputPrice) || isNaN(outputPrice) || isNaN(cachedPrice)) {
      showToast('价格格式错误', 'error');
      return;
    }
    createMutation.mutate({
      name: newVersionName.trim(),
      rules: [{
        model_name: newModelName.trim(),
        input_price_micro_usd: inputPrice,
        output_price_micro_usd: outputPrice,
        cached_price_micro_usd: cachedPrice,
      }],
    });
  };

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <select
            className={styles.filterSelect}
            value={selectedVersionId}
            onChange={(e) => setSelectedVersionId(e.target.value)}
          >
            <option value="">选择价格版本...</option>
            {versions.map((v) => (
              <option key={v.id} value={v.id}>
                {v.name} {v.is_active ? '(当前)' : ''}
              </option>
            ))}
          </select>
        </div>
        <button className={styles.addButton} onClick={() => setShowCreateForm(true)}>
          + 新建版本
        </button>
      </div>

      {versionsQuery.error && (
        <div className={styles.errorBanner}>
          <span>{(versionsQuery.error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => versionsQuery.refetch()}>重试</button>
        </div>
      )}

      {versionsQuery.isLoading && <div className={styles.loading}>加载中...</div>}

      {!versionsQuery.isLoading && !versionsQuery.error && versions.length === 0 && (
        <div className={styles.empty}>暂无价格版本</div>
      )}

      {showCreateForm && (
        <div className={styles.inlineForm}>
          <div className={styles.inlineFormTitle}>新建价格版本</div>
          <div className={styles.formGrid}>
            <div className={styles.formField}>
              <label className={styles.formLabel}>版本名称</label>
              <input
                className={styles.formInput}
                placeholder="如 2026-Q1"
                value={newVersionName}
                onChange={(e) => setNewVersionName(e.target.value)}
              />
            </div>
            <div className={styles.formField}>
              <label className={styles.formLabel}>模型名称</label>
              <input
                className={styles.formInput}
                placeholder="如 gpt-4o"
                value={newModelName}
                onChange={(e) => setNewModelName(e.target.value)}
              />
            </div>
            <div className={styles.formField}>
              <label className={styles.formLabel}>输入价格 (micro USD)</label>
              <input
                className={styles.formInput}
                placeholder="如 5000000 ($5/1M tokens)"
                value={newInputPrice}
                onChange={(e) => setNewInputPrice(e.target.value)}
              />
            </div>
            <div className={styles.formField}>
              <label className={styles.formLabel}>输出价格 (micro USD)</label>
              <input
                className={styles.formInput}
                placeholder="如 15000000 ($15/1M tokens)"
                value={newOutputPrice}
                onChange={(e) => setNewOutputPrice(e.target.value)}
              />
            </div>
            <div className={styles.formField}>
              <label className={styles.formLabel}>缓存价格 (micro USD, 可选)</label>
              <input
                className={styles.formInput}
                placeholder="如 2500000 ($2.5/1M tokens)"
                value={newCachedPrice}
                onChange={(e) => setNewCachedPrice(e.target.value)}
              />
            </div>
          </div>
          <div className={styles.formActions}>
            <button
              className={styles.formConfirm}
              onClick={handleCreate}
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? '创建中...' : '确认创建'}
            </button>
            <button className={styles.formCancel} onClick={() => setShowCreateForm(false)}>
              取消
            </button>
          </div>
        </div>
      )}

      {detailQuery.isLoading && <div className={styles.loading}>加载价格规则...</div>}

      {detailQuery.error && (
        <div className={styles.errorBanner}>
          <span>{(detailQuery.error as Error).message || '加载价格规则失败'}</span>
        </div>
      )}

      {!detailQuery.isLoading && !detailQuery.error && selectedVersionId && rules.length > 0 && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>模型名称</th>
                <th>输入价格</th>
                <th>输出价格</th>
                <th>缓存价格</th>
                <th>图片价格</th>
                <th>音频价格</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id}>
                  <td className={styles.tableName}>{rule.model_name}</td>
                  <td className={styles.amount}>
                    {formatMicroUSD(rule.input_price_micro_usd)}<span style={{ color: '#9ca3af', fontSize: 11 }}>/1M tokens</span>
                  </td>
                  <td className={styles.amount}>
                    {formatMicroUSD(rule.output_price_micro_usd)}<span style={{ color: '#9ca3af', fontSize: 11 }}>/1M tokens</span>
                  </td>
                  <td className={styles.amount}>
                    {rule.cached_price_micro_usd > 0
                      ? <>{formatMicroUSD(rule.cached_price_micro_usd)}<span style={{ color: '#9ca3af', fontSize: 11 }}>/1M tokens</span></>
                      : '-'}
                  </td>
                  <td className={styles.amount}>
                    {rule.image_price_micro_usd > 0
                      ? <>{formatMicroUSD(rule.image_price_micro_usd)}<span style={{ color: '#9ca3af', fontSize: 11 }}>/image</span></>
                      : '-'}
                  </td>
                  <td className={styles.amount}>
                    {rule.audio_price_micro_usd > 0
                      ? <>{formatMicroUSD(rule.audio_price_micro_usd)}<span style={{ color: '#9ca3af', fontSize: 11 }}>/audio</span></>
                      : '-'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!detailQuery.isLoading && !detailQuery.error && selectedVersionId && rules.length === 0 && (
        <div className={styles.empty}>该版本暂无价格规则</div>
      )}

      <Toast toast={toast} />
    </div>
  );
}

// ── Wallet Tab ─────────────────────────────────────────

function WalletTab() {
  const [orgId, setOrgId] = useState('');
  const [searchOrgId, setSearchOrgId] = useState('');
  const [ledgerPage, setLedgerPage] = useState(1);
  const ledgerPageSize = 20;

  const balanceQuery = useQuery({
    queryKey: ['walletBalance', searchOrgId],
    queryFn: () => getWalletBalance(searchOrgId),
    enabled: searchOrgId !== '',
  });

  const ledgerQuery = useQuery({
    queryKey: ['ledgerEntries', searchOrgId, ledgerPage],
    queryFn: () => listLedgerEntries(searchOrgId, { page: ledgerPage, page_size: ledgerPageSize }),
    enabled: searchOrgId !== '',
  });

  const balance: WalletBalance | undefined = balanceQuery.data;
  const entries: LedgerEntry[] = ledgerQuery.data?.data ?? [];
  const ledgerTotal = ledgerQuery.data?.total ?? 0;
  const totalPages = Math.ceil(ledgerTotal / ledgerPageSize);

  const handleSearch = () => {
    if (orgId.trim()) {
      setSearchOrgId(orgId.trim());
      setLedgerPage(1);
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <input
            className={styles.filterInput}
            placeholder="输入组织 ID..."
            value={orgId}
            onChange={(e) => setOrgId(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          />
          <button className={styles.addButton} onClick={handleSearch}>
            查询
          </button>
        </div>
      </div>

      {!searchOrgId && (
        <div className={styles.empty}>请输入组织 ID 查询钱包信息</div>
      )}

      {balanceQuery.error && (
        <div className={styles.errorBanner}>
          <span>{(balanceQuery.error as Error).message || '查询失败'}</span>
          <button className={styles.retryBtn} onClick={() => balanceQuery.refetch()}>重试</button>
        </div>
      )}

      {balanceQuery.isLoading && <div className={styles.loading}>查询中...</div>}

      {balance && (
        <div className={styles.cardGrid}>
          <div className={styles.card}>
            <div className={styles.cardLabel}>总余额</div>
            <div className={styles.cardValue}>{formatMicroUSD(balance.balance_micro_usd)}</div>
          </div>
          <div className={styles.card}>
            <div className={styles.cardLabel}>冻结金额</div>
            <div className={styles.cardValueFrozen}>{formatMicroUSD(balance.frozen_micro_usd)}</div>
          </div>
          <div className={styles.card}>
            <div className={styles.cardLabel}>可用余额</div>
            <div className={styles.cardValueAvailable}>{formatMicroUSD(balance.available_micro_usd)}</div>
          </div>
        </div>
      )}

      {searchOrgId && ledgerQuery.isLoading && <div className={styles.loading}>加载账本记录...</div>}

      {searchOrgId && ledgerQuery.error && !balanceQuery.error && (
        <div className={styles.errorBanner}>
          <span>{(ledgerQuery.error as Error).message || '加载账本失败'}</span>
          <button className={styles.retryBtn} onClick={() => ledgerQuery.refetch()}>重试</button>
        </div>
      )}

      {entries.length > 0 && (
        <>
          <h3 style={{ fontSize: 15, fontWeight: 600, color: '#111827', marginTop: 8 }}>
            账本历史
          </h3>
          <div className={styles.tableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>时间</th>
                  <th>类型</th>
                  <th>金额</th>
                  <th>描述</th>
                  <th>请求 ID</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.id}>
                    <td style={{ fontSize: 13, color: '#6b7280' }}>
                      {new Date(entry.created_at).toLocaleString('zh-CN')}
                    </td>
                    <td>
                      <span className={`${styles.statusBadge} ${getStatusClass(entry.entry_type)}`}>
                        {entry.entry_type}
                      </span>
                    </td>
                    <td className={getAmountClass(entry.amount_micro_usd)}>
                      {formatMicroUSD(entry.amount_micro_usd)}
                    </td>
                    <td style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {entry.description || '-'}
                    </td>
                    <td style={{ fontSize: 12, fontFamily: 'monospace', color: '#6b7280', maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {entry.request_id || '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div className={styles.pagination}>
              <button
                className={styles.pageBtn}
                onClick={() => setLedgerPage((p) => Math.max(1, p - 1))}
                disabled={ledgerPage <= 1}
              >
                上一页
              </button>
              <span className={styles.pageInfo}>
                第 {ledgerPage} / {totalPages} 页，共 {ledgerTotal} 条
              </span>
              <button
                className={styles.pageBtn}
                onClick={() => setLedgerPage((p) => Math.min(totalPages, p + 1))}
                disabled={ledgerPage >= totalPages}
              >
                下一页
              </button>
            </div>
          )}
        </>
      )}

      {searchOrgId && !ledgerQuery.isLoading && !ledgerQuery.error && balance && entries.length === 0 && (
        <div className={styles.empty}>暂无账本记录</div>
      )}
    </div>
  );
}

// ── Payment Orders Tab ─────────────────────────────────

function OrdersTab() {
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState('');
  const [page, setPage] = useState(1);
  const [toast, setToast] = useState<ToastState | null>(null);
  const pageSize = 20;

  const showToast = useCallback((message: string, type: 'success' | 'error') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const ordersQuery = useQuery({
    queryKey: ['paymentOrders', { status: statusFilter, page }],
    queryFn: () => listPaymentOrders({
      status: statusFilter || undefined,
      page,
      page_size: pageSize,
    }),
  });

  const completeMutation = useMutation({
    mutationFn: markPaymentOrderComplete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['paymentOrders'] });
      showToast('订单已标记为完成', 'success');
    },
    onError: (err: Error) => showToast(err.message || '操作失败', 'error'),
  });

  const orders: PaymentOrder[] = ordersQuery.data?.data ?? [];
  const total = ordersQuery.data?.total ?? 0;
  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <select
            className={styles.filterSelect}
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          >
            <option value="">全部状态</option>
            <option value="pending">待处理</option>
            <option value="processing">处理中</option>
            <option value="completed">已完成</option>
            <option value="failed">失败</option>
            <option value="cancelled">已取消</option>
          </select>
        </div>
        <span style={{ fontSize: 13, color: '#6b7280' }}>共 {total} 条</span>
      </div>

      {ordersQuery.error && (
        <div className={styles.errorBanner}>
          <span>{(ordersQuery.error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => ordersQuery.refetch()}>重试</button>
        </div>
      )}

      {ordersQuery.isLoading && <div className={styles.loading}>加载中...</div>}

      {!ordersQuery.isLoading && !ordersQuery.error && orders.length === 0 && (
        <div className={styles.empty}>暂无支付订单</div>
      )}

      {orders.length > 0 && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>订单号</th>
                <th>支付方式</th>
                <th>金额 (元)</th>
                <th>金额 (USD)</th>
                <th>状态</th>
                <th>创建时间</th>
                <th>完成时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr key={order.id}>
                  <td className={styles.tableName} style={{ fontSize: 13, fontFamily: 'monospace' }}>
                    {order.order_no}
                  </td>
                  <td>{order.payment_method === 'alipay' ? '支付宝' : order.payment_method === 'wechat' ? '微信支付' : order.payment_method}</td>
                  <td className={styles.amount}>{formatYuan(order.amount_yuan)}</td>
                  <td className={styles.amount}>{formatMicroUSD(order.amount_micro_usd)}</td>
                  <td>
                    <span className={`${styles.statusBadge} ${getStatusClass(order.status)}`}>
                      {getStatusLabel(order.status)}
                    </span>
                  </td>
                  <td style={{ fontSize: 13, color: '#6b7280' }}>
                    {new Date(order.created_at).toLocaleString('zh-CN')}
                  </td>
                  <td style={{ fontSize: 13, color: '#6b7280' }}>
                    {order.completed_at ? new Date(order.completed_at).toLocaleString('zh-CN') : '-'}
                  </td>
                  <td>
                    {(order.status === 'pending' || order.status === 'processing') && (
                      <button
                        className={styles.actionBtnSuccess}
                        onClick={() => {
                          if (window.confirm(`确定将订单 ${order.order_no} 标记为已完成？`)) {
                            completeMutation.mutate(order.id);
                          }
                        }}
                        disabled={completeMutation.isPending}
                      >
                        标记完成
                      </button>
                    )}
                    {order.status !== 'pending' && order.status !== 'processing' && (
                      <span style={{ color: '#9ca3af', fontSize: 12 }}>-</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

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
        </div>
      )}

      <Toast toast={toast} />
    </div>
  );
}

// ── Reconciliation Tab ─────────────────────────────────

function ReconciliationTab() {
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10));
  const [searchDate, setSearchDate] = useState('');

  const reconciliationQuery = useQuery({
    queryKey: ['reconciliation', searchDate],
    queryFn: () => getReconciliationReport({ date: searchDate }),
    enabled: searchDate !== '',
  });

  const report = reconciliationQuery.data?.report;
  const differences: ReconciliationDifference[] = reconciliationQuery.data?.differences ?? [];

  const handleSearch = () => {
    if (date) {
      setSearchDate(date);
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <input
            type="date"
            className={styles.dateInput}
            value={date}
            onChange={(e) => setDate(e.target.value)}
          />
          <button className={styles.addButton} onClick={handleSearch}>
            查询对账结果
          </button>
        </div>
      </div>

      {!searchDate && (
        <div className={styles.empty}>请选择日期查看对账报告</div>
      )}

      {reconciliationQuery.error && (
        <div className={styles.errorBanner}>
          <span>{(reconciliationQuery.error as Error).message || '查询失败'}</span>
          <button className={styles.retryBtn} onClick={() => reconciliationQuery.refetch()}>重试</button>
        </div>
      )}

      {reconciliationQuery.isLoading && <div className={styles.loading}>对账中...</div>}

      {report && (
        <>
          <div className={styles.reconSummary}>
            <div className={styles.reconCard}>
              <div className={styles.reconCardLabel}>期望金额</div>
              <div className={styles.reconCardValue}>{formatMicroUSD(report.total_expected_micro_usd)}</div>
            </div>
            <div className={styles.reconCard}>
              <div className={styles.reconCardLabel}>实际金额</div>
              <div className={styles.reconCardValue}>{formatMicroUSD(report.total_actual_micro_usd)}</div>
            </div>
            <div className={styles.reconCard}>
              <div className={styles.reconCardLabel}>对账结果</div>
              <div className={styles.reconCardValue} style={{ color: report.discrepancy_count === 0 ? '#059669' : '#dc2626' }}>
                {report.discrepancy_count === 0 ? '一致' : `${report.discrepancy_count} 项差异`}
              </div>
            </div>
          </div>

          {differences.length > 0 && (
            <div className={styles.tableWrapper}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>类型</th>
                    <th>期望金额</th>
                    <th>实际金额</th>
                    <th>差异</th>
                    <th>状态</th>
                    <th>描述</th>
                  </tr>
                </thead>
                <tbody>
                  {differences.map((diff, idx) => (
                    <tr key={diff.id || idx}>
                      <td className={styles.tableName}>{diff.type}</td>
                      <td className={styles.amount}>{formatMicroUSD(diff.expected_micro_usd)}</td>
                      <td className={styles.amount}>{formatMicroUSD(diff.actual_micro_usd)}</td>
                      <td className={getAmountClass(diff.difference_micro_usd)}>
                        {formatMicroUSD(diff.difference_micro_usd)}
                      </td>
                      <td>
                        {diff.difference_micro_usd === 0 ? (
                          <span className={styles.badgeMatch}>匹配</span>
                        ) : (
                          <span className={styles.badgeMismatch}>差异</span>
                        )}
                      </td>
                      <td style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {diff.description || '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {differences.length === 0 && (
            <div className={styles.empty}>当日无差异记录，对账一致</div>
          )}
        </>
      )}
    </div>
  );
}

// ── Main BillingPage ───────────────────────────────────

function BillingPage() {
  const [activeTab, setActiveTab] = useState<TabKey>('pricing');

  const tabContent: Record<TabKey, ReactNode> = {
    pricing: <PricingTab />,
    wallet: <WalletTab />,
    orders: <OrdersTab />,
    reconciliation: <ReconciliationTab />,
  };

  return (
    <div>
      <PageHeader
        title="计费财务"
        breadcrumbs={[{ label: '计费财务' }]}
      />

      <div className={styles.tabs} style={{ marginTop: 16 }}>
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

      <div style={{ marginTop: 16 }}>
        {tabContent[activeTab]}
      </div>
    </div>
  );
}

export default BillingPage;
