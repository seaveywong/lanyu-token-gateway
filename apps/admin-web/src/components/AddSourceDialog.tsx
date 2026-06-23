import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createAccountSource, type CreateAccountSourceParams } from '@/api/sources';
import styles from './AddSourceDialog.module.css';

interface AddSourceDialogProps {
  open: boolean;
  onClose: () => void;
}

type SourceTypeOption = 'official_api_key' | 'official_oauth' | 'upstream_api' | 'subscription_pool';

const SOURCE_TYPE_LABELS: Record<SourceTypeOption, string> = {
  official_api_key: '官方 API Key',
  official_oauth: '官方 OAuth',
  upstream_api: '上游 API',
  subscription_pool: '订阅池',
};

const SOURCE_TYPE_OPTIONS: SourceTypeOption[] = [
  'official_api_key',
  'official_oauth',
  'upstream_api',
  'subscription_pool',
];

const initialForm: CreateAccountSourceParams & { source_type: SourceTypeOption } = {
  name: '',
  source_type: 'official_api_key',
  provider_id: '',
  credential: '',
  priority: 10,
  weight: 1,
  max_concurrency: 5,
  daily_budget_micro_usd: 0,
};

function AddSourceDialog({ open, onClose }: AddSourceDialogProps) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState({ ...initialForm });
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: createAccountSource,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accountSources'] });
      onClose();
      resetForm();
    },
    onError: (err: Error) => {
      setError(err.message || '创建失败，请稍后重试');
    },
  });

  const resetForm = () => {
    setForm({ ...initialForm });
    setError(null);
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const setField = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    if (error) setError(null);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) {
      setError('请输入来源名称');
      return;
    }
    if (!form.credential.trim()) {
      setError('请输入凭证');
      return;
    }
    const payload: CreateAccountSourceParams = {
      name: form.name.trim(),
      source_type: form.source_type,
      credential: form.credential.trim(),
      priority: form.priority,
      weight: form.weight,
    };
    if (form.provider_id?.trim()) {
      payload.provider_id = form.provider_id.trim();
    }
    if (form.max_concurrency !== undefined) {
      payload.max_concurrency = form.max_concurrency;
    }
    if (form.daily_budget_micro_usd !== undefined) {
      payload.daily_budget_micro_usd = form.daily_budget_micro_usd;
    }
    mutation.mutate(payload);
  };

  if (!open) return null;

  return (
    <div className={styles.overlay} onClick={handleClose}>
      <div className={styles.dialog} onClick={(e) => e.stopPropagation()}>
        <div className={styles.header}>
          <h3 className={styles.title}>添加账号来源</h3>
          <button className={styles.closeButton} onClick={handleClose} aria-label="关闭">
            &times;
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          {error && <div className={styles.error}>{error}</div>}

          <div className={styles.field}>
            <label className={styles.label}>
              名称<span className={styles.required}>*</span>
            </label>
            <input
              className={styles.input}
              type="text"
              placeholder="例如：OpenAI 生产密钥"
              value={form.name}
              onChange={(e) => setField('name', e.target.value)}
            />
          </div>

          <div className={styles.field}>
            <label className={styles.label}>
              类型<span className={styles.required}>*</span>
            </label>
            <select
              className={styles.select}
              value={form.source_type}
              onChange={(e) => setField('source_type', e.target.value as SourceTypeOption)}
            >
              {SOURCE_TYPE_OPTIONS.map((opt) => (
                <option key={opt} value={opt}>
                  {SOURCE_TYPE_LABELS[opt]}
                </option>
              ))}
            </select>
          </div>

          <div className={styles.field}>
            <label className={styles.label}>供应商</label>
            <input
              className={styles.input}
              type="text"
              placeholder="例如：openai / azure"
              value={form.provider_id ?? ''}
              onChange={(e) => setField('provider_id', e.target.value)}
            />
          </div>

          <div className={styles.field}>
            <label className={styles.label}>
              凭证<span className={styles.required}>*</span>
            </label>
            <input
              className={styles.input}
              type="password"
              placeholder="API Key 或其他认证凭据"
              value={form.credential}
              onChange={(e) => setField('credential', e.target.value)}
            />
          </div>

          <div className={styles.row}>
            <div className={styles.field}>
              <label className={styles.label}>优先级</label>
              <input
                className={styles.input}
                type="number"
                min={0}
                max={100}
                value={form.priority}
                onChange={(e) => setField('priority', Number(e.target.value))}
              />
            </div>
            <div className={styles.field}>
              <label className={styles.label}>权重</label>
              <input
                className={styles.input}
                type="number"
                min={0}
                max={100}
                value={form.weight}
                onChange={(e) => setField('weight', Number(e.target.value))}
              />
            </div>
          </div>

          <div className={styles.row}>
            <div className={styles.field}>
              <label className={styles.label}>最大并发</label>
              <input
                className={styles.input}
                type="number"
                min={1}
                max={1000}
                value={form.max_concurrency ?? 5}
                onChange={(e) => setField('max_concurrency', Number(e.target.value))}
              />
            </div>
            <div className={styles.field}>
              <label className={styles.label}>日预算 (微美元)</label>
              <input
                className={styles.input}
                type="number"
                min={0}
                placeholder="0 表示不限制"
                value={form.daily_budget_micro_usd ?? 0}
                onChange={(e) => setField('daily_budget_micro_usd', Number(e.target.value))}
              />
            </div>
          </div>

          <div className={styles.actions}>
            <button type="button" className={styles.cancelButton} onClick={handleClose}>
              取消
            </button>
            <button type="submit" className={styles.submitButton} disabled={mutation.isPending}>
              {mutation.isPending ? '提交中...' : '确认添加'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default AddSourceDialog;
