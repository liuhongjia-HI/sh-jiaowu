import {
  Alert,
  Button,
  Card,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  Modal,
  Select,
  Skeleton,
  Space,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
  Upload,
  message
} from 'antd';
import type { TableColumnsType, UploadFile } from 'antd';
import { BellOutlined, EditOutlined, EyeOutlined, ImportOutlined, PlusOutlined, UnlockOutlined, UploadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { getData, postData, postForm, putData } from '../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, TagGroup, useListViewMode } from '../components/ListViews';
import { gradeOptions as curriculumGradeOptions } from '../utils/curriculum';
import type {
  CurrentUser,
  GrantPreview,
  Student,
  StudentDetail,
  StudentImportResult,
  StudentRemindResult,
  StudentUpsertRequest,
  StudyPackage
} from '../types/starline';

type StudentFormValues = {
  name: string;
  phone: string;
  grade: string;
  remark: string;
  enabled: boolean;
};

type StudentFilters = {
  keyword?: string;
  grade?: string;
  accountStatus?: string;
  learningStatus?: string;
  packageState?: string;
};

type GrantFormValues = {
  packageId: string;
};

function canWrite(user: CurrentUser) {
  return user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));
}

export default function Students({ user }: { user: CurrentUser }) {
  const [filters, setFilters] = useState<StudentFilters>({});
  const [studentForm] = Form.useForm<StudentFormValues>();
  const [grantForm] = Form.useForm<GrantFormValues>();
  const [editing, setEditing] = useState<Student | null>(null);
  const [studentModalOpen, setStudentModalOpen] = useState(false);
  const [selected, setSelected] = useState<Student | null>(null);
  const [grantStudent, setGrantStudent] = useState<Student | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const queryClient = useQueryClient();
  const writable = canWrite(user);
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:students');

  const students = useQuery({
    queryKey: ['students', filters],
    queryFn: () => getData<Student[]>('/students', compactParams(filters))
  });
  const packages = useQuery({ queryKey: ['packages'], queryFn: () => getData<StudyPackage[]>('/packages') });
  const detail = useQuery({
    queryKey: ['students', selected?.id, 'detail'],
    enabled: Boolean(selected),
    queryFn: () => getData<StudentDetail>(`/students/${selected?.id}`)
  });

  const packageId = Form.useWatch('packageId', grantForm);
  const grantPreview = useQuery({
    queryKey: ['student-grant-preview', grantStudent?.id, packageId],
    enabled: Boolean(grantStudent?.id && packageId),
    queryFn: () => getData<GrantPreview>('/grants/preview', { studentId: grantStudent?.id ?? '', packageId })
  });

  const saveStudent = useMutation({
    mutationFn: (values: StudentFormValues) => {
      const body: StudentUpsertRequest = {
        name: values.name,
        phone: values.phone,
        grade: values.grade,
        remark: values.remark ?? '',
        accountStatus: editing ? (values.enabled ? '正常' : '停用') : undefined
      };
      if (editing) return putData<Student>(`/students/${editing.id}`, body);
      return postData<Student>('/students', body);
    },
    onSuccess: () => {
      message.success(editing ? '学生信息已保存' : '学生已新增');
      setStudentModalOpen(false);
      setEditing(null);
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
    onError: () => message.error('保存失败，请检查姓名、手机号是否填写完整。')
  });

  const remindStudent = useMutation({
    mutationFn: (student: Student) => postData<StudentRemindResult>(`/students/${student.id}/remind`, {}),
    onSuccess: (result) => {
      message.success(result.message);
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
    onError: () => message.error('提醒失败，请稍后重试。')
  });

  const createGrant = useMutation({
    mutationFn: (values: GrantFormValues) => postData<GrantPreview>('/grants', { studentId: grantStudent?.id, packageId: values.packageId }),
    onSuccess: (result) => {
      message.success(result.alreadyOpened ? '该学生已开通过这个套餐，学习权限保持有效。' : '学习套餐已开通');
      setGrantStudent(null);
      grantForm.resetFields();
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
    onError: () => message.error('开通失败，请检查学生和学习套餐。')
  });

  const importStudents = useMutation({
    mutationFn: () => {
      const file = fileList[0]?.originFileObj;
      if (!file) throw new Error('请选择导入文件');
      const body = new FormData();
      body.append('file', file);
      return postForm<StudentImportResult>('/students/import', body);
    },
    onSuccess: (result) => {
      message.success(`导入完成：成功 ${result.successCount} 条，失败 ${result.failedCount} 条`);
      setFileList([]);
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
    onError: () => message.error('导入失败，请确认 CSV 文件格式是否正确。')
  });

  const rows = students.data ?? [];
  const gradeOptions = useMemo(() => uniqueOptions(rows.map((item) => item.grade)), [rows]);
  const learningOptions = useMemo(() => uniqueOptions(rows.map((item) => item.learningStatus)), [rows]);
  const accountOptions = useMemo(() => uniqueOptions(rows.map((item) => item.accountStatus)), [rows]);
  const stats = useMemo(() => ({
    total: rows.length,
    opened: rows.filter((item) => item.openedPackages.length > 0).length,
    waiting: rows.filter((item) => item.accountStatus.includes('待') || item.learningStatus.includes('未')).length,
    disabled: rows.filter((item) => item.accountStatus === '停用').length
  }), [rows]);

  function openCreate() {
    setEditing(null);
    studentForm.setFieldsValue({ name: '', phone: '', grade: '', remark: '', enabled: true });
    setStudentModalOpen(true);
  }

  function openEdit(student: Student) {
    setEditing(student);
    studentForm.setFieldsValue({
      name: student.name,
      phone: student.phone,
      grade: student.grade,
      remark: student.remark ?? '',
      enabled: student.accountStatus !== '停用'
    });
    setStudentModalOpen(true);
  }

  const columns: TableColumnsType<Student> = [
    { title: '学生', dataIndex: 'name', width: 120, fixed: 'left' },
    { title: '年级', dataIndex: 'grade', width: 100 },
    { title: '手机号', dataIndex: 'phone', width: 140 },
    { title: '微信绑定', dataIndex: 'bindStatus', width: 110, render: (value) => <Tag color={value === '已绑定' ? 'green' : 'orange'}>{value}</Tag> },
    { title: '已开通套餐', dataIndex: 'openedPackages', render: (values: string[]) => tagList(values, 'blue', '暂未开通') },
    { title: '学习状态', dataIndex: 'learningStatus', width: 110, render: (value) => <Tag color={String(value).includes('未') ? 'orange' : 'green'}>{value}</Tag> },
    { title: '账号状态', dataIndex: 'accountStatus', width: 110, render: (value) => <Tag color={value === '正常' ? 'green' : value === '停用' ? 'default' : 'orange'}>{value}</Tag> },
    { title: '连续学习', dataIndex: 'streakDays', width: 100, render: (value) => `${value} 天` },
    { title: '平均分', dataIndex: 'averageScore', width: 90 },
    { title: '徽章', dataIndex: 'badgeCount', width: 80 },
    { title: '最近学习', dataIndex: 'lastStudyAt', width: 160, render: (value) => value || '-' },
    {
      title: '操作',
      width: writable ? 260 : 100,
      fixed: 'right',
      render: (_, record) => (
        <Space>
          <Button icon={<EyeOutlined />} onClick={() => setSelected(record)}>查看</Button>
          {writable && <Button icon={<UnlockOutlined />} onClick={() => setGrantStudent(record)}>开通</Button>}
          {writable && <Button icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)}>提醒</Button>}
          {writable && <Button icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>}
        </Space>
      )
    }
  ];

  if (students.isLoading) return <Skeleton active />;
  if (students.error) return <Alert type="error" message="学生列表加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>学生管理</Typography.Title>
          <Typography.Text type="secondary">查看学生账号、学习套餐、学习进度和提醒跟进情况。</Typography.Text>
        </div>
        {writable && (
          <Space>
            <Button icon={<ImportOutlined />} onClick={() => setImportOpen(true)}>批量导入</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增学生</Button>
          </Space>
        )}
      </div>

      <div className="student-stat-grid">
        <Card><Statistic title="学生总数" value={stats.total} /></Card>
        <Card><Statistic title="已开通套餐" value={stats.opened} /></Card>
        <Card><Statistic title="待跟进" value={stats.waiting} /></Card>
        <Card><Statistic title="已停用" value={stats.disabled} /></Card>
      </div>

      <Card>
        <div className="list-panel">
          <div className="list-toolbar">
            <Space wrap>
              <Input.Search placeholder="搜索姓名或手机号" allowClear onSearch={(keyword) => setFilters((prev) => ({ ...prev, keyword }))} style={{ width: 240 }} />
              <Select allowClear placeholder="年级" options={gradeOptions} style={{ width: 140 }} onChange={(grade) => setFilters((prev) => ({ ...prev, grade }))} />
              <Select allowClear placeholder="账号状态" options={accountOptions} style={{ width: 140 }} onChange={(accountStatus) => setFilters((prev) => ({ ...prev, accountStatus }))} />
              <Select allowClear placeholder="学习状态" options={learningOptions} style={{ width: 150 }} onChange={(learningStatus) => setFilters((prev) => ({ ...prev, learningStatus }))} />
              <Select
                allowClear
                placeholder="开通状态"
                options={[{ label: '已开通', value: '已开通' }, { label: '未开通', value: '未开通' }]}
                style={{ width: 140 }}
                onChange={(packageState) => setFilters((prev) => ({ ...prev, packageState }))}
              />
            </Space>
            <ListViewToggle storageKey="starline:list-view:students" value={viewMode} onChange={setViewMode} />
          </div>
          {viewMode === 'card' ? (
            <CardList
              rows={rows}
              rowKey={(record) => record.id}
              emptyText="还没有学生，先新增学生或批量导入。"
              renderCard={(record) => (
                <InfoCard
                  title={record.name}
                  subtitle={`${record.grade} · ${record.phone}`}
                  status={<Tag color={record.accountStatus === '正常' ? 'green' : record.accountStatus === '停用' ? 'default' : 'orange'}>{record.accountStatus}</Tag>}
                  fields={[
                    { label: '微信绑定', value: <Tag color={record.bindStatus === '已绑定' ? 'green' : 'orange'}>{record.bindStatus}</Tag> },
                    { label: '学习状态', value: <Tag color={record.learningStatus.includes('未') ? 'orange' : 'green'}>{record.learningStatus}</Tag> },
                    { label: '连续学习', value: `${record.streakDays} 天` },
                    { label: '平均分', value: record.averageScore ?? '-' },
                    { label: '徽章', value: `${record.badgeCount} 枚` },
                    { label: '最近学习', value: record.lastStudyAt || '-' }
                  ]}
                  tags={<TagGroup values={record.openedPackages} color="blue" emptyText="暂未开通学习套餐" />}
                  actions={(
                    <>
                      <ActionButton icon={<EyeOutlined />} onClick={() => setSelected(record)}>查看</ActionButton>
                      {writable && <ActionButton icon={<UnlockOutlined />} onClick={() => setGrantStudent(record)}>开通</ActionButton>}
                      {writable && <ActionButton icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)}>提醒</ActionButton>}
                      {writable && <ActionButton icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</ActionButton>}
                    </>
                  )}
                />
              )}
            />
          ) : (
            rows.length === 0 ? <Empty description="还没有学生，先新增学生或批量导入。" /> : <Table rowKey="id" columns={columns} dataSource={rows} scroll={{ x: 1500 }} pagination={{ pageSize: 8 }} />
          )}
        </div>
      </Card>

      <Modal
        title={editing ? '编辑学生' : '新增学生'}
        open={studentModalOpen}
        onCancel={() => setStudentModalOpen(false)}
        onOk={() => studentForm.submit()}
        confirmLoading={saveStudent.isPending}
        destroyOnHidden
      >
        <Form form={studentForm} layout="vertical" onFinish={(values) => saveStudent.mutate(values)}>
          <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入学生姓名' }]}>
            <Input placeholder="例如：小明" />
          </Form.Item>
          <Form.Item name="phone" label="手机号" rules={[{ required: true, message: '请输入手机号' }]}>
            <Input placeholder="用于首次登录和身份确认" />
          </Form.Item>
          <Form.Item name="grade" label="年级" rules={[{ required: true, message: '请选择年级' }]}>
            <Select options={gradeOptions.length ? gradeOptions : curriculumGradeOptions()} placeholder="请选择年级" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={3} placeholder="可填写家长沟通、分班或交接信息" />
          </Form.Item>
          {editing && (
            <Form.Item name="enabled" label="启用账号" valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>

      <Modal
        title={grantStudent ? `给 ${grantStudent.name} 开通套餐` : '开通套餐'}
        open={Boolean(grantStudent)}
        onCancel={() => setGrantStudent(null)}
        onOk={() => grantForm.submit()}
        confirmLoading={createGrant.isPending}
        okButtonProps={{ disabled: Boolean(grantPreview.data?.alreadyOpened) }}
        destroyOnHidden
      >
        <Form form={grantForm} layout="vertical" onFinish={(values) => createGrant.mutate(values)}>
          <Form.Item name="packageId" label="学习套餐" rules={[{ required: true, message: '请选择套餐' }]}>
            <Select options={(packages.data ?? []).map((item) => ({ label: item.name, value: item.id }))} loading={packages.isLoading} />
          </Form.Item>
        </Form>
        {!packageId && <Alert type="info" message="请选择学习套餐后查看学生可学习的内容。" />}
        {grantPreview.isLoading && <Skeleton active />}
        {grantPreview.data && (
          <Alert
            type={grantPreview.data.alreadyOpened ? 'info' : 'success'}
            showIcon
            message={grantPreview.data.alreadyOpened ? `${grantPreview.data.studentName} 已开通：${grantPreview.data.packageName}` : `${grantPreview.data.studentName} 将开通：${grantPreview.data.packageName}`}
            description={grantPreview.data.alreadyOpened ? `当前有效期至：${grantPreview.data.existingUntil || '暂无'}` : `适用课程范围：${grantPreview.data.learningSpaces.join('、') || '暂无'}；包含学习内容：${grantPreview.data.contentTypes.join('、') || '暂无'}；有效期：${grantPreview.data.effectiveDefault}`}
          />
        )}
      </Modal>

      <Modal
        title="批量导入学生"
        open={importOpen}
        onCancel={() => setImportOpen(false)}
        onOk={() => importStudents.mutate()}
        confirmLoading={importStudents.isPending}
      >
        <Upload
          accept=".csv"
          beforeUpload={() => false}
          maxCount={1}
          fileList={fileList}
          onChange={({ fileList: nextFileList }) => setFileList(nextFileList)}
        >
          <Button icon={<UploadOutlined />}>选择 CSV 文件</Button>
        </Upload>
        <Typography.Paragraph type="secondary" style={{ marginTop: 12 }}>
          CSV 第一行请使用表头：name, phone, grade, remark。
        </Typography.Paragraph>
        {importStudents.data && importStudents.data.errors.length > 0 && (
          <Alert
            type="warning"
            message="部分学生导入失败"
            description={importStudents.data.errors.map((item) => `第 ${item.row} 行：${item.message}`).join('；')}
          />
        )}
      </Modal>

      <Drawer title={selected?.name ?? '学生详情'} width={720} open={Boolean(selected)} onClose={() => setSelected(null)}>
        {detail.isLoading && <Skeleton active />}
        {detail.error && <Alert type="error" message="学生详情加载失败，请稍后重试。" />}
        {detail.data && (
          <Tabs
            items={[
              { key: 'profile', label: '基础信息', children: <StudentProfile detail={detail.data} /> },
              { key: 'records', label: '学习记录', children: <RecordTable detail={detail.data} /> },
              { key: 'logs', label: '操作记录', children: <LogTable detail={detail.data} /> }
            ]}
          />
        )}
      </Drawer>
    </div>
  );
}

function StudentProfile({ detail }: { detail: StudentDetail }) {
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Descriptions column={2} bordered size="small">
        <Descriptions.Item label="姓名">{detail.student.name}</Descriptions.Item>
        <Descriptions.Item label="年级">{detail.student.grade}</Descriptions.Item>
        <Descriptions.Item label="手机号">{detail.student.phone}</Descriptions.Item>
        <Descriptions.Item label="微信绑定">{detail.student.bindStatus}</Descriptions.Item>
        <Descriptions.Item label="账号状态">{detail.student.accountStatus}</Descriptions.Item>
        <Descriptions.Item label="最近学习">{detail.student.lastStudyAt || '-'}</Descriptions.Item>
        <Descriptions.Item label="备注" span={2}>{detail.student.remark || '-'}</Descriptions.Item>
      </Descriptions>
      <CardList
        rows={detail.grants}
        rowKey={(record) => record.packageId}
        emptyText="暂未开通学习套餐"
        renderCard={(record) => (
          <InfoCard
            title={record.packageName}
            subtitle={`有效期至 ${record.effectiveUntil}`}
            status={<Tag color={record.permissionState === '生效中' ? 'green' : 'default'}>{record.permissionState}</Tag>}
          />
        )}
      />
      <Descriptions column={1} bordered size="small">
        <Descriptions.Item label="适用课程范围">{detail.permissions.learningSpaces.join('、') || '暂无'}</Descriptions.Item>
        <Descriptions.Item label="包含学习内容">{detail.permissions.contentTypes.join('、') || '暂无'}</Descriptions.Item>
        <Descriptions.Item label="开放课程">{detail.permissions.openCourses.join('、') || '暂无'}</Descriptions.Item>
        <Descriptions.Item label="开放资料">{detail.permissions.openMaterials.join('、') || '暂无'}</Descriptions.Item>
        <Descriptions.Item label="开放练习">{detail.permissions.openHomework.join('、') || '暂无'}</Descriptions.Item>
      </Descriptions>
    </Space>
  );
}

