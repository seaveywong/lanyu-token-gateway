import PageHeader from '@/components/PageHeader';

function SupportPage() {
  return (
    <div>
      <PageHeader
        title="客服工单"
        breadcrumbs={[{ label: '客服工单' }]}
      />
      <p>此处处理客户提交的工单：查看、回复、转派和关闭工单。</p>
    </div>
  );
}

export default SupportPage;
