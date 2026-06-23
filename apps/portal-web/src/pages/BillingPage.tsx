import { useState, useCallback } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getWalletBalance,
  listLedgerEntries,
  listPaymentOrders,
} from '@/api/billing';
import type {
  WalletBalance,
  LedgerEntry,
  PaymentOrder,
} from '@/api/billing';
import { formatMicroUSD, formatYuan } from '@/utils/format';
import RechargeDialog from '@/components/RechargeDialog';
import styles from './BillingPage.module.css';

// ── Helpers ────────────────────────────────────────────

const PAYMENT_LABELS: Record<string, string> = {
  alipay: '支付宝',
  wechat: '微信支付',
};

function getOrderStatusClass(status: string): string {
  switch (status) {
    case 'completed':
    case 'paid':
      return styles.statusSuccess;
    case 'pending':
    case 'processing':
      return styles.statusPending;
    case 'failed':
      return styles.statusFailed;
    default:
      return styles.statusCancelled;
  }
}

function getOrderStatusLabel(status: string): string {
  switch (status) {
    case 'completed': return '已完成';
    case 'paid': return '已支付';
    case 'pending': return '待支付';
    case 'processing': return '处理中';
    case 'failed': return '失败';
    case 'cancelled': return '已取消';
    default: return status;
  }
}

function getEntryTypeLabel(type: string): string {
  switch (type) {
    case 'charge': return '充值';
    case 'spend': return '消费';
    case 'refund': return '退款';
    case 'freeze': return '冻结';
    case 'unfreeze': return '解冻';
    case 'adjustment': return '调整';
    default: return type;
  }
}

function getAmountClass(amountMicroUSD: number): string {
  if (amountMicroUSD > 0) return styles.amountPositive;
  if (amountMicroUSD < 0) return styles.amountNegative;
  return styles.amountZero;
}

// ── BillingPage ────────────────────────────────────────