function RecordTable({ detail }: { detail: StudentDetail }) {
  if (detail.learningRecords.length === 0) return <Empty description="还没有学习记录。" />;
  return (
    <CardList
      rows={detail.learningRecords}
      rowKey={(record) => record.id}
      emptyText="还没有学习记录。"
      renderCard={(record) => (
        <InfoCard
          title={record.title}
          subtitle={`${record.type} · ${record.course}`}
          status={<Tag>{record.status}</Tag>}
          fields={[
            { label: '分数', value: record.score ?? '-' },
            { label: '时间', value: record.occurredAt },
            { label: '说明', value: record.description || '-' }
          ]}
        />
      )}
    />
  );
}

function LogTable({ detail }: { detail: StudentDetail }) {
  if (detail.logs.length === 0 && detail.notices.length === 0) return <Empty description="还没有操作记录。" />;
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {detail.notices.length > 0 && (
        <CardList
          rows={detail.notices}
          rowKey={(record) => record.id}
          emptyText="还没有提醒记录。"
          renderCard={(record) => (
            <InfoCard
              title={record.title}
              subtitle={record.summary}
              status={<Tag>{record.status}</Tag>}
            />
          )}
        />
      )}
      {detail.logs.length > 0 && (
        <CardList
          rows={detail.logs}
          rowKey={(record) => record.id}
          emptyText="还没有操作记录。"
          renderCard={(record) => (
            <InfoCard
              title={record.action}
              subtitle={record.target}
              fields={[
                { label: '操作人', value: record.operator },
                { label: '时间', value: record.time }
              ]}
            />
          )}
        />
      )}
    </Space>
  );
}

function compactParams(filters: StudentFilters) {
  return Object.fromEntries(Object.entries(filters).filter(([, value]) => value)) as Record<string, string>;
}

function uniqueOptions(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).map((value) => ({ label: value, value }));
}

function tagList(values: string[], color: string, emptyText: string) {
  if (!values || values.length === 0) return <Typography.Text type="secondary">{emptyText}</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {values.map((value) => <Tag key={value} color={color}>{value}</Tag>)}
    </Space>
  );
}
