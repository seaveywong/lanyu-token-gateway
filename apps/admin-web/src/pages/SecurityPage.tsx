import PageHeader from '@/components/PageHeader';

function SecurityPage() {
  return (
    <div>
      <PageHeader
        title="运营安全"
        breadcrumbs={[{ label: '运营安全' }]}
      />
      <p>此处查看安全日志、异常访问告警、内容审核记录和风控配置。</p>
    </div>
  );
}

export default SecurityPage;