function BillingPage() {
  const queryClient = useQueryClient();
  const [rechargeOpen, setRechargeOpen] = useState(false);
  const [ledgerPage, setLedgerPage] = useState(1);
  const [orderPage, setOrderPage] = useState(1);
  const [orderStatusFilter, setOrderStatusFilter] = useState('');
  const pageSize = 15;

  // Wallet balance
  const balanceQuery = useQuery({
    queryKey: ['walletBalance'],
    queryFn: getWalletBalance,
  });

  // Ledger entries
  const ledgerQuery = useQuery({
    queryKey: ['ledgerEntries', ledgerPage],
    queryFn: () => listLedgerEntries({ page: ledgerPage, page_size: pageSize }),
  });

  // Payment orders
  const ordersQuery = useQuery({
    queryKey: ['paymentOrders', orderPage, orderStatusFilter],
    queryFn: () => listPaymentOrders({
      page: orderPage,
      page_size: pageSize,
      status: orderStatusFilter || undefined,
    }),
  });

  const handleRechargeClose = useCallback(() => {
    setRechargeOpen(false);
    queryClient.invalidateQueries({ queryKey: ['walletBalance'] });
    queryClient.invalidateQueries({ queryKey: ['paymentOrders'] });
  }, [queryClient]);

  const balance: WalletBalance | undefined = balanceQuery.data;
  const entries: LedgerEntry[] = ledgerQuery.data?.data ?? [];
  const ledgerTotal = ledgerQuery.data?.total ?? 0;
  const ledgerPages = Math.ceil(ledgerTotal / pageSize);

  const orders: PaymentOrder[] = ordersQuery.data?.data ?? [];
  const orderTotal = ordersQuery.data?.total ?? 0;
  const orderPages = Math.ceil(orderTotal / pageSize);

  return (
    <div className={styles.page}>
      <h1 className={styles.pageTitle}>账单</h1>

      {/* ── Balance Cards ─────────────────────────────── */}
      <div className={styles.balanceSection}>
        {balanceQuery.isLoading && (
          <div className={styles.balanceLoading}>加载余额...</div>
        )}

        {balanceQuery.error && (
          <div className={styles.errorBanner}>
            <span>{(balanceQuery.error as Error).message || '加载失败'}</span>
            <button className={styles.retryBtn} onClick={() => balanceQuery.refetch()}>重试</button>
          </div>
        )}

        {balance && (
          <>
            <div className={styles.balanceCards}>
              <div className={styles.balanceCard}>
                <div className={styles.balanceCardLabel}>可用余额</div>
                <div className={styles.balanceCardValueAvailable}>
                  {formatMicroUSD(balance.available_micro_usd)}
                </div>
              </div>
              <div className={styles.balanceCard}>
                <div className={styles.balanceCardLabel}>冻结金额</div>
                <div className={styles.balanceCardValueFrozen}>
                  {formatMicroUSD(balance.frozen_micro_usd)}
                </div>
              </div>
              <div className={styles.balanceCard}>
                <div className={styles.balanceCardLabel}>总余额</div>
                <div className={styles.balanceCardValue}>
                  {formatMicroUSD(balance.balance_micro_usd)}
                </div>
              </div>
            </div>

            <button
              className={styles.rechargeBtn}
              onClick={() => setRechargeOpen(true)}
            >
              💰 充值
            </button>
          </>
        )}
      </div>

      {/* ── Payment Orders ────────────────────────────── */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>支付订单</h2>
          <select
            className={styles.filterSelect}
            value={orderStatusFilter}
            onChange={(e) => { setOrderStatusFilter(e.target.value); setOrderPage(1); }}
          >
            <option value="">全部</option>
            <option value="pending">待支付</option>
            <option value="completed">已完成</option>
            <option value="failed">失败</option>
          </select>
        </div>

        {ordersQuery.isLoading && <div className={styles.loading}>加载订单...</div>}

        {ordersQuery.error && (
          <div className={styles.errorBanner}>
            <span>{(ordersQuery.error as Error).message || '加载失败'}</span>
          </div>
        )}

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
                  <th>金额</th>
                  <th>状态</th>
                  <th>时间</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((order) => (
                  <tr key={order.id}>
                    <td className={styles.tableMono}>{order.order_no}</td>
                    <td>{PAYMENT_LABELS[order.payment_method] || order.payment_method}</td>
                    <td className={styles.amount}>{formatYuan(order.amount_yuan)}</td>
                    <td>
                      <span className={`${styles.statusBadge} ${getOrderStatusClass(order.status)}`}>
                        {getOrderStatusLabel(order.status)}
                      </span>
                    </td>
                    <td className={styles.tableTime}>
                      {new Date(order.created_at).toLocaleString('zh-CN')}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>

            {orderPages > 1 && (
              <div className={styles.pagination}>
                <button
                  className={styles.pageBtn}
                  onClick={() => setOrderPage((p) => Math.max(1, p - 1))}
                  disabled={orderPage <= 1}
                >
                  上一页
                </button>
                <span className={styles.pageInfo}>
                  第 {orderPage} / {orderPages} 页，共 {orderTotal} 条
                </span>
                <button
                  className={styles.pageBtn}
                  onClick={() => setOrderPage((p) => Math.min(orderPages, p + 1))}
                  disabled={orderPage >= orderPages}
                >
                  下一页
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* ── Ledger History ────────────────────────────── */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>账单历史</h2>
        </div>

        {ledgerQuery.isLoading && <div className={styles.loading}>加载账单...</div>}

        {ledgerQuery.error && (
          <div className={styles.errorBanner}>
            <span>{(ledgerQuery.error as Error).message || '加载失败'}</span>
          </div>
        )}

        {!ledgerQuery.isLoading && !ledgerQuery.error && entries.length === 0 && (
          <div className={styles.empty}>暂无账单记录</div>
        )}

        {entries.length > 0 && (
          <div className={styles.tableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>时间</th>
                  <th>类型</th>
                  <th>金额</th>
                  <th>描述</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.id}>
                    <td className={styles.tableTime}>
                      {new Date(entry.created_at).toLocaleString('zh-CN')}
                    </td>
                    <td>
                      <span className={`${styles.statusBadge} ${getOrderStatusClass(entry.entry_type)}`}>
                        {getEntryTypeLabel(entry.entry_type)}
                      </span>
                    </td>
                    <td className={getAmountClass(entry.amount_micro_usd)}>
                      {formatMicroUSD(entry.amount_micro_usd)}
                    </td>
                    <td className={styles.tableDesc}>
                      {entry.description || '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>

            {ledgerPages > 1 && (
              <div className={styles.pagination}>
                <button
                  className={styles.pageBtn}
                  onClick={() => setLedgerPage((p) => Math.max(1, p - 1))}
                  disabled={ledgerPage <= 1}
                >
                  上一页
                </button>
                <span className={styles.pageInfo}>
                  第 {ledgerPage} / {ledgerPages} 页，共 {ledgerTotal} 条
                </span>
                <button
                  className={styles.pageBtn}
                  onClick={() => setLedgerPage((p) => Math.min(ledgerPages, p + 1))}
                  disabled={ledgerPage >= ledgerPages}
                >
                  下一页
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* ── Recharge Dialog ───────────────────────────── */}
      <RechargeDialog open={rechargeOpen} onClose={handleRechargeClose} />
    </div>
  );
}

export default BillingPage;
