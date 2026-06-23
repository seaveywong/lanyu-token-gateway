import PageHeader from '@/components/PageHeader';

function BillingPage() {
  return (
    <div>
      <PageHeader
        title="计费财务"
        breadcrumbs={[{ label: '计费财务' }]}
      />
      <p>此处查看计费记录、定价方案、账户充值和财务报表。</p>
    </div>
  );
}

export default BillingPage;
