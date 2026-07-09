import { EditOutlined, KeyOutlined, PlusOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Form, Input, Modal, Select, Skeleton, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData, putData, resetAdminStaffPassword } from '../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../components/ListViews';
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
  const [resetResult, setResetResult] = useState<PasswordResetResult | null>(null);
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
    onSuccess: () => {
      message.success(editing ? '管理人员信息已保存' : '管理人员已新增，等待首次登录确认身份');
      setOpen(false);
      setEditing(null);
      queryClient.invalidateQueries({ queryKey: ['admin-staff'] });
    },
    onError: (error: any) => message.error(error.response?.data?.message || '保存失败，请检查姓名、手机号和岗位。')
  });

  const resetPassword = useMutation({
    mutationFn: (record: AdminStaff) => resetAdminStaffPassword(record.id),
    onSuccess: (result) => {
      setResetResult(result);
      message.success('临时密码已生成');
    },
    onError: (error: any) => message.error(error.response?.data?.message || '重置密码失败，请稍后重试。')
  });

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
    <div className="page-stack">
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
                  { label: '登录方式', value: passwordFallbackTag(record.bindStatus) },
                  { label: '备注', value: record.remark || '-' }
                ]}
                actions={(
                  <>
                    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />
                    <ActionButton tooltip="重置密码" icon={<KeyOutlined />} loading={resetPassword.isPending} onClick={() => resetPassword.mutate(record)} />
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
            columns={[
              { title: '姓名', dataIndex: 'name', width: 120 },
              { title: '手机号', dataIndex: 'phone', width: 140 },
              { title: '岗位', dataIndex: 'role', width: 120, render: (value: string) => <Tag color={roleColor(value)}>{roleLabels[value] ?? value}</Tag> },
              { title: '校区', dataIndex: 'campusId', width: 120, render: (value?: string) => value || <Typography.Text type="secondary">全部校区</Typography.Text> },
              { title: '微信绑定', dataIndex: 'bindStatus', width: 110, render: (value: string) => <Tag color={value === '已绑定' ? 'green' : 'orange'}>{value}</Tag> },
              { title: '登录方式', dataIndex: 'bindStatus', width: 130, render: passwordFallbackTag },
              { title: '账号状态', dataIndex: 'accountStatus', width: 110, render: (value: string) => <Tag color={value === '正常' ? 'green' : 'default'}>{value}</Tag> },
              { title: '备注', dataIndex: 'remark', ellipsis: true },
              {
                title: '操作',
                width: 96,
                render: (_, record) => (
                  <Space size={4}>
                    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />
                    <ActionButton tooltip="重置密码" icon={<KeyOutlined />} loading={resetPassword.isPending} onClick={() => resetPassword.mutate(record)} />
                  </Space>
                )
              }
            ]}
          />
        )}
      </Card>

      <Modal
        title={editing ? '编辑管理人员' : '新增管理人员'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saveStaff.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveStaff.mutate(values)}>
          <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
            <Input placeholder="例如：张老师" />
          </Form.Item>
          <Form.Item name="phone" label="手机号" rules={[{ required: true, message: '请输入手机号' }]}>
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
      </Modal>
      <Modal
        title="临时密码"
        open={Boolean(resetResult)}
        onCancel={() => setResetResult(null)}
        footer={<Button type="primary" onClick={() => setResetResult(null)}>我已记录</Button>}
        destroyOnHidden
      >
        <Typography.Paragraph>请把下面的临时密码通过安全渠道发给对方。对方首次用手机号密码登录时需要修改密码。</Typography.Paragraph>
        <Typography.Text copyable strong>{resetResult?.temporaryPassword}</Typography.Text>
      </Modal>
    </div>
  );
}

function roleColor(role: string) {
  if (role === 'super_admin') return 'red';
  if (role === 'campus_admin') return 'blue';
  return 'purple';
}

function passwordFallbackTag(bindStatus: string) {
  return bindStatus === '已绑定'
    ? <Tag color="green">微信优先</Tag>
    : <Tag color="orange">可用临时密码</Tag>;
}
