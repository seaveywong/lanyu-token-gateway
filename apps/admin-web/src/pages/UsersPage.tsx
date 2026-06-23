import PageHeader from '@/components/PageHeader';

function UsersPage() {
  return (
    <div>
      <PageHeader
        title="用户与组织"
        breadcrumbs={[{ label: '用户与组织' }]}
      />
      <p>此处管理用户账号、组织（企业）信息、角色权限和账户余额。</p>
    </div>
  );
}

export default UsersPage;
