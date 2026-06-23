import PageHeader from '@/components/PageHeader';

function DashboardPage() {
  return (
    <div>
      <PageHeader
        title="数据概览"
        breadcrumbs={[{ label: '数据概览' }]}
      />
      <p>此处展示 Token 网关的核心运营指标：请求量、活跃用户数、模型调用分布、收入趋势等。</p>
    </div>
  );
}

export default DashboardPage;
