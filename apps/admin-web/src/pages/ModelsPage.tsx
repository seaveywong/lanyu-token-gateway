import { useState, useCallback } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/api/client';
import PageHeader from '@/components/PageHeader';
import styles from './ModelsPage.module.css';

// ---- Types ----

interface ModelInfo {
  id: string;
  model_name: string;
  provider: string;
  modality: string;
  supports_streaming: boolean;
  supports_tools: boolean;
  supports_vision: boolean;
  max_input_tokens: number;
  max_output_tokens: number;
  status: string;
}

interface ModelListResponse {
  data: ModelInfo[];
  total: number;
}

interface CreateModelParams {
  model_name: string;
  provider: string;
  modality: string;
  supports_streaming?: boolean;
  supports_tools?: boolean;
  supports_vision?: boolean;
  max_input_tokens?: number;
  max_output_tokens?: number;
}

// ---- API ----

async function listModels(params?: { page?: number; page_size?: number; provider?: string }) {
  const query = new URLSearchParams();
  if (params?.page !== undefined) query.set('page', String(params.page));
  if (params?.page_size !== undefined) query.set('page_size', String(params.page_size));
  if (params?.provider) query.set('provider', params.provider);
  const qs = query.toString();
  return apiClient<ModelListResponse>(`/admin-api/models${qs ? '?' + qs : ''}`);
}

async function createModel(data: CreateModelParams) {
  return apiClient<ModelInfo>('/admin-api/models', { method: 'POST', body: data });
}

// ---- Toast ----

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

// ---- Format helpers ----

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
  return String(n);
}

function CapBadge({ yes }: { yes: boolean }) {
  return <span className={yes ? styles.capYes : styles.capNo}>{yes ? '是' : '否'}</span>;
}

// ---- Page ----

