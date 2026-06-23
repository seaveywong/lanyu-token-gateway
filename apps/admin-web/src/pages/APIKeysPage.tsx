import PageHeader from '@/components/PageHeader';

function APIKeysPage() {
  return (
    <div>
      <PageHeader
        title="API 与模型"
        breadcrumbs={[{ label: 'API 与模型' }]}
      />
      <p>此处管理 API Key 的创建、启用/禁用、模型路由配置和限流策略。</p>
    </div>
  );
}

export default APIKeysPage;
