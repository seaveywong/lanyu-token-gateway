import { useState, useMemo, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getUsageSummary, getUsageTrend } from '@/api/usage';
import type { UsageByModel, UsageByDay } from '@/api/usage';
import { formatMicroUSD } from '@/utils/format';
import styles from './UsagePage.module.css';

// ── Helpers ────────────────────────────────────────────

type DateRange = '7d' | '30d' | 'custom';

function getDateRange(range: DateRange, customStart?: string, customEnd?: string) {
  const end = range === 'custom' && customEnd
    ? new Date(customEnd)
    : new Date();
  end.setHours(23, 59, 59, 999);

  const start = new Date(end);
  if (range === '7d') {
    start.setDate(start.getDate() - 7);
  } else if (range === '30d') {
    start.setDate(start.getDate() - 30);
  } else if (range === 'custom' && customStart) {
    return {
      start_date: new Date(customStart).toISOString().slice(0, 10),
      end_date: end.toISOString().slice(0, 10),
    };
  }
  start.setHours(0, 0, 0, 0);

  return {
    start_date: start.toISOString().slice(0, 10),
    end_date: end.toISOString().slice(0, 10),
  };
}

function formatTokens(n: number): string {
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1)}B`;
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

// ── Bar chart component ────────────────────────────────

function BarChart({ data, valueKey, color }: {
  data: UsageByDay[];
  valueKey: keyof UsageByDay;
  color: string;
}) {
  const maxVal = Math.max(...data.map((d) => Number(d[valueKey]) || 0), 1);

  return (
    <div className={styles.barChart}>
      {data.map((day) => {
        const val = Number(day[valueKey]) || 0;
        const pct = (val / maxVal) * 100;
        return (
          <div key={day.date} className={styles.barItem}>
            <div className={styles.barLabel}>
              {new Date(day.date).toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })}
            </div>
            <div className={styles.barTrack}>
              <div
                className={styles.bar}
                style={{ width: `${pct}%`, background: color }}
              />
            </div>
            <div className={styles.barValue}>
              {valueKey === 'cost_micro_usd'
                ? formatMicroUSD(val)
                : formatTokens(val)}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ── CSV export ─────────────────────────────────────────

function exportCSV(models: UsageByModel[]) {
  const header = '模型名称,请求数,输入 Token,输出 Token,缓存 Token,费用 (USD)';
  const rows = models.map((m) =>
    [
      m.model_name,
      m.request_count,
      m.input_tokens,
      m.output_tokens,
      m.cached_tokens ?? 0,
      formatMicroUSD(m.cost_micro_usd).replace('$', ''),
    ].join(',')
  );
  const csv = [header, ...rows].join('\n');
  const blob = new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `usage-export-${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

// ── UsagePage ──────────────────────────────────────────

function UsagePage() {
  const [dateRange, setDateRange] = useState<DateRange>('7d');
  const [customStart, setCustomStart] = useState('');
  const [customEnd, setCustomEnd] = useState('');
  const [chartMode, setChartMode] = useState<'cost' | 'tokens' | 'requests'>('cost');

  const dates = useMemo(
    () => getDateRange(dateRange, customStart || undefined, customEnd || undefined),
    [dateRange, customStart, customEnd],
  );

  const summaryQuery = useQuery({
    queryKey: ['usageSummary', dates],
    queryFn: () => getUsageSummary(dates),
  });

  const trendQuery = useQuery({
    queryKey: ['usageTrend', dates],
    queryFn: () => getUsageTrend(dates),
  });

  const models: UsageByModel[] = summaryQuery.data?.by_model ?? [];
  const summary = summaryQuery.data;
  const trendData: UsageByDay[] = trendQuery.data?.data ?? [];

  const handleExport = useCallback(() => {
    if (models.length > 0) {
      exportCSV(models);
    }
  }, [models]);

  const handleDateRangeChange = (range: DateRange) => {
    setDateRange(range);
    if (range !== 'custom') {
      setCustomStart('');
      setCustomEnd('');
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>用量统计</h1>
        <div className={styles.pageActions}>
          <button className={styles.exportBtn} onClick={handleExport} disabled={models.length === 0}>
            📥 导出 CSV
          </button>
        </div>
      </div>

      {/* ── Date range selector ──────────────────────── */}
      <div className={styles.dateBar}>
        <div className={styles.dateRangeBtns}>
          <button
            className={dateRange === '7d' ? styles.dateBtnActive : styles.dateBtn}
            onClick={() => handleDateRangeChange('7d')}
          >
            最近 7 天
          </button>
          <button
            className={dateRange === '30d' ? styles.dateBtnActive : styles.dateBtn}
            onClick={() => handleDateRangeChange('30d')}
          >
            最近 30 天
          </button>
          <button
            className={dateRange === 'custom' ? styles.dateBtnActive : styles.dateBtn}
            onClick={() => handleDateRangeChange('custom')}
          >
            自定义
          </button>
        </div>

        {dateRange === 'custom' && (
          <div className={styles.customDates}>
            <input
              type="date"
              className={styles.dateInput}
              value={customStart}
              onChange={(e) => setCustomStart(e.target.value)}
            />
            <span className={styles.dateSep}>至</span>
            <input
              type="date"
              className={styles.dateInput}
              value={customEnd}
              onChange={(e) => setCustomEnd(e.target.value)}
            />
          </div>
        )}
      </div>

      {/* ── Summary cards ────────────────────────────── */}
      {summaryQuery.isLoading && <div className={styles.loading}>加载用量数据...</div>}

      {summaryQuery.error && (
        <div className={styles.errorBanner}>
          <span>{(summaryQuery.error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => summaryQuery.refetch()}>重试</button>
        </div>
      )}

      {summary && (
        <div className={styles.summaryCards}>
          <div className={styles.summaryCard}>
            <div className={styles.summaryCardLabel}>总请求数</div>
            <div className={styles.summaryCardValue}>
              {summary.total_requests.toLocaleString()}
            </div>
          </div>
          <div className={styles.summaryCard}>
            <div className={styles.summaryCardLabel}>输入 Token</div>
            <div className={styles.summaryCardValue}>
              {formatTokens(summary.total_input_tokens)}
            </div>
          </div>
          <div className={styles.summaryCard}>
            <div className={styles.summaryCardLabel}>输出 Token</div>
            <div className={styles.summaryCardValue}>
              {formatTokens(summary.total_output_tokens)}
            </div>
          </div>
          <div className={styles.summaryCard}>
            <div className={styles.summaryCardLabel}>总费用</div>
            <div className={styles.summaryCardValueCost}>
              {formatMicroUSD(summary.total_cost_micro_usd)}
            </div>
          </div>
        </div>
      )}

      {/* ── Trend chart ──────────────────────────────── */}
      {trendData.length > 0 && !trendQuery.isLoading && (
        <div className={styles.chartSection}>
          <div className={styles.chartHeader}>
            <h3 className={styles.chartTitle}>用量趋势</h3>
            <div className={styles.chartToggles}>
              <button
                className={chartMode === 'cost' ? styles.toggleActive : styles.toggle}
                onClick={() => setChartMode('cost')}
              >
                费用
              </button>
              <button
                className={chartMode === 'tokens' ? styles.toggleActive : styles.toggle}
                onClick={() => setChartMode('tokens')}
              >
                Token
              </button>
              <button
                className={chartMode === 'requests' ? styles.toggleActive : styles.toggle}
                onClick={() => setChartMode('requests')}
              >
                请求数
              </button>
            </div>
          </div>

          {chartMode === 'cost' && (
            <BarChart data={trendData} valueKey="cost_micro_usd" color="#4f46e5" />
          )}
          {chartMode === 'tokens' && (
            <BarChart data={trendData} valueKey="input_tokens" color="#059669" />
          )}
          {chartMode === 'requests' && (
            <BarChart data={trendData} valueKey="request_count" color="#f59e0b" />
          )}
        </div>
      )}

      {/* ── By model table ───────────────────────────── */}
      {!summaryQuery.isLoading && !summaryQuery.error && models.length > 0 && (
        <div className={styles.section}>
          <h3 className={styles.sectionTitle}>按模型分组</h3>
          <div className={styles.tableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>模型名称</th>
                  <th>请求数</th>
                  <th>输入 Token</th>
                  <th>输出 Token</th>
                  <th>缓存 Token</th>
                  <th>费用</th>
                </tr>
              </thead>
              <tbody>
                {models.map((model) => (
                  <tr key={model.model_name}>
                    <td className={styles.tableName}>{model.model_name}</td>
                    <td className={styles.amount}>{model.request_count.toLocaleString()}</td>
                    <td className={styles.amount}>{formatTokens(model.input_tokens)}</td>
                    <td className={styles.amount}>{formatTokens(model.output_tokens)}</td>
                    <td className={styles.amount}>
                      {model.cached_tokens > 0 ? formatTokens(model.cached_tokens) : '-'}
                    </td>
                    <td className={styles.amount}>{formatMicroUSD(model.cost_micro_usd)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {!summaryQuery.isLoading && !summaryQuery.error && models.length === 0 && summary && (
        <div className={styles.empty}>该时间段暂无用量数据</div>
      )}
    </div>
  );
}

export default UsagePage;
