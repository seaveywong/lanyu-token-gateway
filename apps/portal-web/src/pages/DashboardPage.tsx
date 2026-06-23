import { useQuery } from '@tanstack/react-query';
import { getDashboardSummary } from '@/api/usage';
import type { DashboardSummary } from '@/api/usage';
import { formatMicroUSD } from '@/utils/format';
import styles from './DashboardPage.module.css';

// ── Helpers ────────────────────────────────────────────

function ProgressBar({ used, total, color }: { used: number; total: number; color: string }) {
  const pct = total > 0 ? Math.min((used / total) * 100, 100) : 0;
  const dangerPct = total > 0 ? (used / total) * 100 : 0;

  let barColor = color;
  if (dangerPct >= 90) barColor = '#ef4444';
  else if (dangerPct >= 75) barColor = '#f59e0b';

  return (
    <div className={styles.progressWrapper}>
      <div className={styles.progressTrack}>
        <div
          className={styles.progressBar}
          style={{ width: `${pct}%`, background: barColor }}
        />
      </div>
      <span className={styles.progressLabel}>{pct.toFixed(0)}%</span>
    </div>
  );
}

// ── Skeleton ───────────────────────────────────────────

function SkeletonCard() {
  return (
    <div className={styles.card}>
      <div className={styles.skeletonLabel} />
      <div className={styles.skeletonValue} />
    </div>
  );
}

// ── DashboardPage ──────────────────────────────────────

function DashboardPage() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['dashboardSummary'],
    queryFn: getDashboardSummary,
    staleTime: 60_000,
    refetchInterval: 120_000,
  });

  const summary: DashboardSummary | undefined = data;

  return (
    <div className={styles.page}>
      <h1 className={styles.pageTitle}>首页</h1>

      {/* ── Error ───────────────────────────────────── */}
      {error && (
        <div className={styles.errorBanner}>
          <span>{(error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => refetch()}>重试</button>
        </div>
      )}

      {/* ── Balance Overview ────────────────────────── */}
      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>余额概览</h2>
        <div className={styles.cardGrid}>
          {isLoading && (
            <>
              <SkeletonCard />
              <SkeletonCard />
            </>
          )}

          {summary && (
            <>
              <div className={styles.card}>
                <div className={styles.cardLabel}>可用余额</div>
                <div className={styles.cardValueBalance}>
                  {formatMicroUSD(summary.balance.available_micro_usd)}
                </div>
              </div>
              <div className={styles.card}>
                <div className={styles.cardLabel}>冻结金额</div>
                <div className={styles.cardValueFrozen}>
                  {formatMicroUSD(summary.balance.frozen_micro_usd)}
                </div>
              </div>
            </>
          )}

          {!isLoading && !summary && (
            <div className={styles.emptyCard}>暂无数据</div>
          )}
        </div>
      </div>

      {/* ── Today Usage ─────────────────────────────── */}
      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>今日用量</h2>
        <div className={styles.cardGrid}>
          {isLoading && (
            <>
              <SkeletonCard />
              <SkeletonCard />
              <SkeletonCard />
            </>
          )}

          {summary && (
            <>
              <div className={styles.card}>
                <div className={styles.cardLabel}>请求数</div>
                <div className={styles.cardValue}>
                  {summary.today.requests.toLocaleString()}
                </div>
              </div>
              <div className={styles.card}>
                <div className={styles.cardLabel}>Token 消耗</div>
                <div className={styles.cardValue}>
                  {summary.today.tokens >= 1_000_000
                    ? `${(summary.today.tokens / 1_000_000).toFixed(1)}M`
                    : summary.today.tokens >= 1_000
                      ? `${(summary.today.tokens / 1_000).toFixed(1)}K`
                      : String(summary.today.tokens)}
                </div>
              </div>
              <div className={styles.card}>
                <div className={styles.cardLabel}>费用</div>
                <div className={styles.cardValueCost}>
                  {formatMicroUSD(summary.today.cost_micro_usd)}
                </div>
              </div>
            </>
          )}
        </div>
      </div>

      {/* ── Recent Errors ───────────────────────────── */}
      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>最近错误</h2>
        {isLoading && <SkeletonCard />}

        {summary && (
          <div className={styles.card}>
            <div className={styles.errorCardInner}>
              <div className={styles.errorCardStat}>
                <div className={styles.cardLabel}>错误数</div>
                <div className={styles.cardValue} style={summary.recent_errors.count > 0 ? { color: '#dc2626' } : { color: '#059669' }}>
                  {summary.recent_errors.count}
                </div>
              </div>
              <div className={styles.errorCardStat}>
                <div className={styles.cardLabel}>错误率</div>
                <div className={styles.cardValue} style={summary.recent_errors.rate > 0.05 ? { color: '#dc2626' } : { color: '#059669' }}>
                  {(summary.recent_errors.rate * 100).toFixed(2)}%
                </div>
              </div>
            </div>
          </div>
        )}

        {!isLoading && !summary && (
          <div className={styles.emptyCard}>暂无数据</div>
        )}
      </div>

      {/* ── Project Budgets ─────────────────────────── */}
      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>项目预算</h2>

        {isLoading && (
          <div className={styles.cardGrid}>
            <SkeletonCard />
            <SkeletonCard />
          </div>
        )}

        {summary && summary.projects.length === 0 && (
          <div className={styles.emptyCard}>暂无项目</div>
        )}

        {summary && summary.projects.length > 0 && (
          <div className={styles.projectList}>
            {summary.projects.map((project) => (
              <div key={project.id} className={styles.projectCard}>
                <div className={styles.projectHeader}>
                  <span className={styles.projectName}>{project.name}</span>
                  <span className={styles.projectId}>{project.id}</span>
                </div>

                <div className={styles.projectBudgets}>
                  {project.daily_budget_micro_usd > 0 && (
                    <div className={styles.projectBudgetItem}>
                      <div className={styles.budgetHeader}>
                        <span className={styles.budgetLabel}>日预算</span>
                        <span className={styles.budgetValue}>
                          {formatMicroUSD(project.daily_used_micro_usd)} / {formatMicroUSD(project.daily_budget_micro_usd)}
                        </span>
                      </div>
                      <ProgressBar
                        used={project.daily_used_micro_usd}
                        total={project.daily_budget_micro_usd}
                        color="#4f46e5"
                      />
                    </div>
                  )}

                  {project.monthly_budget_micro_usd > 0 && (
                    <div className={styles.projectBudgetItem}>
                      <div className={styles.budgetHeader}>
                        <span className={styles.budgetLabel}>月预算</span>
                        <span className={styles.budgetValue}>
                          {formatMicroUSD(project.monthly_used_micro_usd)} / {formatMicroUSD(project.monthly_budget_micro_usd)}
                        </span>
                      </div>
                      <ProgressBar
                        used={project.monthly_used_micro_usd}
                        total={project.monthly_budget_micro_usd}
                        color="#059669"
                      />
                    </div>
                  )}

                  {project.daily_budget_micro_usd <= 0 && project.monthly_budget_micro_usd <= 0 && (
                    <span className={styles.noBudget}>未设置预算</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export default DashboardPage;