function ModelsPage() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [provider, setProvider] = useState('');
  const [showAddForm, setShowAddForm] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);

  // Add form state
  const [formModel, setFormModel] = useState('');
  const [formProvider, setFormProvider] = useState('');
  const [formModality, setFormModality] = useState('text');
  const [formStreaming, setFormStreaming] = useState(true);
  const [formTools, setFormTools] = useState(false);
  const [formVision, setFormVision] = useState(false);
  const [formMaxInput, setFormMaxInput] = useState('128000');
  const [formMaxOutput, setFormMaxOutput] = useState('4096');

  const showToast = useCallback((message: string, type: 'success' | 'error') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['models', { page, provider }],
    queryFn: () => listModels({ page, page_size: 30, provider: provider || undefined }),
  });

  const createMutation = useMutation({
    mutationFn: createModel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] });
      setShowAddForm(false);
      resetAddForm();
      showToast('模型已添加', 'success');
    },
    onError: (err: Error) => showToast(err.message || '添加失败', 'error'),
  });

  const resetAddForm = () => {
    setFormModel('');
    setFormProvider('');
    setFormModality('text');
    setFormStreaming(true);
    setFormTools(false);
    setFormVision(false);
    setFormMaxInput('128000');
    setFormMaxOutput('4096');
  };

  const handleAdd = () => {
    if (!formModel.trim() || !formProvider.trim()) {
      showToast('请填写模型名称和供应商', 'error');
      return;
    }
    createMutation.mutate({
      model_name: formModel.trim(),
      provider: formProvider.trim(),
      modality: formModality,
      supports_streaming: formStreaming,
      supports_tools: formTools,
      supports_vision: formVision,
      max_input_tokens: parseInt(formMaxInput, 10) || undefined,
      max_output_tokens: parseInt(formMaxOutput, 10) || undefined,
    });
  };

  const models: ModelInfo[] = data?.data ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.ceil(total / 30);

  return (
    <div className={styles.page}>
      <PageHeader
        title="模型管理"
        breadcrumbs={[{ label: '模型管理' }]}
      />

      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <select
            className={styles.filterSelect}
            value={provider}
            onChange={(e) => {
              setProvider(e.target.value);
              setPage(1);
            }}
          >
            <option value="">全部供应商</option>
            <option value="openai">OpenAI</option>
            <option value="anthropic">Anthropic</option>
            <option value="google">Google</option>
            <option value="azure">Azure</option>
            <option value="deepseek">DeepSeek</option>
          </select>
          <span style={{ fontSize: 13, color: '#6b7280' }}>
            {total} 个模型
          </span>
        </div>
        <button className={styles.addButton} onClick={() => setShowAddForm(true)}>
          + 添加模型
        </button>
      </div>

      {showAddForm && (
        <div style={{
          background: '#fff',
          border: '1px solid #e5e7eb',
          borderRadius: 8,
          padding: '14px 16px',
        }}>
          <div className={styles.inlineForm}>
            <input
              className={styles.inlineInput}
              placeholder="模型名 (如 gpt-4o)"
              value={formModel}
              onChange={(e) => setFormModel(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
            />
            <input
              className={styles.inlineInput}
              placeholder="供应商 (如 openai)"
              value={formProvider}
              onChange={(e) => setFormProvider(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
            />
            <select
              className={styles.filterSelect}
              value={formModality}
              onChange={(e) => setFormModality(e.target.value)}
              style={{ minWidth: 100 }}
            >
              <option value="text">text</option>
              <option value="image">image</option>
              <option value="audio">audio</option>
              <option value="multimodal">multimodal</option>
            </select>
            <input
              className={styles.inlineInput}
              placeholder="最大输入 (tokens)"
              value={formMaxInput}
              onChange={(e) => setFormMaxInput(e.target.value)}
              style={{ maxWidth: 100 }}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
            />
            <input
              className={styles.inlineInput}
              placeholder="最大输出"
              value={formMaxOutput}
              onChange={(e) => setFormMaxOutput(e.target.value)}
              style={{ maxWidth: 80 }}
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
            />
            <button
              className={styles.inlineConfirm}
              onClick={handleAdd}
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? '...' : '确认'}
            </button>
            <button className={styles.actionBtn} onClick={() => setShowAddForm(false)}>
              取消
            </button>
          </div>
          <div style={{ display: 'flex', gap: 12, marginTop: 8, flexWrap: 'wrap', fontSize: 13, color: '#6b7280' }}>
            <label style={{ cursor: 'pointer' }}>
              <input type="checkbox" checked={formStreaming} onChange={(e) => setFormStreaming(e.target.checked)} />
              {' '}流式
            </label>
            <label style={{ cursor: 'pointer' }}>
              <input type="checkbox" checked={formTools} onChange={(e) => setFormTools(e.target.checked)} />
              {' '}工具调用
            </label>
            <label style={{ cursor: 'pointer' }}>
              <input type="checkbox" checked={formVision} onChange={(e) => setFormVision(e.target.checked)} />
              {' '}视觉
            </label>
          </div>
        </div>
      )}

      {error && (
        <div className={styles.errorBanner}>
          <span>{(error as Error).message || '加载失败'}</span>
          <button className={styles.retryBtn} onClick={() => refetch()}>重试</button>
        </div>
      )}

      {isLoading && <div className={styles.loading}>加载中...</div>}

      {!isLoading && !error && models.length === 0 && (
        <div className={styles.empty}>
          {provider
            ? `暂无「${provider}」供应商的模型`
            : '暂无模型，后端 API 可能尚未就绪（501），请稍后刷新'}
        </div>
      )}

      {!isLoading && models.length > 0 && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>模型名</th>
                <th>供应商</th>
                <th>模态</th>
                <th>流式</th>
                <th>工具</th>
                <th>视觉</th>
                <th>最大输入</th>
                <th>最大输出</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              {models.map((m) => (
                <tr key={m.id}>
                  <td className={styles.tableName}>{m.model_name}</td>
                  <td>
                    <span className={styles.providerBadge}>{m.provider}</span>
                  </td>
                  <td>{m.modality}</td>
                  <td>
                    <div className={styles.caps}>
                      <CapBadge yes={m.supports_streaming} />
                    </div>
                  </td>
                  <td>
                    <div className={styles.caps}>
                      <CapBadge yes={m.supports_tools} />
                    </div>
                  </td>
                  <td>
                    <div className={styles.caps}>
                      <CapBadge yes={m.supports_vision} />
                    </div>
                  </td>
                  <td>{formatTokens(m.max_input_tokens)}</td>
                  <td>{formatTokens(m.max_output_tokens)}</td>
                  <td>{m.status === 'active' ? '活跃' : m.status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {totalPages > 1 && (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 8,
          padding: '8px 0',
        }}>
          <button
            className={styles.actionBtn}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1}
          >
            上一页
          </button>
          <span style={{ fontSize: 13, color: '#6b7280' }}>
            第 {page} / {totalPages} 页
          </span>
          <button
            className={styles.actionBtn}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
          >
            下一页
          </button>
        </div>
      )}

      <Toast toast={toast} />
    </div>
  );
}

export default ModelsPage;
