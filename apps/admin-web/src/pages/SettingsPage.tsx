import { useState } from 'react';
import PageHeader from '@/components/PageHeader';

interface WebhookEndpoint {
  id: string;
  url: string;
  events: string[];
  status: 'active' | 'inactive';
  createdAt: string;
  lastDelivery?: string;
}

function SettingsPage() {
  const [tab, setTab] = useState<'general' | 'key-rotation' | 'webhooks'>('general');

  // General settings (mock)
  const [platformName, setPlatformName] = useState('蓝域 Token Gateway');
  const [logLevel, setLogLevel] = useState('info');
  const [sessionTimeout, setSessionTimeout] = useState('30');

  // Key rotation
  const [lastJwtRotation, setLastJwtRotation] = useState('2026-06-01 00:00:00');
  const [lastPepperRotation, setLastPepperRotation] = useState('2026-05-15 00:00:00');

  // Webhooks (mock)
  const [webhooks] = useState<WebhookEndpoint[]>([
    {
      id: 'wh_001',
      url: 'https://hooks.example.com/token-events',
      events: ['payment.completed', 'payment.refunded', 'user.registered'],
      status: 'active',
      createdAt: '2026-04-10',
      lastDelivery: '2026-06-23 09:15:00',
    },
    {
      id: 'wh_002',
      url: 'https://api.partner.com/callbacks',
      events: ['api_key.revoked', 'source.disabled'],
      status: 'active',
      createdAt: '2026-05-20',
      lastDelivery: '2026-06-23 08:42:00',
    },
    {
      id: 'wh_003',
      url: 'https://monitoring.example.org/alerts',
      events: ['circuit.breaker.open', 'channel.unhealthy'],
      status: 'inactive',
      createdAt: '2026-03-01',
    },
  ]);

  const handleSaveGeneral = () => {
    alert('系统设置已保存（演示）');
  };

  const handleRotateJWT = () => {
    if (confirm('确定要轮换 JWT 签名密钥？所有现有会话将失效。')) {
      setLastJwtRotation(new Date().toLocaleString('zh-CN'));
      alert('JWT 密钥已轮换（演示）');
    }
  };

  const handleRotatePepper = () => {
    if (confirm('确定要轮换 API Key Pepper？所有现有 API Key 将需要重新生成。')) {
      setLastPepperRotation(new Date().toLocaleString('zh-CN'));
      alert('API Key Pepper 已轮换（演示）');
    }
  };

  return (
    <div>
      <PageHeader
        title="系统设置"
        breadcrumbs={[{ label: '系统设置' }]}
      />

      <div style={{ display: 'flex', gap: 0, marginBottom: 24 }}>
        {[
          { key: 'general' as const, label: '通用设置' },
          { key: 'key-rotation' as const, label: '密钥轮换' },
          { key: 'webhooks' as const, label: 'Webhook 管理' },
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
              borderRadius: i === 0 ? '6px 0 0 6px' : i === 2 ? '0 6px 6px 0' : '0',
              fontWeight: 500,
            }}
          >
            {item.label}
          </button>
        ))}
      </div>

      {/* General Settings Tab */}
      {tab === 'general' && (
        <div style={{
          background: '#fff',
          borderRadius: 8,
          padding: 24,
          boxShadow: '0 1px 4px rgba(0,0,0,0.06)',
          maxWidth: 520,
        }}>
          <div style={{ marginBottom: 20 }}>
            <label style={labelStyle}>平台名称</label>
            <input
              type="text"
              value={platformName}
              onChange={(e) => setPlatformName(e.target.value)}
              style={inputStyle}
              placeholder="Token Gateway"
            />
          </div>
          <div style={{ marginBottom: 20 }}>
            <label style={labelStyle}>日志级别</label>
            <select
              value={logLevel}
              onChange={(e) => setLogLevel(e.target.value)}
              style={inputStyle}
            >
              <option value="debug">Debug</option>
              <option value="info">Info</option>
              <option value="warn">Warn</option>
              <option value="error">Error</option>
            </select>
          </div>
          <div style={{ marginBottom: 24 }}>
            <label style={labelStyle}>会话超时 (分钟)</label>
            <input
              type="number"
              value={sessionTimeout}
              onChange={(e) => setSessionTimeout(e.target.value)}
              style={{ ...inputStyle, width: 120 }}
              min={5}
              max={1440}
            />
          </div>
          <button onClick={handleSaveGeneral} style={primaryBtnStyle}>
            保存设置
          </button>
        </div>
      )}

      {/* Key Rotation Tab */}
      {tab === 'key-rotation' && (
        <div style={{
          background: '#fff',
          borderRadius: 8,
          padding: 24,
          boxShadow: '0 1px 4px rgba(0,0,0,0.06)',
          maxWidth: 520,
        }}>
          <div style={{
            padding: 16,
            background: '#fff3e0',
            borderRadius: 8,
            marginBottom: 24,
            borderLeft: '4px solid #e65100',
          }}>
            <strong style={{ color: '#e65100' }}>注意:</strong> 密钥轮换将立即使现有凭证失效，
            请在计划维护窗口内执行此操作。
          </div>

          <div style={{
            padding: 20,
            border: '1px solid #e0e0e0',
            borderRadius: 8,
            marginBottom: 16,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}>
            <div>
              <h3 style={{ margin: 0, fontSize: 16 }}>JWT 签名密钥</h3>
              <p style={{ margin: '4px 0 0', fontSize: 13, color: '#888' }}>
                上次轮换: {lastJwtRotation}
              </p>
              <p style={{ margin: '4px 0 0', fontSize: 13, color: '#c62828' }}>
                影响: 所有用户会话将失效，需要重新登录
              </p>
            </div>
            <button onClick={handleRotateJWT} style={dangerBtnStyle}>
              轮换 JWT 密钥
            </button>
          </div>

          <div style={{
            padding: 20,
            border: '1px solid #e0e0e0',
            borderRadius: 8,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}>
            <div>
              <h3 style={{ margin: 0, fontSize: 16 }}>API Key Pepper</h3>
              <p style={{ margin: '4px 0 0', fontSize: 13, color: '#888' }}>
                上次轮换: {lastPepperRotation}
              </p>
              <p style={{ margin: '4px 0 0', fontSize: 13, color: '#c62828' }}>
                影响: 所有现有 API Key 将立即失效，用户需要重新生成
              </p>
            </div>
            <button onClick={handleRotatePepper} style={dangerBtnStyle}>
              轮换 Pepper
            </button>
          </div>
        </div>
      )}

      {/* Webhooks Tab */}
      {tab === 'webhooks' && (
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
            <p style={{ color: '#888', fontSize: 14, margin: 0 }}>
              Webhook 端点管理 — 配置事件通知的目标 URL
            </p>
            <button style={primaryBtnStyle}>添加端点</button>
          </div>

          {webhooks.length === 0 ? (
            <div style={{ textAlign: 'center', padding: 60, background: '#fafafa', borderRadius: 8, color: '#999' }}>
              暂无 Webhook 端点
            </div>
          ) : (
            <table style={tableStyle}>
              <thead>
                <tr style={{ background: '#f5f7fa', borderBottom: '2px solid #e0e0e0' }}>
                  <th style={thStyle}>URL</th>
                  <th style={thStyle}>订阅事件</th>
                  <th style={thStyle}>状态</th>
                  <th style={thStyle}>创建时间</th>
                  <th style={thStyle}>最后投递</th>
                </tr>
              </thead>
              <tbody>
                {webhooks.map((wh) => (
                  <tr key={wh.id} style={{ borderBottom: '1px solid #eee' }}>
                    <td style={tdStyle}>
                      <code style={{ fontSize: 12, wordBreak: 'break-all' }}>{wh.url}</code>
                    </td>
                    <td style={tdStyle}>
                      <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                        {wh.events.map((ev) => (
                          <span
                            key={ev}
                            style={{
                              padding: '2px 8px',
                              borderRadius: 10,
                              fontSize: 11,
                              background: '#e3f2fd',
                              color: '#1565c0',
                            }}
                          >
                            {ev}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td style={tdStyle}>
                      <span style={{
                        padding: '2px 10px',
                        borderRadius: 12,
                        fontSize: 13,
                        fontWeight: 500,
                        background: wh.status === 'active' ? '#e8f5e9' : '#f5f5f5',
                        color: wh.status === 'active' ? '#2e7d32' : '#9e9e9e',
                      }}>
                        {wh.status === 'active' ? '活跃' : '停用'}
                      </span>
                    </td>
                    <td style={tdStyle}>{wh.createdAt}</td>
                    <td style={tdStyle}>{wh.lastDelivery ?? '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
}

const labelStyle: React.CSSProperties = {
  display: 'block',
  marginBottom: 6,
  fontSize: 14,
  fontWeight: 500,
  color: '#333',
};

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '8px 12px',
  border: '1px solid #ddd',
  borderRadius: 6,
  fontSize: 14,
  outline: 'none',
};

const primaryBtnStyle: React.CSSProperties = {
  padding: '8px 24px',
  border: 'none',
  borderRadius: 6,
  background: '#1a73e8',
  color: '#fff',
  cursor: 'pointer',
  fontSize: 14,
  fontWeight: 500,
};

const dangerBtnStyle: React.CSSProperties = {
  padding: '8px 20px',
  border: '1px solid #d32f2f',
  borderRadius: 6,
  background: '#fff',
  color: '#d32f2f',
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
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

export default SettingsPage;
