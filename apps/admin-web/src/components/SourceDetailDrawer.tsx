import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getAccountSource, disableAccountSource, validateAccountSource } from '@/api/sources';
import styles from './SourceDetailDrawer.module.css';

interface SourceDetailDrawerProps {
  sourceId: string | null;
  onClose: () => void;
}

const TYPE_LABELS: Record<string, string> = {
  official_api_key: '官方 API Key',
  official_oauth: '官方 OAuth',
  upstream_api: '上游 API',
  subscription_pool: '订阅池',
};

function getTypeBadgeClass(type: string): string {
  switch (type) {
    case 'official_oauth':
      return styles.typeOAuth;
    case 'upstream_api':
      return styles.typeUpstream;
    case 'subscription_pool':
      return styles.typePool;
    default:
      return styles.typeOfficial;
  }
}

function getHealthDotClass(state: string): string {
  switch (state) {
    case 'healthy':
      return styles.healthy;
    case 'degraded':
      return styles.degraded;
    default:
      return styles.unhealthy;
  }
}

function formatDateTime(iso?: string): string {
  if (!iso) return '未记录';
  try {
    return new Date(iso).toLocaleString('zh-CN');
  } catch {
    return iso;
  }
}

function SourceDetailDrawer({ sourceId, onClose }: SourceDetailDrawerProps) {
  const queryClient = useQueryClient();

  const { data: source, isLoading, error } = useQuery({
    queryKey: ['accountSource', sourceId],
    queryFn: () => getAccountSource(sourceId!),
    enabled: !!sourceId,
    retry: 1,
  });

  const disableMutation = useMutation({
    mutationFn: disableAccountSource,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accountSources'] });
      queryClient.invalidateQueries({ queryKey: ['accountSource', sourceId] });
    },
  });

  const validateMutation = useMutation({
    mutationFn: validateAccountSource,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accountSources'] });
      queryClient.invalidateQueries({ queryKey: ['accountSource', sourceId] });
    },
  });

  if (!sourceId) return null;

  return (
    <>
      <div className={styles.overlay} onClick={onClose} />
      <div className={styles.drawer}>
        <div className={styles.header}>
          <h3 className={styles.title}>来源详情</h3>
          <button className={styles.closeButton} onClick={onClose} aria-label="关闭">
            &times;
          </button>
        </div>

        <div className={styles.body}>
          {isLoading && <div className={styles.loading}>加载中...</div>}

          {error && (
            <div className={styles.error}>
              {error instanceof Error ? error.message : '加载失败'}
            </div>
          )}

          {source && (
            <>
              <div className={styles.section}>
                <span className={styles.sectionTitle}>基本信息</span>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>名称</span>
                  <span className={styles.fieldValue}>{source.name}</span>
                </div>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>ID</span>
                  <span className={styles.fieldValue} style={{ fontSize: 12 }}>
                    {source.id}
                  </span>
                </div>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>类型</span>
                  <span className={styles.fieldValue}>
                    <span className={`${styles.typeBadge} ${getTypeBadgeClass(source.source_type)}`}>
                      {TYPE_LABELS[source.source_type] || source.source_type}
                    </span>
                  </span>
                </div>
                {source.provider_id && (
                  <div className={styles.fieldRow}>
                    <span className={styles.fieldLabel}>供应商</span>
                    <span className={styles.fieldValue}>{source.provider_id}</span>
                  </div>
                )}
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>状态</span>
                  <span className={styles.fieldValue}>{source.status}</span>
                </div>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>健康状态</span>
                  <span className={styles.fieldValue}>
                    <span className={`${styles.healthDot} ${getHealthDotClass(source.health_state)}`} />
                    {source.health_state}
                  </span>
                </div>
              </div>

              <div className={styles.section}>
                <span className={styles.sectionTitle}>配置参数</span>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>优先级</span>
                  <span className={styles.fieldValue}>{source.priority}</span>
                </div>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>权重</span>
                  <span className={styles.fieldValue}>{source.weight}</span>
                </div>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>最大并发</span>
                  <span className={styles.fieldValue}>{source.max_concurrency}</span>
                </div>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>日预算 (微美元)</span>
                  <span className={styles.fieldValue}>
                    {source.daily_budget_micro_usd > 0
                      ? `$${(source.daily_budget_micro_usd / 1_000_000).toFixed(4)}`
                      : '不限制'}
                  </span>
                </div>
              </div>

              {source.source_type === 'subscription_pool' && (
                <div className={styles.section}>
                  <span className={styles.sectionTitle}>订阅账号统计</span>
                  <div className={styles.poolStats}>
                    <div className={styles.poolStatCard}>
                      <div className={`${styles.poolStatNumber} ${styles.availableColor}`}>
                        {source.subscription_accounts_count}
                      </div>
                      <div className={styles.poolStatLabel}>总计</div>
                    </div>
                    <div className={styles.poolStatCard}>
                      <div className={`${styles.poolStatNumber} ${styles.cooldownColor}`}>-</div>
                      <div className={styles.poolStatLabel}>冷却中</div>
                    </div>
                    <div className={styles.poolStatCard}>
                      <div className={`${styles.poolStatNumber} ${styles.deadColor}`}>-</div>
                      <div className={styles.poolStatLabel}>已失效</div>
                    </div>
                  </div>
                </div>
              )}

              <div className={styles.section}>
                <span className={styles.sectionTitle}>时间戳</span>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>最后验证</span>
                  <span className={styles.fieldValue}>
                    {formatDateTime(source.last_validated_at)}
                  </span>
                </div>
                <div className={styles.fieldRow}>
                  <span className={styles.fieldLabel}>创建时间</span>
                  <span className={styles.fieldValue}>
                    {formatDateTime(source.created_at)}
                  </span>
                </div>
              </div>

              <div className={styles.actions}>
                <button
                  className={styles.actionButton}
                  onClick={() => validateMutation.mutate(source.id)}
                  disabled={validateMutation.isPending}
                >
                  {validateMutation.isPending ? '验证中...' : '立即验证'}
                </button>
                <button
                  className={styles.dangerButton}
                  onClick={() => {
                    if (window.confirm(`确定要禁用来源「${source.name}」吗？`)) {
                      disableMutation.mutate(source.id);
                    }
                  }}
                  disabled={disableMutation.isPending}
                >
                  {disableMutation.isPending ? '禁用中...' : '禁用来源'}
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </>
  );
}

export default SourceDetailDrawer;
