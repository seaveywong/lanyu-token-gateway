import { useState, useCallback } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import PageHeader from '@/components/PageHeader';
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

async function listAllTickets(params?: {
  page?: number;
  page_size?: number;
  status?: string;
  priority?: string;
}): Promise<TicketListResponse> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set('page', String(params.page));
  if (params?.page_size) qs.set('page_size', String(params.page_size));
  if (params?.status) qs.set('status', params.status);
  if (params?.priority) qs.set('priority', params.priority);
  const query = qs.toString();
  return apiClient<TicketListResponse>(`/admin-api/tickets${query ? '?' + query : ''}`);
}

async function getTicket(id: string): Promise<TicketDetailResponse> {
  return apiClient<TicketDetailResponse>(`/admin-api/tickets/${id}`);
}

async function updateTicket(id: string, data: {
  status?: string;
  priority?: string;
  assigned_to?: string;
}): Promise<SupportTicket> {
  return apiClient<SupportTicket>(`/admin-api/tickets/${id}`, {
    method: 'PATCH',
    body: data,
  });
}

async function addMessage(ticketId: string, body: string, isInternal: boolean): Promise<TicketMessage> {
  return apiClient<TicketMessage>(`/admin-api/tickets/${ticketId}/messages`, {
    method: 'POST',
    body: { body, is_internal: isInternal },
  });
}

// ---- Helpers ----

const STATUS_LABELS: Record<string, string> = {
  open: '待处理',
  in_progress: '处理中',
  waiting: '等待回复',
  resolved: '已解决',
  closed: '已关闭',
};

const PRIORITY_LABELS: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  urgent: '紧急',
};

function getStatusClass(status: string): string {
  switch (status) {
    case 'open':
      return styles.statusOpen;
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

function getPriorityClass(priority: string): string {
  switch (priority) {
    case 'urgent':
      return styles.priorityUrgent;
    case 'high':
      return styles.priorityHigh;
    default:
      return styles.priorityNormal;
  }
}

// ---- TicketDetail sub-component ----

function TicketDetail({ ticketId, onBack }: { ticketId: string; onBack: () => void }) {
  const queryClient = useQueryClient();
  const [replyBody, setReplyBody] = useState('');
  const [replyInternal, setReplyInternal] = useState(false);
  const [sending, setSending] = useState(false);
  const [editingStatus, setEditingStatus] = useState(false);
  const [newStatus, setNewStatus] = useState('');
  const [editingPriority, setEditingPriority] = useState(false);
  const [newPriority, setNewPriority] = useState('');

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['adminTicket', ticketId],
    queryFn: () => getTicket(ticketId),
  });

  const updateMutation = useMutation({
    mutationFn: (params: { status?: string; priority?: string }) =>
      updateTicket(ticketId, params),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['adminTicket', ticketId] });
      queryClient.invalidateQueries({ queryKey: ['adminTickets'] });
      setEditingStatus(false);
      setEditingPriority(false);
    },
  });

  const handleSend = useCallback(async () => {
    if (!replyBody.trim()) return;
    setSending(true);
    try {
      await addMessage(ticketId, replyBody.trim(), replyInternal);
      setReplyBody('');
      queryClient.invalidateQueries({ queryKey: ['adminTicket', ticketId] });
    } catch (e) {
      // handled
    } finally {
      setSending(false);
    }
  }, [replyBody, replyInternal, ticketId, queryClient]);

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
        &larr; 返回队列
      </button>

      <div className={styles.ticketDetailHeader}>
        <div className={styles.ticketDetailInfo}>
          <h2 className={styles.ticketDetailSubject}>{ticket.subject}</h2>
          <div className={styles.ticketDetailMeta}>
            <span className={`${styles.statusBadge} ${getStatusClass(ticket.status)}`}>
              {STATUS_LABELS[ticket.status] || ticket.status}
            </span>
            <span className={`${styles.priorityBadge} ${getPriorityClass(ticket.priority)}`}>
              {PRIORITY_LABELS[ticket.priority] || ticket.priority}
            </span>
            <span className={styles.ticketOrgId}>组织: {ticket.organization_id}</span>
            <span className={styles.ticketDate}>
              {new Date(ticket.created_at).toLocaleString('zh-CN')}
            </span>
          </div>
        </div>

        {/* Admin actions */}
        <div className={styles.adminActions}>
          <div className={styles.actionGroup}>
            <span className={styles.actionLabel}>状态:</span>
            {editingStatus ? (
              <select
                className={styles.inlineSelect}
                value={newStatus}
                onChange={(e) => setNewStatus(e.target.value)}
              >
                <option value="">-- 选择 --</option>
                <option value="open">待处理</option>
                <option value="in_progress">处理中</option>
                <option value="waiting">等待回复</option>
                <option value="resolved">已解决</option>
                <option value="closed">已关闭</option>
              </select>
            ) : (
              <span className={styles.actionValue}>{STATUS_LABELS[ticket.status]}</span>
            )}
            {editingStatus ? (
              <>
                <button
                  className={styles.confirmBtn}
                  onClick={() => newStatus && updateMutation.mutate({ status: newStatus })}
                  disabled={!newStatus}
                >
                  确认
                </button>
                <button className={styles.cancelBtn} onClick={() => setEditingStatus(false)}>
                  取消
                </button>
              </>
            ) : (
              <button
                className={styles.editBtn}
                onClick={() => { setEditingStatus(true); setNewStatus(ticket.status); }}
              >
                修改
              </button>
            )}
          </div>

          <div className={styles.actionGroup}>
            <span className={styles.actionLabel}>优先级:</span>
            {editingPriority ? (
              <select
                className={styles.inlineSelect}
                value={newPriority}
                onChange={(e) => setNewPriority(e.target.value)}
              >
                <option value="">-- 选择 --</option>
                <option value="low">低</option>
                <option value="medium">中</option>
                <option value="high">高</option>
                <option value="urgent">紧急</option>
              </select>
            ) : (
              <span className={styles.actionValue}>
                {PRIORITY_LABELS[ticket.priority] || ticket.priority}
              </span>
            )}
            {editingPriority ? (
              <>
                <button
                  className={styles.confirmBtn}
                  onClick={() => newPriority && updateMutation.mutate({ priority: newPriority })}
                  disabled={!newPriority}
                >
                  确认
                </button>
                <button className={styles.cancelBtn} onClick={() => setEditingPriority(false)}>
                  取消
                </button>
              </>
            ) : (
              <button
                className={styles.editBtn}
                onClick={() => { setEditingPriority(true); setNewPriority(ticket.priority); }}
              >
                修改
              </button>
            )}
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
              <span className={styles.messageSender}>用户 {msg.sender_id.slice(0, 8)}</span>
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
          <div className={styles.replyToolbar}>
            <label className={styles.checkboxLabel}>
              <input
                type="checkbox"
                checked={replyInternal}
                onChange={(e) => setReplyInternal(e.target.checked)}
              />
              <span>内部备注（客户不可见）</span>
            </label>
          </div>
          <textarea
            className={styles.replyTextarea}
            placeholder={replyInternal ? '输入内部备注...' : '输入公开回复...'}
            value={replyBody}
            onChange={(e) => setReplyBody(e.target.value)}
            rows={4}
          />
          <button
            className={styles.sendBtn}
            onClick={handleSend}
            disabled={sending || !replyBody.trim()}
          >
            {sending ? '发送中...' : replyInternal ? '添加内部备注' : '发送回复'}
          </button>
        </div>
      )}
    </div>
  );
}

