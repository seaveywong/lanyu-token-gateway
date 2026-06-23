import { useState, useCallback } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/api/client';
import styles from './SupportPage.module.css';

// ---- Types ----

interface SupportTicket {
  id: string;
  organization_id: string;
  created_by: string;
  subject: string;
  status: 'open' | 'in_progress' | 'waiting' | 'resolved' | 'closed';
  priority: string;
  assigned_to: string | null;
  related_request_id: string | null;
  created_at: string;
  updated_at: string;
}

interface TicketMessage {
  id: string;
  ticket_id: string;
  sender_id: string;
  body: string;
  is_internal: boolean;
  created_at: string;
}

interface TicketListResponse {
  data: SupportTicket[];
  total: number;
  page: number;
  page_size: number;
}

interface TicketDetailResponse {
  ticket: SupportTicket;
  messages: TicketMessage[];
}

// ---- API functions ----

async function listTickets(params?: { page?: number; page_size?: number; status?: string }): Promise<TicketListResponse> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set('page', String(params.page));
  if (params?.page_size) qs.set('page_size', String(params.page_size));
  if (params?.status) qs.set('status', params.status);
  const query = qs.toString();
  return apiClient<TicketListResponse>(`/portal-api/tickets${query ? '?' + query : ''}`);
}

async function getTicket(id: string): Promise<TicketDetailResponse> {
  return apiClient<TicketDetailResponse>(`/portal-api/tickets/${id}`);
}

async function createTicket(data: { subject: string; body: string }): Promise<SupportTicket> {
  return apiClient<SupportTicket>('/portal-api/tickets', { method: 'POST', body: data });
}

async function addMessage(ticketId: string, body: string): Promise<TicketMessage> {
  return apiClient<TicketMessage>(`/portal-api/tickets/${ticketId}/messages`, {
    method: 'POST',
    body: { body },
  });
}

// ---- Helpers ----

const STATUS_LABELS: Record<string, string> = {
  open: '处理中',
  in_progress: '处理中',
  waiting: '等待回复',
  resolved: '已解决',
  closed: '已关闭',
};

function getStatusClass(status: string): string {
  switch (status) {
    case 'open':
    case 'in_progress':
      return styles.statusActive;
    case 'waiting':
      return styles.statusWaiting;
    case 'resolved':
      return styles.statusSuccess;
    case 'closed':
      return styles.statusClosed;
    default:
      return '';
  }
}

// ---- TicketDetail sub-component ----

function TicketDetail({ ticketId, onBack }: { ticketId: string; onBack: () => void }) {
  const queryClient = useQueryClient();
  const [replyBody, setReplyBody] = useState('');
  const [sending, setSending] = useState(false);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['ticket', ticketId],
    queryFn: () => getTicket(ticketId),
  });

  const handleSend = useCallback(async () => {
    if (!replyBody.trim()) return;
    setSending(true);
    try {
      await addMessage(ticketId, replyBody.trim());
      setReplyBody('');
      queryClient.invalidateQueries({ queryKey: ['ticket', ticketId] });
    } catch (e) {
      // handled below
    } finally {
      setSending(false);
    }
  }, [replyBody, ticketId, queryClient]);

  if (isLoading) return <div className={styles.loading}>加载工单...</div>;

  if (error) {
    return (
      <div className={styles.errorBanner}>
        <span>{(error as Error).message || '加载失败'}</span>
        <button className={styles.retryBtn} onClick={() => refetch()}>重试</button>
      </div>
    );
  }

  if (!data) return null;

  const { ticket, messages } = data;

  return (
    <div>
      <button className={styles.backBtn} onClick={onBack}>
        &larr; 返回列表
      </button>

      <div className={styles.ticketDetailHeader}>
        <div className={styles.ticketDetailInfo}>
          <h2 className={styles.ticketDetailSubject}>{ticket.subject}</h2>
          <div className={styles.ticketDetailMeta}>
            <span className={`${styles.statusBadge} ${getStatusClass(ticket.status)}`}>
              {STATUS_LABELS[ticket.status] || ticket.status}
            </span>
            <span className={styles.ticketDate}>
              {new Date(ticket.created_at).toLocaleString('zh-CN')}
            </span>
          </div>
        </div>
      </div>

      {/* Messages thread */}
      <div className={styles.messageThread}>
        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`${styles.messageBubble} ${msg.is_internal ? styles.messageInternal : ''}`}
          >
            <div className={styles.messageMeta}>
              {msg.is_internal && <span className={styles.internalBadge}>内部备注</span>}
              <span className={styles.messageTime}>
                {new Date(msg.created_at).toLocaleString('zh-CN')}
              </span>
            </div>
            <div className={styles.messageBody}>{msg.body}</div>
          </div>
        ))}

        {messages.length === 0 && (
          <div className={styles.empty}>暂无消息</div>
        )}
      </div>

      {/* Reply box */}
      {ticket.status !== 'closed' && (
        <div className={styles.replySection}>
          <textarea
            className={styles.replyTextarea}
            placeholder="输入回复内容..."
            value={replyBody}
            onChange={(e) => setReplyBody(e.target.value)}
            rows={4}
          />
          <button
            className={styles.sendBtn}
            onClick={handleSend}
            disabled={sending || !replyBody.trim()}
          >
            {sending ? '发送中...' : '发送'}
          </button>
        </div>
      )}
    </div>
  );
}

