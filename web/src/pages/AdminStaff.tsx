import { EditOutlined, KeyOutlined, PlusOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Form, Input, Modal, Select, Skeleton, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData, putData, resetAdminStaffPassword } from '../services/http';
import { FormDrawer } from '../components/FormDrawer';
import { CardList, InfoCard, ListViewToggle, useListViewMode } from '../components/ListViews';
import type { AdminStaff, AdminStaffUpsertRequest, PasswordResetResult, Role } from '../types/starline';

type AdminStaffFormValues = {
  name: string;
  phone: string;
  role: Role;
  campusId?: string;
  remark: string;
  enabled: boolean;
};

const roleOptions = [
  { label: '运营教务', value: 'ops_staff' },
  { label: '校区管理员', value: 'campus_admin' },
  { label: '超级管理员', value: 'super_admin' }
];

const roleLabels: Record<string, string> = {
  ops_staff: '运营教务',
  campus_admin: '校区管理员',
  super_admin: '超级管理员'
};

export default function AdminStaff() {
  const [form] = Form.useForm<AdminStaffFormValues>();
  const [editing, setEditing] = useState<AdminStaff | null>(null);
  const [open, setOpen] = useState(false);
  const [resetResult, setResetResult] = useState<(PasswordResetResult & { name: string; phone: string }) | null>(null);
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:admin-staff');
  const role = Form.useWatch('role', form);
  const queryClient = useQueryClient();

  const staff = useQuery({ queryKey: ['admin-staff'], queryFn: () => getData<AdminStaff[]>('/admin-staff') });

  const saveStaff = useMutation({
    mutationFn: (values: AdminStaffFormValues) => {
      const body: AdminStaffUpsertRequest = {
        name: values.name,
        phone: values.phone,
        role: values.role,
        campusId: values.role === 'campus_admin' ? values.campusId : undefined,
        remark: values.remark ?? '',
        accountStatus: editing ? (values.enabled ? '正常' : '停用') : undefined
      };
      if (editing) return putData<AdminStaff>(`/admin-staff/${editing.id}`, body);
      return postData<AdminStaff>('/admin-staff', body);
    },
    onSuccess: (result) => {
      message.success(editing ? '管理人员信息已保存' : '管理人员已新增');
      if (result.temporaryPassword) {
        setResetResult({ userId: result.id, name: result.name, phone: result.phone, temporaryPassword: result.temporaryPassword, mustChangePassword: true });
      }
      setOpen(false);
      setEditing(null);
      queryClient.invalidateQueries({ queryKey: ['admin-staff'] });
    },
    onError: (error: any) => message.error(error.response?.data?.message || '保存失败，请检查姓名、手机号和岗位。')
  });

  const resetPassword = useMutation({
    mutationFn: (record: AdminStaff) => resetAdminStaffPassword(record.id),
    onSuccess: (result, record) => {
      setResetResult({ ...result, name: record.name, phone: record.phone });
      queryClient.invalidateQueries({ queryKey: ['admin-staff'] });
      message.success('临时密码已生成');
    },
    onError: (error: any) => message.error(error.response?.data?.message || '重置密码失败，请稍后重试。')
  });

  function confirmReset(record: AdminStaff) {
    Modal.confirm({
      title: '重置管理人员密码',
      content: `确认重置 ${record.name}（${record.phone}）的密码？旧密码和现有登录会话将失效，对方需用新临时密码登录并修改密码。`,
      okText: '确认重置',
      cancelText: '取消',
      onOk: () => resetPassword.mutateAsync(record)
    });
  }

  function openCreate() {
    setEditing(null);
    form.setFieldsValue({ name: '', phone: '', role: 'ops_staff', campusId: '', remark: '', enabled: true });
    setOpen(true);
  }

  function openEdit(record: AdminStaff) {
    setEditing(record);
    form.setFieldsValue({
      name: record.name,
      phone: record.phone,
      role: record.role,
      campusId: record.campusId,
      remark: record.remark,
      enabled: record.accountStatus === '正常'
    });
    setOpen(true);
  }

  if (staff.isLoading) return <Skeleton active />;
  if (staff.error) return <Alert type="error" message="管理人员加载失败，请稍后重试。" />;

  const rows = staff.data ?? [];

  return (
    <div className="page-stack admin-staff-page">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>管理人员</Typography.Title>
          <Typography.Text type="secondary">维护岗位、校区权限和账号状态。</Typography.Text>
        </div>
        <div className="page-heading-actions">
          <ListViewToggle storageKey="starline:list-view:admin-staff" value={viewMode} onChange={setViewMode} />
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增人员</Button>
        </div>
      </div>

      <Card>
        {viewMode === 'card' ? (
          <CardList
            rows={rows}
            rowKey={(record) => record.id}
            emptyText="还没有管理人员，先新增人员并设置岗位。"
            renderCard={(record) => (
              <InfoCard
                title={record.name}
                subtitle={record.phone}
                status={<Tag color={record.accountStatus === '正常' ? 'green' : 'default'}>{record.accountStatus}</Tag>}
                fields={[
                  { label: '岗位', value: <Tag color={roleColor(record.role)}>{roleLabels[record.role] ?? record.role}</Tag> },
                  { label: '校区', value: record.campusId || <Typography.Text type="secondary">全部校区</Typography.Text> },
                  { label: '微信绑定', value: <Tag color={record.bindStatus === '已绑定' ? 'green' : 'orange'}>{record.bindStatus}</Tag> },
                  { label: '密码登录', value: passwordFallbackTag(record) },
                  { label: '备注', value: record.remark || '-' }
                ]}
                actions={(
                  <>
                    <Button size="small" aria-label="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
                    <Button size="small" aria-label="重置密码" icon={<KeyOutlined />} disabled={record.accountStatus !== '正常'} loading={resetPassword.isPending && resetPassword.variables?.id === record.id} onClick={() => confirmReset(record)}>重置密码</Button>
                  </>
                )}
              />
            )}
          />
        ) : rows.length === 0 ? (
          <Empty description="还没有管理人员，先新增人员并设置岗位。" />
        ) : (
          <Table
            rowKey="id"
            dataSource={rows}
            pagination={false}
            scroll={{ x: 1250 }}
            columns={[
              { title: '姓名', dataIndex: 'name', width: 120 },
              { title: '手机号', dataIndex: 'phone', width: 140 },
              { title: '岗位', dataIndex: 'role', width: 120, render: (value: string) => <Tag color={roleColor(value)}>{roleLabels[value] ?? value}</Tag> },
              { title: '校区', dataIndex: 'campusId', width: 120, render: (value?: string) => value || <Typography.Text type="secondary">全部校区</Typography.Text> },
              { title: '微信绑定', dataIndex: 'bindStatus', width: 110, render: (value: string) => <Tag color={value === '已绑定' ? 'green' : 'orange'}>{value}</Tag> },
              { title: '密码登录', width: 130, render: (_, record) => passwordFallbackTag(record) },
              { title: '账号状态', dataIndex: 'accountStatus', width: 110, render: (value: string) => <Tag color={value === '正常' ? 'green' : 'default'}>{value}</Tag> },
              { title: '备注', dataIndex: 'remark', ellipsis: true },
              {
                title: '操作',
                width: 220,
                fixed: 'right',
                render: (_, record) => (
                  <Space size={4}>
                    <Button size="small" aria-label="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
                    <Button size="small" aria-label="重置密码" icon={<KeyOutlined />} disabled={record.accountStatus !== '正常'} loading={resetPassword.isPending && resetPassword.variables?.id === record.id} onClick={() => confirmReset(record)}>重置密码</Button>
                  </Space>
                )
              }
            ]}
          />
        )}
      </Card>

      <FormDrawer
        title={editing ? '编辑管理人员' : '新增管理人员'}
        open={open}
        onCancel={() => setOpen(false)}
        onSubmit={() => form.submit()}
        submitting={saveStaff.isPending}
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveStaff.mutate(values)}>
          {!editing && <Alert type="info" showIcon message="保存后生成临时密码" description="请复制登录信息交给对方，首次登录后需修改密码。" style={{ marginBottom: 16 }} />}
          <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
            <Input placeholder="例如：张老师" />
          </Form.Item>
          <Form.Item name="phone" label="手机号" rules={[{ required: true, message: '请输入手机号' }, { pattern: /^1\d{10}$/, message: '请输入正确的 11 位手机号' }]}>
            <Input placeholder="用于首次登录和身份确认" />
          </Form.Item>
          <Form.Item name="role" label="岗位" rules={[{ required: true, message: '请选择岗位' }]}>
            <Select options={roleOptions} />
          </Form.Item>
          {role === 'campus_admin' && (
            <Form.Item name="campusId" label="校区" rules={[{ required: true, message: '请输入校区' }]}>
              <Input placeholder="例如：campus-main" />
            </Form.Item>
          )}
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={3} placeholder="可填写岗位说明或交接备注" />
          </Form.Item>
          {editing && (
            <Form.Item name="enabled" label="启用账号" valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </FormDrawer>
      <Modal
        title="交接登录信息"
        open={Boolean(resetResult)}
        onCancel={() => setResetResult(null)}
        footer={<Button type="primary" onClick={() => setResetResult(null)}>我已记录</Button>}
        destroyOnHidden
      >
        <Typography.Paragraph>临时密码仅在此展示，请通过安全渠道交给本人。首次登录需修改密码；密码遗失时可重新生成。</Typography.Paragraph>
        <Typography.Paragraph>{resetResult?.name} · {resetResult?.phone}</Typography.Paragraph>
        <Typography.Paragraph>登录地址：<Typography.Link href={`${window.location.origin}/login`} target="_blank">{window.location.origin}/login</Typography.Link></Typography.Paragraph>
        <Typography.Paragraph>临时密码：<Typography.Text copyable strong>{resetResult?.temporaryPassword}</Typography.Text></Typography.Paragraph>
        <Typography.Paragraph copyable={{ text: `Starline 教务后台\n姓名：${resetResult?.name}\n登录地址：${window.location.origin}/login\n手机号：${resetResult?.phone}\n临时密码：${resetResult?.temporaryPassword}\n首次登录后请修改密码。` }}>复制完整登录信息</Typography.Paragraph>
      </Modal>
    </div>
  );
}

function roleColor(role: string) {
  if (role === 'super_admin') return 'red';
  if (role === 'campus_admin') return 'blue';
  return 'purple';
}

function passwordFallbackTag(record: AdminStaff) {
  if (record.accountStatus !== '正常') return <Tag>不可登录</Tag>;
  if (!record.passwordEnabled) return <Tag color="orange">需重置密码</Tag>;
  return record.mustChangePassword
    ? <Tag color="orange">首次登录需改密</Tag>
    : <Tag color="green">已设置密码</Tag>;
}
