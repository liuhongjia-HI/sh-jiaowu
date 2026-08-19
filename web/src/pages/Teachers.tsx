import { EditOutlined, KeyOutlined, PlusOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Form, Input, Modal, Select, Skeleton, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData, putData, resetTeacherPassword } from '../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, TagGroup, useListViewMode } from '../components/ListViews';
import type { LearningSpace, PasswordResetResult, Teacher, TeacherUpsertRequest } from '../types/starline';

type TeacherFormValues = {
  name: string;
  phone: string;
  learningSpaceIds: string[];
  canUploadHandout: boolean;
  canUploadQuestion: boolean;
  canReview: boolean;
  remark: string;
  enabled: boolean;
};

export default function Teachers() {
  const [form] = Form.useForm<TeacherFormValues>();
  const [editing, setEditing] = useState<Teacher | null>(null);
  const [open, setOpen] = useState(false);
  const [resetResult, setResetResult] = useState<PasswordResetResult | null>(null);
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:teachers');
  const queryClient = useQueryClient();

  const teachers = useQuery({ queryKey: ['teachers'], queryFn: () => getData<Teacher[]>('/teachers') });
  const learningSpaces = useQuery({ queryKey: ['learning-spaces'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') });

  const saveTeacher = useMutation({
    mutationFn: (values: TeacherFormValues) => {
      const body: TeacherUpsertRequest = {
        name: values.name,
        phone: values.phone,
        learningSpaceIds: values.learningSpaceIds ?? [],
        canUploadHandout: Boolean(values.canUploadHandout),
        canUploadQuestion: Boolean(values.canUploadQuestion),
        canReview: Boolean(values.canReview),
        remark: values.remark ?? '',
        accountStatus: editing ? (values.enabled ? '正常' : '停用') : undefined
      };
      if (editing) return putData<Teacher>(`/teachers/${editing.id}`, body);
      return postData<Teacher>('/teachers', body);
    },
    onSuccess: (result) => {
      message.success(editing ? '教师信息已保存' : '教师账号已创建');
      if (!editing && result.temporaryPassword) {
        setResetResult({ userId: result.id, temporaryPassword: result.temporaryPassword, mustChangePassword: true });
      }
      setOpen(false);
      setEditing(null);
      queryClient.invalidateQueries({ queryKey: ['teachers'] });
    },
    onError: () => message.error('保存失败，请检查姓名、手机号和负责课程范围。')
  });

  const resetPassword = useMutation({
    mutationFn: (teacher: Teacher) => resetTeacherPassword(teacher.id),
    onSuccess: (result) => {
      setResetResult(result);
      message.success('临时密码已生成');
    },
    onError: (error: any) => message.error(error.response?.data?.message || '重置密码失败，请稍后重试。')
  });

  function openCreate() {
    setEditing(null);
    form.setFieldsValue({
      name: '',
      phone: '',
      learningSpaceIds: [],
      canUploadHandout: true,
      canUploadQuestion: true,
      canReview: true,
      remark: '',
      enabled: true
    });
    setOpen(true);
  }

  function openEdit(teacher: Teacher) {
    setEditing(teacher);
    form.setFieldsValue({
      name: teacher.name,
      phone: teacher.phone,
      learningSpaceIds: teacher.learningSpaceIds,
      canUploadHandout: teacher.canUploadHandout,
      canUploadQuestion: teacher.canUploadQuestion,
      canReview: teacher.canReview,
      remark: teacher.remark,
      enabled: teacher.accountStatus === '正常'
    });
    setOpen(true);
  }

  if (teachers.isLoading || learningSpaces.isLoading) return <Skeleton active />;
  if (teachers.error || learningSpaces.error) return <Alert type="error" message="教师列表加载失败，请稍后重试。" />;

  const rows = teachers.data ?? [];
  const learningSpaceOptions = (learningSpaces.data ?? []).map((item) => ({
    label: item.name,
    value: item.id
  }));

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>老师管理</Typography.Title>
          <Typography.Text type="secondary">维护教师账号、课程范围和批改权限。</Typography.Text>
        </div>
        <div className="page-heading-actions">
          <ListViewToggle storageKey="starline:list-view:teachers" value={viewMode} onChange={setViewMode} />
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增教师</Button>
        </div>
      </div>

      <Card>
        {viewMode === 'card' ? (
          <CardList
            rows={rows}
            rowKey={(record) => record.id}
            emptyText="还没有教师，先新增教师并设置负责课程范围。"
            renderCard={(record) => (
              <InfoCard
                title={record.name}
                subtitle={record.phone}
                status={<Tag color={record.accountStatus === '正常' ? 'green' : 'default'}>{record.accountStatus}</Tag>}
                fields={[
                  { label: '微信绑定', value: <Tag color={record.bindStatus === '已绑定' ? 'green' : 'orange'}>{record.bindStatus}</Tag> },
                  { label: '登录方式', value: passwordFallbackTag(record.bindStatus) },
                  { label: '可上传内容', value: uploadTags(record) },
                  { label: '可批改', value: <Tag color={record.canReview ? 'green' : 'default'}>{record.canReview ? '是' : '否'}</Tag> },
                  { label: '备注', value: record.remark || '-' }
                ]}
                tags={<TagGroup values={record.learningSpaces} color="blue" emptyText="未分配负责课程范围" />}
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
          <Empty description="还没有教师，先新增教师并设置负责课程范围。" />
        ) : (
          <Table
            rowKey="id"
            dataSource={rows}
            pagination={false}
            columns={[
              { title: '姓名', dataIndex: 'name', width: 120 },
              { title: '手机号', dataIndex: 'phone', width: 140 },
              { title: '微信绑定', dataIndex: 'bindStatus', width: 110, render: (value: string) => <Tag color={value === '已绑定' ? 'green' : 'orange'}>{value}</Tag> },
              { title: '登录方式', dataIndex: 'bindStatus', width: 130, render: passwordFallbackTag },
              { title: '账号状态', dataIndex: 'accountStatus', width: 110, render: (value: string) => <Tag color={value === '正常' ? 'green' : 'default'}>{value}</Tag> },
              { title: '负责课程范围', dataIndex: 'learningSpaces', render: (values: string[]) => scopeTags(values, 'blue', '未分配') },
              { title: '可上传内容', width: 180, render: (_, record) => uploadTags(record) },
              { title: '可批改', dataIndex: 'canReview', width: 100, render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '是' : '否'}</Tag> },
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
        title={editing ? '编辑教师' : '新增教师'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saveTeacher.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveTeacher.mutate(values)}>
          {!editing && (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="保存后会自动生成临时密码"
              description="这里不需要设置密码。保存成功后会弹出临时密码，请复制并通过安全渠道发给老师；对方首次用手机号登录时需要自行修改。密码遗失可在列表里用钥匙图标重置。"
            />
          )}
          <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入教师姓名' }]}>
            <Input placeholder="例如：英语老师" />
          </Form.Item>
          <Form.Item name="phone" label="手机号" rules={[{ required: true, message: '请输入手机号' }]}>
            <Input placeholder="用于首次登录和身份确认" />
          </Form.Item>
          <Form.Item name="learningSpaceIds" label="负责课程范围" rules={[{ required: true, message: '请选择负责课程范围' }]}>
            <Select
              mode="multiple"
              allowClear
              showSearch
              optionFilterProp="label"
              options={learningSpaceOptions}
              placeholder="选择负责的年级、学科、学期和阶段"
            />
          </Form.Item>
          <Form.Item name="canUploadHandout" label="可上传学习资料" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="canUploadQuestion" label="可上传练习" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="canReview" label="可批改" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={3} placeholder="可填写岗位、排班或交接说明" />
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
        <Typography.Paragraph>请把下面的临时密码通过安全渠道发给老师。对方首次用手机号密码登录时需要修改密码。</Typography.Paragraph>
        <Typography.Text copyable strong>{resetResult?.temporaryPassword}</Typography.Text>
      </Modal>
    </div>
  );
}

function uploadTags(record: Teacher) {
  const values = [
    record.canUploadHandout ? '学习资料' : '',
    record.canUploadQuestion ? '练习' : ''
  ].filter(Boolean);
  return scopeTags(values, 'purple', '不可上传');
}

function passwordFallbackTag(bindStatus: string) {
  return bindStatus === '已绑定'
    ? <Tag color="green">微信优先</Tag>
    : <Tag color="orange">可用临时密码</Tag>;
}

function scopeTags(values: string[], color: string, emptyText: string) {
  if (!values || values.length === 0) return <Typography.Text type="secondary">{emptyText}</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {values.map((value) => <Tag key={value} color={color}>{value}</Tag>)}
    </Space>
  );
}
