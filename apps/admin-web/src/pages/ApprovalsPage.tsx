import { useState, useEffect, useCallback } from 'react';
import PageHeader from '@/components/PageHeader';
import { apiClient } from '@lanyu/web-shared/api/client';

interface ApprovalRequest {
  id: string;
  organization_id: string;
  requester_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  payload: Record<string, unknown>;
  status: string;
  required_approvals: number;
  approved_by: string[];
  rejected_by?: string;
  rejection_reason?: string;
  expires_at: string;
  created_at: string;
  updated_at: string;
}

const ACTION_LABELS: Record<string, string> = {
  'payment.refund': '退款申请',
  'org.transfer': '组织转让',
  'key.export': '密钥导出',
  'source.disable': '渠道禁用',
};

const STATUS_LABELS: Record<string, string> = {
  pending: '待审批',
  approved: '已通过',
  rejected: '已驳回',
  cancelled: '已取消',
};


function ApprovalsPage() {
  const [tab, setTab] = useState<'pending' | 'history'>('pending');
  const [requests, setRequests] = useState<ApprovalRequest[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [rejectReason, setRejectReason] = useState('');
  const [rejectingId, setRejectingId] = useState<string | null>(null);

  const fetchRequests = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const endpoint = tab === 'pending'
        ? '/admin-api/approvals/pending'
        : '/admin-api/approvals/history';
      const data = await apiClient<{ data: ApprovalRequest[] }>(endpoint);
      setRequests(data.data ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [tab]);

  useEffect(() => {
    fetchRequests();
  }, [fetchRequests]);

  const handleApprove = async (id: string) => {
    try {
      await apiClient(`/admin-api/approvals/${id}/approve`, { method: 'POST' });
      fetchRequests();
    } catch (err) {
      alert(err instanceof Error ? err.message : '审批失败');
    }
  };

  const handleReject = async (id: string) => {
    if (!rejectReason.trim()) {
      alert('请填写驳回原因');
      return;
    }
    try {
      await apiClient(`/admin-api/approvals/${id}/reject`, {
        method: 'POST',
        body: JSON.stringify({ reason: rejectReason }),
      });
      setRejectingId(null);
      setRejectReason('');
      fetchRequests();
    } catch (err) {
      alert(err instanceof Error ? err.message : '驳回失败');
    }
  };

  const handleCancel = async (id: string) => {
    if (!confirm('确定要取消此审批请求？')) return;
    try {
      await apiClient(`/admin-api/approvals/${id}/cancel`, { method: 'POST' });
      fetchRequests();
    } catch (err) {
      alert(err instanceof Error ? err.message : '取消失败');
    }
  };

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleString('zh-CN');
  };

  return (
    <div>
      <PageHeader
        title="审批管理"
        breadcrumbs={[{ label: '审批管理' }]}
      />

      <div style={{ display: 'flex', gap: 0, marginBottom: 24 }}>
        <button
          onClick={() => setTab('pending')}
          style={{
            padding: '8px 20px',
            border: '1px solid #e0e0e0',
            background: tab === 'pending' ? '#1a73e8' : '#fff',
            color: tab === 'pending' ? '#fff' : '#333',
            cursor: 'pointer',
            borderRadius: '6px 0 0 6px',
            fontWeight: 500,
          }}
        >
          待审批
        </button>
        <button
          onClick={() => setTab('history')}
          style={{
            padding: '8px 20px',
            border: '1px solid #e0e0e0',
            borderLeft: 'none',
            background: tab === 'history' ? '#1a73e8' : '#fff',
            color: tab === 'history' ? '#fff' : '#333',
            cursor: 'pointer',
            borderRadius: '0 6px 6px 0',
            fontWeight: 500,
          }}
        >
          审批历史
        </button>
      </div>

      {error && (
        <div style={{ padding: 12, background: '#fee', color: '#c00', borderRadius: 6, marginBottom: 16 }}>
          {error}
          <button onClick={fetchRequests} style={{ marginLeft: 12, cursor: 'pointer' }}>重试</button>
        </div>
      )}

      {loading ? (
        <div style={{ textAlign: 'center', padding: 40, color: '#666' }}>加载中...</div>
      ) : requests.length === 0 ? (
        <div style={{
          textAlign: 'center',
          padding: 60,
          background: '#fafafa',
          borderRadius: 8,
          color: '#999',
        }}>
          {tab === 'pending' ? '暂无待审批事项' : '暂无审批记录'}
        </div>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', background: '#fff', borderRadius: 8, overflow: 'hidden', boxShadow: '0 1px 4px rgba(0,0,0,0.06)' }}>
          <thead>
            <tr style={{ background: '#f5f7fa', borderBottom: '2px solid #e0e0e0' }}>
              <th style={thStyle}>操作</th>
              <th style={thStyle}>资源类型</th>
              <th style={thStyle}>资源 ID</th>
              <th style={thStyle}>审批进度</th>
              <th style={thStyle}>状态</th>
              <th style={thStyle}>创建时间</th>
              <th style={thStyle}>过期时间</th>
              <th style={thStyle}>操作</th>
            </tr>
          </thead>
          <tbody>
            {requests.map((req) => (
              <tr key={req.id} style={{ borderBottom: '1px solid #eee' }}>
                <td style={tdStyle}>{ACTION_LABELS[req.action] ?? req.action}</td>
                <td style={tdStyle}>{req.resource_type}</td>
                <td style={tdStyle}>
                  <code style={{ fontSize: 12, background: '#f0f0f0', padding: '2px 6px', borderRadius: 3 }}>
                    {req.resource_id.substring(0, 8)}...
                  </code>
                </td>
                <td style={tdStyle}>
                  {req.approved_by.length} / {req.required_approvals}
                </td>
                <td style={tdStyle}>
                  <span style={{
                    padding: '2px 10px',
                    borderRadius: 12,
                    fontSize: 13,
                    fontWeight: 500,
                    background:
                      req.status === 'approved' ? '#e8f5e9' :
                      req.status === 'rejected' ? '#ffebee' :
                      req.status === 'cancelled' ? '#f5f5f5' : '#fff3e0',
                    color:
                      req.status === 'approved' ? '#2e7d32' :
                      req.status === 'rejected' ? '#c62828' :
                      req.status === 'cancelled' ? '#9e9e9e' : '#e65100',
                  }}>
                    {STATUS_LABELS[req.status] ?? req.status}
                  </span>
                </td>
                <td style={tdStyle}>{formatDate(req.created_at)}</td>
                <td style={tdStyle}>{formatDate(req.expires_at)}</td>
                <td style={tdStyle}>
                  {req.status === 'pending' && (
                    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                      <button
                        onClick={() => handleApprove(req.id)}
                        style={approveBtnStyle}
                      >
                        通过
                      </button>
                      {rejectingId === req.id ? (
                        <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                          <input
                            placeholder="驳回原因"
                            value={rejectReason}
                            onChange={(e) => setRejectReason(e.target.value)}
                            style={{ padding: '4px 8px', border: '1px solid #ccc', borderRadius: 4, width: 120 }}
                          />
                          <button onClick={() => handleReject(req.id)} style={confirmBtnStyle}>确认</button>
                          <button onClick={() => { setRejectingId(null); setRejectReason(''); }} style={cancelBtnStyle}>取消</button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setRejectingId(req.id)}
                          style={rejectBtnStyle}
                        >
                          驳回
                        </button>
                      )}
                      <button
                        onClick={() => handleCancel(req.id)}
                        style={cancelSmallBtnStyle}
                      >
                        取消
                      </button>
                    </div>
                  )}
                  {req.status === 'rejected' && req.rejection_reason && (
                    <div style={{ fontSize: 12, color: '#c62828', marginTop: 4 }}>
                      原因: {req.rejection_reason}
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

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

const approveBtnStyle: React.CSSProperties = {
  padding: '4px 14px',
  border: 'none',
  borderRadius: 4,
  background: '#1a73e8',
  color: '#fff',
  cursor: 'pointer',
  fontSize: 13,
};

const rejectBtnStyle: React.CSSProperties = {
  padding: '4px 14px',
  border: '1px solid #d32f2f',
  borderRadius: 4,
  background: '#fff',
  color: '#d32f2f',
  cursor: 'pointer',
  fontSize: 13,
};

const confirmBtnStyle: React.CSSProperties = {
  padding: '4px 10px',
  border: 'none',
  borderRadius: 4,
  background: '#d32f2f',
  color: '#fff',
  cursor: 'pointer',
  fontSize: 12,
};

const cancelBtnStyle: React.CSSProperties = {
  padding: '4px 10px',
  border: '1px solid #ccc',
  borderRadius: 4,
  background: '#fff',
  color: '#666',
  cursor: 'pointer',
  fontSize: 12,
};

const cancelSmallBtnStyle: React.CSSProperties = {
  padding: '4px 14px',
  border: '1px solid #bbb',
  borderRadius: 4,
  background: '#fff',
  color: '#888',
  cursor: 'pointer',
  fontSize: 13,
};

export default ApprovalsPage;