// ---- Main SupportPage ----

function SupportPage() {
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState('');
  const [priorityFilter, setPriorityFilter] = useState('');
  const [selectedTicketId, setSelectedTicketId] = useState<string | null>(null);
  const pageSize = 20;

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['adminTickets', page, statusFilter, priorityFilter],
    queryFn: () =>
      listAllTickets({
        page,
        page_size: pageSize,
        status: statusFilter || undefined,
        priority: priorityFilter || undefined,
      }),
  });

  // If viewing a ticket detail
  if (selectedTicketId) {
    return (
      <div className={styles.page}>
        <PageHeader title="客服工单" breadcrumbs={[{ label: '客服工单' }]} />
        <TicketDetail ticketId={selectedTicketId} onBack={() => setSelectedTicketId(null)} />
      </div>
    );
  }

  const tickets: SupportTicket[] = data?.data ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className={styles.page}>
      <PageHeader title="客服工单" breadcrumbs={[{ label: '客服工单' }]} />

      {/* Toolbar */}
      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <select
            className={styles.filterSelect}
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          >
            <option value="">全部状态</option>
            <option value="open">待处理</option>
            <option value="in_progress">处理中</option>
            <option value="waiting">等待回复</option>
            <option value="resolved">已解决</option>
            <option value="closed">已关闭</option>
          </select>
          <select
            className={styles.filterSelect}
            value={priorityFilter}
            onChange={(e) => { setPriorityFilter(e.target.value); setPage(1); }}
          >
            <option value="">全部优先级</option>
            <option value="low">低</option>
            <option value="medium">中</option>
            <option value="high">高</option>
            <option value="urgent">紧急</option>
          </select>
        </div>
        <span className={styles.ticketCount}>
          共 {total} 个工单
        </span>
      </div>

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
        <div className={styles.empty}>暂无工单</div>
      )}

      {/* Ticket queue */}
      {!isLoading && tickets.length > 0 && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>主题</th>
                <th>组织</th>
                <th>状态</th>
                <th>优先级</th>
                <th>负责人</th>
                <th>创建时间</th>
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
                  <td className={styles.tableId}>{ticket.organization_id.slice(0, 8)}...</td>
                  <td>
                    <span className={`${styles.statusBadge} ${getStatusClass(ticket.status)}`}>
                      {STATUS_LABELS[ticket.status] || ticket.status}
                    </span>
                  </td>
                  <td>
                    <span className={`${styles.priorityBadge} ${getPriorityClass(ticket.priority)}`}>
                      {PRIORITY_LABELS[ticket.priority] || ticket.priority}
                    </span>
                  </td>
                  <td className={styles.tableId}>
                    {ticket.assigned_to ? ticket.assigned_to.slice(0, 8) + '...' : '未分配'}
                  </td>
                  <td className={styles.tableTime}>
                    {new Date(ticket.created_at).toLocaleString('zh-CN')}
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