// ---- CreateTicketForm sub-component ----

function CreateTicketForm({ onCreated }: { onCreated: () => void }) {
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = useCallback(async () => {
    if (!subject.trim()) {
      setError('请输入工单主题');
      return;
    }
    if (!body.trim()) {
      setError('请输入问题描述');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      await createTicket({ subject: subject.trim(), body: body.trim() });
      setSubject('');
      setBody('');
      onCreated();
    } catch (e) {
      setError((e as Error).message || '创建失败');
    } finally {
      setSubmitting(false);
    }
  }, [subject, body, onCreated]);

  return (
    <div className={styles.createFormSection}>
      <h3 className={styles.sectionTitle}>创建工单</h3>

      {error && <div className={styles.formError}>{error}</div>}

      <div className={styles.formGroup}>
        <label className={styles.formLabel}>主题</label>
        <input
          className={styles.formInput}
          placeholder="简要描述您的问题"
          value={subject}
          onChange={(e) => setSubject(e.target.value)}
          maxLength={500}
        />
      </div>

      <div className={styles.formGroup}>
        <label className={styles.formLabel}>描述</label>
        <textarea
          className={styles.formTextarea}
          placeholder="请详细描述您遇到的问题..."
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={5}
        />
      </div>

      <button
        className={styles.submitBtn}
        onClick={handleSubmit}
        disabled={submitting}
      >
        {submitting ? '提交中...' : '提交工单'}
      </button>
    </div>
  );
}

// ---- Main SupportPage ----

function SupportPage() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState('');
  const [selectedTicketId, setSelectedTicketId] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const pageSize = 10;

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['tickets', page, statusFilter],
    queryFn: () => listTickets({ page, page_size: pageSize, status: statusFilter || undefined }),
  });

  const handleCreated = useCallback(() => {
    setShowCreateForm(false);
    queryClient.invalidateQueries({ queryKey: ['tickets'] });
  }, [queryClient]);

  // If viewing a ticket detail
  if (selectedTicketId) {
    return (
      <div className={styles.page}>
        <h1 className={styles.pageTitle}>支持</h1>
        <TicketDetail ticketId={selectedTicketId} onBack={() => setSelectedTicketId(null)} />
      </div>
    );
  }

  const tickets: SupportTicket[] = data?.data ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className={styles.page}>
      <h1 className={styles.pageTitle}>支持</h1>

      {/* Toolbar */}
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <select
            className={styles.filterSelect}
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          >
            <option value="">全部状态</option>
            <option value="open">处理中</option>
            <option value="waiting">等待回复</option>
            <option value="resolved">已解决</option>
            <option value="closed">已关闭</option>
          </select>
        </div>
        <button className={styles.addButton} onClick={() => setShowCreateForm(!showCreateForm)}>
          {showCreateForm ? '取消' : '+ 创建工单'}
        </button>
      </div>

      {/* Create form */}
      {showCreateForm && <CreateTicketForm onCreated={handleCreated} />}

      {/* Error */}
      {error && (
        <div className={styles.errorBanner}>
          <span>{(error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => refetch()}>重试</button>
        </div>
      )}

      {/* Loading */}
      {isLoading && <div className={styles.loading}>加载工单...</div>}

      {/* Empty */}
      {!isLoading && !error && tickets.length === 0 && (
        <div className={styles.empty}>
          {statusFilter ? '没有符合条件的工单' : '暂无工单，点击「创建工单」提交您的问题'}
        </div>
      )}

      {/* Ticket list */}
      {!isLoading && tickets.length > 0 && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>主题</th>
                <th>状态</th>
                <th>优先级</th>
                <th>创建时间</th>
                <th>更新时间</th>
              </tr>
            </thead>
            <tbody>
              {tickets.map((ticket) => (
                <tr
                  key={ticket.id}
                  className={styles.ticketRow}
                  onClick={() => setSelectedTicketId(ticket.id)}
                >
                  <td className={styles.tableName}>{ticket.subject}</td>
                  <td>
                    <span className={`${styles.statusBadge} ${getStatusClass(ticket.status)}`}>
                      {STATUS_LABELS[ticket.status] || ticket.status}
                    </span>
                  </td>
                  <td>{ticket.priority === 'urgent' ? '紧急' : ticket.priority === 'high' ? '高' : '普通'}</td>
                  <td className={styles.tableTime}>
                    {new Date(ticket.created_at).toLocaleString('zh-CN')}
                  </td>
                  <td className={styles.tableTime}>
                    {new Date(ticket.updated_at).toLocaleString('zh-CN')}
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
    </div>
  );
}

export default SupportPage;
