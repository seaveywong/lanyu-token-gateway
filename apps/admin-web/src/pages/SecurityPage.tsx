import { useState, useEffect, useCallback } from 'react';
import PageHeader from '@/components/PageHeader';
import { apiClient } from '@lanyu/web-shared/api/client';

interface AuditLog {
  id: string;
  organization_id?: string;
  actor_id?: string;
  action: string;
  resource_type?: string;
  resource_id?: string;
  metadata: string;
  ip_address?: string;
  user_agent?: string;
  trace_id?: string;
  created_at: string;
}

function SecurityPage() {
  const [tab, setTab] = useState<'audit' | 'events' | 'features' | 'ip'>('audit');
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [actionFilter, setActionFilter] = useState('');
  const [actorFilter, setActorFilter] = useState('');

  // Feature flags (mock for now)
  const [featureFlags] = useState([
    { key: 'sso_enabled', label: 'SSO 单点登录', enabled: false },
    { key: 'mfa_required', label: '强制 MFA', enabled: true },
    { key: 'approval_flow', label: '四眼审批流', enabled: true },
    { key: 'audit_export', label: '审计日志导出', enabled: true },
    { key: 'rate_limit_strict', label: '严格速率限制', enabled: false },
    { key: 'ip_whitelist', label: 'IP 白名单', enabled: false },
  ]);

  // IP list (mock)
  const [ipEntries] = useState([
    { type: 'whitelist', ip: '10.0.0.0/8', note: '内网办公网络', addedAt: '2026-01-15' },
    { type: 'whitelist', ip: '203.0.113.0/24', note: 'VPN 出口', addedAt: '2026-03-01' },
    { type: 'blacklist', ip: '198.51.100.42', note: '多次暴力破解', addedAt: '2026-05-20' },
  ]);

  // Mock security events
  const securityEvents = [
    { id: '1', type: 'abnormal_login', severity: 'medium', detail: 'admin@example.com 从新 IP 192.0.2.100 登录', time: '2026-06-22 14:30:00' },
    { id: '2', type: 'key_leak_risk', severity: 'high', detail: 'API Key ly_live_abc123 在公开仓库中被发现', time: '2026-06-22 10:15:00' },
    { id: '3', type: 'rate_limit_hit', severity: 'low', detail: 'Org demo-org 5 分钟内触发 50 次限流', time: '2026-06-22 08:00:00' },
    { id: '4', type: 'permission_change', severity: 'medium', detail: '用户 operator@example.com 被提升为 platform_admin', time: '2026-06-21 16:45:00' },
  ];

  const fetchAuditLogs = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (actionFilter) params.set('action', actionFilter);
      const data = await apiClient<{ data: AuditLog[] }>(`/admin-api/audit-logs?${params}`);
      setLogs(data.data ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载审计日志失败');
    } finally {
      setLoading(false);
    }
  }, [actionFilter]);

  useEffect(() => {
    if (tab === 'audit') {
      fetchAuditLogs();
    }
  }, [tab, fetchAuditLogs]);

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleString('zh-CN');
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'high': return '#c62828';
      case 'medium': return '#e65100';
      case 'low': return '#2e7d32';
      default: return '#666';
    }
  };

  return (
    <div>
      <PageHeader
        title="运营安全"
        breadcrumbs={[{ label: '运营安全' }]}
      />

      <div style={{ display: 'flex', gap: 0, marginBottom: 24 }}>
        {[
          { key: 'audit' as const, label: '审计日志' },
          { key: 'events' as const, label: '安全事件' },
          { key: 'features' as const, label: 'Feature Flag' },
          { key: 'ip' as const, label: 'IP 黑白名单' },
        ].map((item, i) => (
          <button
            key={item.key}
            onClick={() => setTab(item.key)}
            style={{
              padding: '8px 20px',
              border: '1px solid #e0e0e0',
              borderLeft: i > 0 ? 'none' : undefined,
              background: tab === item.key ? '#1a73e8' : '#fff',
              color: tab === item.key ? '#fff' : '#333',
              cursor: 'pointer',
              borderRadius: i === 0 ? '6px 0 0 6px' : i === 3 ? '0 6px 6px 0' : '0',
              fontWeight: 500,
            }}
          >
            {item.label}
          </button>
        ))}
      </div>

      {/* Audit Logs Tab */}
      {tab === 'audit' && (
        <div>
          <div style={{ display: 'flex', gap: 12, marginBottom: 16, flexWrap: 'wrap', alignItems: 'center' }}>
            <input
              type="date"
              value={dateFrom}
              onChange={(e) => setDateFrom(e.target.value)}
              style={filterInputStyle}
              placeholder="开始日期"
            />
            <input
              type="date"
              value={dateTo}
              onChange={(e) => setDateTo(e.target.value)}
              style={filterInputStyle}
              placeholder="结束日期"
            />
            <select
              value={actionFilter}
              onChange={(e) => setActionFilter(e.target.value)}
              style={filterInputStyle}
            >
              <option value="">全部操作类型</option>
              <option value="user.login">用户登录</option>
              <option value="user.register">用户注册</option>
              <option value="api_key.create">创建 API Key</option>
              <option value="api_key.revoke">吊销 API Key</option>
              <option value="approval.create">创建审批</option>
              <option value="approval.approve">通过审批</option>
              <option value="approval.reject">驳回审批</option>
              <option value="payment.refund">退款</option>
            </select>
            <input
              type="text"
              value={actorFilter}
              onChange={(e) => setActorFilter(e.target.value)}
              style={{ ...filterInputStyle, width: 160 }}
              placeholder="操作者 ID"
            />
            <button onClick={fetchAuditLogs} style={searchBtnStyle}>搜索</button>
          </div>

          {error && (
            <div style={{ padding: 12, background: '#fee', color: '#c00', borderRadius: 6, marginBottom: 16 }}>
              {error}
            </div>
          )}

          {loading ? (
            <div style={{ textAlign: 'center', padding: 40, color: '#666' }}>加载中...</div>
          ) : logs.length === 0 ? (
            <div style={{ textAlign: 'center', padding: 60, background: '#fafafa', borderRadius: 8, color: '#999' }}>
              暂无审计日志
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={tableStyle}>
                <thead>
                  <tr style={{ background: '#f5f7fa', borderBottom: '2px solid #e0e0e0' }}>
                    <th style={thStyle}>时间</th>
                    <th style={thStyle}>操作者</th>
                    <th style={thStyle}>操作</th>
                    <th style={thStyle}>资源</th>
                    <th style={thStyle}>IP</th>
                    <th style={thStyle}>状态</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map((log) => (
                    <tr key={log.id} style={{ borderBottom: '1px solid #eee' }}>
                      <td style={tdStyle}>{formatDate(log.created_at)}</td>
                      <td style={tdStyle}>
                        <code style={{ fontSize: 12, background: '#f0f0f0', padding: '2px 6px', borderRadius: 3 }}>
                          {log.actor_id ? log.actor_id.substring(0, 8) + '...' : '-'}
                        </code>
                      </td>
                      <td style={tdStyle}>{log.action}</td>
                      <td style={tdStyle}>
                        {log.resource_type && (
                          <span style={{ fontSize: 13 }}>
                            {log.resource_type}: {log.resource_id ? log.resource_id.substring(0, 8) + '...' : ''}
                          </span>
                        )}
                      </td>
                      <td style={tdStyle}>{log.ip_address ?? '-'}</td>
                      <td style={tdStyle}>
                        <span style={{
                          padding: '2px 10px',
                          borderRadius: 12,
                          fontSize: 13,
                          background: log.action.includes('fail') ? '#ffebee' : '#e8f5e9',
                          color: log.action.includes('fail') ? '#c62828' : '#2e7d32',
                        }}>
                          {log.action.includes('fail') ? '失败' : '成功'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Security Events Tab */}
      {tab === 'events' && (
        <div>
          <table style={tableStyle}>
            <thead>
              <tr style={{ background: '#f5f7fa', borderBottom: '2px solid #e0e0e0' }}>
                <th style={thStyle}>时间</th>
                <th style={thStyle}>类型</th>
                <th style={thStyle}>严重程度</th>
                <th style={thStyle}>详情</th>
              </tr>
            </thead>
            <tbody>
              {securityEvents.map((event) => (
                <tr key={event.id} style={{ borderBottom: '1px solid #eee' }}>
                  <td style={tdStyle}>{event.time}</td>
                  <td style={tdStyle}>
                    {event.type === 'abnormal_login' && '异常登录'}
                    {event.type === 'key_leak_risk' && '密钥泄露风险'}
                    {event.type === 'rate_limit_hit' && '限流触发'}
                    {event.type === 'permission_change' && '权限变更'}
                  </td>
                  <td style={tdStyle}>
                    <span style={{
                      padding: '2px 10px',
                      borderRadius: 12,
                      fontSize: 13,
                      fontWeight: 500,
                      background:
                        event.severity === 'high' ? '#ffebee' :
                        event.severity === 'medium' ? '#fff3e0' : '#e8f5e9',
                      color: getSeverityColor(event.severity),
                    }}>
                      {event.severity === 'high' ? '高' : event.severity === 'medium' ? '中' : '低'}
                    </span>
                  </td>
                  <td style={tdStyle}>{event.detail}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Feature Flags Tab */}
      {tab === 'features' && (
        <div>
          <p style={{ color: '#888', marginBottom: 16, fontSize: 14 }}>
            功能开关管理 — 控制平台功能的启用/禁用
          </p>
          <table style={tableStyle}>
            <thead>
              <tr style={{ background: '#f5f7fa', borderBottom: '2px solid #e0e0e0' }}>
                <th style={thStyle}>开关 Key</th>
                <th style={thStyle}>说明</th>
                <th style={thStyle}>状态</th>
              </tr>
            </thead>
            <tbody>
              {featureFlags.map((flag) => (
                <tr key={flag.key} style={{ borderBottom: '1px solid #eee' }}>
                  <td style={tdStyle}>
                    <code style={{ fontSize: 12, background: '#f0f0f0', padding: '2px 6px', borderRadius: 3 }}>
                      {flag.key}
                    </code>
                  </td>
                  <td style={tdStyle}>{flag.label}</td>
                  <td style={tdStyle}>
                    <span style={{
                      padding: '2px 14px',
                      borderRadius: 12,
                      fontSize: 13,
                      fontWeight: 500,
                      background: flag.enabled ? '#e8f5e9' : '#f5f5f5',
                      color: flag.enabled ? '#2e7d32' : '#9e9e9e',
                    }}>
                      {flag.enabled ? '已启用' : '已禁用'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* IP Blacklist/Whitelist Tab */}
      {tab === 'ip' && (
        <div>
          <p style={{ color: '#888', marginBottom: 16, fontSize: 14 }}>
            IP 黑名单/白名单 — 控制来源 IP 访问权限
          </p>
          <table style={tableStyle}>
            <thead>
              <tr style={{ background: '#f5f7fa', borderBottom: '2px solid #e0e0e0' }}>
                <th style={thStyle}>类型</th>
                <th style={thStyle}>IP / CIDR</th>
                <th style={thStyle}>备注</th>
                <th style={thStyle}>添加时间</th>
              </tr>
            </thead>
            <tbody>
              {ipEntries.map((entry, i) => (
                <tr key={i} style={{ borderBottom: '1px solid #eee' }}>
                  <td style={tdStyle}>
                    <span style={{
                      padding: '2px 10px',
                      borderRadius: 12,
                      fontSize: 13,
                      fontWeight: 500,
                      background: entry.type === 'whitelist' ? '#e8f5e9' : '#ffebee',
                      color: entry.type === 'whitelist' ? '#2e7d32' : '#c62828',
                    }}>
                      {entry.type === 'whitelist' ? '白名单' : '黑名单'}
                    </span>
                  </td>
                  <td style={tdStyle}>
                    <code style={{ fontSize: 12, background: '#f0f0f0', padding: '2px 6px', borderRadius: 3 }}>
                      {entry.ip}
                    </code>
                  </td>
                  <td style={tdStyle}>{entry.note}</td>
                  <td style={tdStyle}>{entry.addedAt}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

const filterInputStyle: React.CSSProperties = {
  padding: '6px 12px',
  border: '1px solid #ddd',
  borderRadius: 6,
  fontSize: 13,
  outline: 'none',
};

const searchBtnStyle: React.CSSProperties = {
  padding: '6px 20px',
  border: 'none',
  borderRadius: 6,
  background: '#1a73e8',
  color: '#fff',
  cursor: 'pointer',
  fontSize: 13,
};

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  background: '#fff',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: '0 1px 4px rgba(0,0,0,0.06)',
};

const thStyle: React.CSSProperties = {
  padding: '12px 16px',
  textAlign: 'left',
  fontWeight: 600,
  fontSize: 13,
  color: '#555',
};

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontSize: 14,
  color: '#333',
};

export default SecurityPage;
