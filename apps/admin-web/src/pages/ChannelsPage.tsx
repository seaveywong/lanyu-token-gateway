import PageHeader from '@/components/PageHeader';

function ChannelsPage() {
  return (
    <div>
      <PageHeader
        title="渠道管理"
        breadcrumbs={[{ label: '渠道管理' }]}
      />
      <p>此处管理上游模型渠道配置：API 端点、认证凭据、权重分配和健康检查。</p>
    </div>
  );
}

export default ChannelsPage;
