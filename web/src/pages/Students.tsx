import {
  Alert,
  Button,
  Card,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
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
import { useEffect, useMemo, useState } from 'react';
import { getData, postData, postForm, putData } from '../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, TagGroup, useListViewMode } from '../components/ListViews';
import { gradeOptions as curriculumGradeOptions, subjectOptions } from '../utils/curriculum';
import type {
  CurrentUser,
  GrantCreateRequest,
  GrantPreview,
  Student,
  StudentDetail,
  StudentImportResult,
  StudentRemindResult,
  StudentScoreRecord,
  StudentScoreSummary,
  StudentScoreUpsertRequest,
  StudentUpsertRequest,
  StudyPackage
} from '../types/starline';

type StudentFormValues = {
  name: string;
  phone: string;
  grade: string;
  schoolName: string;
  officialAccountOpenId: string;
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
  startsAt?: string;
  endsAt?: string;
};

function canWrite(user: CurrentUser) {
  return user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));
}

function canManageScores(user: CurrentUser) {
  return user.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role));
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
  const grantStartsAt = Form.useWatch('startsAt', grantForm);
  const availableGrantPackages = useMemo(
    () => (packages.data ?? []).filter((item) => item.status === '启用' && item.grade === grantStudent?.grade),
    [packages.data, grantStudent?.grade]
  );
  const grantPreview = useQuery({
    queryKey: ['student-grant-preview', grantStudent?.id, packageId],
    enabled: Boolean(grantStudent?.id && packageId),
    queryFn: () => getData<GrantPreview>('/grants/preview', { studentId: grantStudent?.id ?? '', packageId })
  });
  useEffect(() => {
    if (!grantPreview.data) return;
    if (!grantForm.getFieldValue('startsAt')) grantForm.setFieldValue('startsAt', grantPreview.data.existingStartsAt || grantPreview.data.startsAtDefault);
    if (!grantForm.getFieldValue('endsAt')) grantForm.setFieldValue('endsAt', grantPreview.data.existingUntil || grantPreview.data.endsAtDefault);
  }, [grantForm, grantPreview.data]);

  const saveStudent = useMutation({
    mutationFn: (values: StudentFormValues) => {
      const body: StudentUpsertRequest = {
        name: values.name,
        phone: values.phone,
        grade: values.grade,
        schoolName: values.schoolName ?? '',
        officialAccountOpenId: values.officialAccountOpenId ?? '',
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

  const generateBindCode = useMutation({
    mutationFn: (studentId: string) => postData<Student>(`/students/${studentId}/bind-code`, {}),
    onSuccess: (result) => {
      message.success('绑定码已生成，7 天内有效');
      queryClient.invalidateQueries({ queryKey: ['students', result.id, 'detail'] });
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
    onError: () => message.error('绑定码生成失败，请稍后重试。')
  });

  const createGrant = useMutation({
    mutationFn: (values: GrantFormValues) => postData<GrantPreview>('/grants', grantBody(grantStudent, values)),
    onSuccess: (result) => {
      message.success(result.alreadyOpened ? '套餐有效期已更新，学习权限已同步。' : '学习套餐已开通');
      setGrantStudent(null);
      grantForm.resetFields();
      queryClient.invalidateQueries({ queryKey: ['students'] });
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
    },
    onError: (err: Error) => message.error(err.message || '开通失败，请检查学生和学习套餐。')
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
    opened: rows.filter((item) => (item.openedPackages?.length ?? 0) > 0).length,
    waiting: rows.filter((item) => item.accountStatus.includes('待') || item.learningStatus.includes('未')).length,
    disabled: rows.filter((item) => item.accountStatus === '停用').length
  }), [rows]);

  function openCreate() {
    setEditing(null);
    studentForm.setFieldsValue({ name: '', phone: '', grade: '', schoolName: '', officialAccountOpenId: '', remark: '', enabled: true });
    setStudentModalOpen(true);
  }

  function openEdit(student: Student) {
    setEditing(student);
    studentForm.setFieldsValue({
      name: student.name,
      phone: student.phone,
      grade: student.grade,
      schoolName: student.schoolName ?? '',
      officialAccountOpenId: student.officialAccountOpenId ?? '',
      remark: student.remark ?? '',
      enabled: student.accountStatus !== '停用'
    });
    setStudentModalOpen(true);
  }

  const columns: TableColumnsType<Student> = [
    {
      title: '学生',
      dataIndex: 'name',
      width: 170,
      render: (value, record) => <Space direction="vertical" size={0}><Typography.Text strong>{value}</Typography.Text><Typography.Text type="secondary">{record.phone}</Typography.Text></Space>
    },
    { title: '年级', dataIndex: 'grade', width: 88 },
    { title: '学校', dataIndex: 'schoolName', width: 130, ellipsis: true, render: (value) => value || '-' },
    {
      title: '关联状态',
      width: 108,
      render: (_, record) => <Space direction="vertical" size={2}><Tag color={record.officialAccountOpenId ? 'green' : 'orange'}>{record.officialAccountOpenId ? '公众号已关联' : '公众号未关联'}</Tag><Tag color={record.bindStatus === '已绑定' ? 'green' : 'orange'}>{record.bindStatus}</Tag></Space>
    },
    {
      title: '套餐',
      dataIndex: 'openedPackages',
      width: 130,
      render: (values: string[]) => <Tag color={values?.length ? 'blue' : 'default'}>{values?.length ? `已开通 ${values.length} 个` : '暂未开通'}</Tag>
    },
    {
      title: '学习情况',
      width: 146,
      render: (_, record) => <Space direction="vertical" size={2}><Tag color={record.learningStatus.includes('未') ? 'orange' : 'green'}>{record.learningStatus}</Tag><Typography.Text type="secondary">连续 {record.streakDays} 天 · 均分 {record.averageScore ?? '-'}</Typography.Text></Space>
    },
    { title: '账号', dataIndex: 'accountStatus', width: 88, render: (value) => <Tag color={value === '正常' ? 'green' : value === '停用' ? 'default' : 'orange'}>{value}</Tag> },
    { title: '最近学习', dataIndex: 'lastStudyAt', width: 150, ellipsis: true, render: (value) => value || '-' },
    {
      title: '操作',
      width: writable ? 128 : 52,
      render: (_, record) => (
        <Space size={4}>
          <ActionButton tooltip="查看" icon={<EyeOutlined />} onClick={() => setSelected(record)} />
          {writable && <ActionButton tooltip="开通" icon={<UnlockOutlined />} onClick={() => openGrant(record)} />}
          {writable && <ActionButton tooltip="提醒" icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)} />}
          {writable && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />}
        </Space>
      )
    }
  ];

  function openGrant(student: Student) {
    grantForm.resetFields();
    setGrantStudent(student);
  }

  if (students.isLoading) return <Skeleton active />;
  if (students.error) return <Alert type="error" message="学生列表加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>学生管理</Typography.Title>
          <Typography.Text type="secondary">查看账号、套餐、进度和跟进状态。</Typography.Text>
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
                    { label: '学校', value: record.schoolName || '-' },
                    { label: '公众号', value: <Tag color={record.officialAccountOpenId ? 'green' : 'orange'}>{record.officialAccountOpenId ? '已关联' : '未关联'}</Tag> },
                    { label: '学习状态', value: <Tag color={record.learningStatus.includes('未') ? 'orange' : 'green'}>{record.learningStatus}</Tag> },
                    { label: '连续学习', value: `${record.streakDays} 天` },
                    { label: '平均分', value: record.averageScore ?? '-' },
                    { label: '徽章', value: `${record.badgeCount} 枚` },
                    { label: '最近学习', value: record.lastStudyAt || '-' },
                    {
                      label: '最近提交',
                      value: record.lastSubmissionStatus
                        ? <Space size={4}><Tag color={submissionStatusColor(record.lastSubmissionStatus)}>{record.lastSubmissionStatus}</Tag><Typography.Text type="secondary">{record.lastSubmittedAt || '-'}</Typography.Text></Space>
                        : '-'
                    }
                  ]}
                  tags={<TagGroup values={record.openedPackages} color="blue" emptyText="暂未开通学习套餐" />}
                  actions={(
                    <>
                      <ActionButton tooltip="查看" icon={<EyeOutlined />} onClick={() => setSelected(record)} />
                      {writable && <ActionButton tooltip="开通" icon={<UnlockOutlined />} onClick={() => openGrant(record)} />}
                      {writable && <ActionButton tooltip="提醒" icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)} />}
                      {writable && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />}
                    </>
                  )}
                />
              )}
            />
          ) : (
            rows.length === 0 ? <Empty description="还没有学生，先新增学生或批量导入。" /> : <Table className="student-table" rowKey="id" columns={columns} dataSource={rows} tableLayout="fixed" pagination={{ pageSize: 8 }} />
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
          <Form.Item name="schoolName" label="学校" rules={[{ required: true, message: '请输入学校' }]}>
            <Input placeholder="例如：星河小学" />
          </Form.Item>
          <Form.Item name="officialAccountOpenId" label="公众号 openid">
            <Input placeholder="关注公众号后的 openid，用于模板消息提醒" />
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
        onCancel={() => {
          setGrantStudent(null);
          grantForm.resetFields();
        }}
        onOk={() => grantForm.submit()}
        confirmLoading={createGrant.isPending}
        okButtonProps={{ disabled: !packageId }}
        destroyOnHidden
      >
        <Form form={grantForm} layout="vertical" onFinish={(values) => createGrant.mutate(values)}>
          <Form.Item name="packageId" label="学习套餐" rules={[{ required: true, message: '请选择套餐' }]}>
            <Select
              showSearch
              optionFilterProp="label"
              placeholder={grantStudent ? '选择学习套餐' : '请先选择学生'}
              options={availableGrantPackages.map((item) => ({ label: packageOptionLabel(item), value: item.id }))}
              loading={packages.isLoading}
              disabled={!grantStudent}
              notFoundContent={grantStudent ? '该学生所在年级暂无启用套餐，请先到学习套餐中创建或启用对应年级套餐。' : '请先选择学生，再选择该年级可用套餐。'}
              onChange={() => grantForm.setFieldsValue({ startsAt: undefined, endsAt: undefined })}
            />
          </Form.Item>
          <Space size={12} style={{ width: '100%' }} align="start">
            <Form.Item name="startsAt" label="开始日期" rules={[{ required: true, message: '请选择开始日期' }]} style={{ flex: 1 }}>
              <InputDate disabled={!packageId} />
            </Form.Item>
            <Form.Item
              name="endsAt"
              label="结束日期"
              dependencies={['startsAt']}
              rules={[
                { required: true, message: '请选择结束日期' },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    const start = getFieldValue('startsAt');
                    if (!value || !start || value >= start) return Promise.resolve();
                    return Promise.reject(new Error('结束日期不能早于开始日期'));
                  }
                })
              ]}
              style={{ flex: 1 }}
            >
              <InputDate disabled={!packageId} min={grantStartsAt} />
            </Form.Item>
          </Space>
        </Form>
        {grantStudent && availableGrantPackages.length === 0 && (
          <Alert type="warning" showIcon message="该学生所在年级暂无启用套餐，请先到学习套餐中创建或启用对应年级套餐。" />
        )}
        {!packageId && availableGrantPackages.length > 0 && <Alert type="info" message="请选择学习套餐后查看学生可学习的内容。" />}
        {grantPreview.isLoading && <Skeleton active />}
        {grantPreview.data && (
          <Alert
            type={grantPreview.data.alreadyOpened ? 'info' : 'success'}
            showIcon
            message={grantPreview.data.alreadyOpened ? `${grantPreview.data.studentName} 已开通：${grantPreview.data.packageName}` : `${grantPreview.data.studentName} 将开通：${grantPreview.data.packageName}`}
            description={grantPreview.data.alreadyOpened ? `当前有效期：${grantPreview.data.existingStartsAt || '-'} 至 ${grantPreview.data.existingUntil || '-'}` : `适用课程范围：${grantPreview.data.learningSpaces.join('、') || '暂无'}；包含学习内容：${grantPreview.data.contentTypes.join('、') || '暂无'}；默认有效期：${grantPreview.data.effectiveDefault}`}
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
          CSV 第一行请使用表头：name, phone, grade, schoolName, remark, officialAccountOpenId。
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
              {
                key: 'profile',
                label: '基础信息',
                children: (
                  <StudentProfile
                    detail={detail.data}
                    writable={writable}
                    generatingBindCode={generateBindCode.isPending}
                    onGenerateBindCode={() => generateBindCode.mutate(detail.data!.student.id)}
                  />
                )
              },
              { key: 'records', label: '学习记录', children: <RecordTable detail={detail.data} /> },
              { key: 'scores', label: '成绩对比', children: selected ? <ScorePanel student={selected} canEdit={canManageScores(user)} /> : null },
              { key: 'logs', label: '操作记录', children: <LogTable detail={detail.data} /> }
            ]}
          />
        )}
      </Drawer>
    </div>
  );
}

const examTypeOptions = ['期中', '期末', '单元测', '模拟考', '阶段测评'].map((value) => ({ label: value, value }));

function ScorePanel({ student, canEdit }: { student: Student; canEdit: boolean }) {
  const [form] = Form.useForm<StudentScoreUpsertRequest>();
  const [editing, setEditing] = useState<StudentScoreRecord | null>(null);
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  const queryKey = ['students', student.id, 'scores'];
  const scores = useQuery({
    queryKey,
    queryFn: () => getData<StudentScoreSummary[]>(`/students/${student.id}/scores`)
  });

  const saveScore = useMutation({
    mutationFn: (values: StudentScoreUpsertRequest) => {
      const body: StudentScoreUpsertRequest = {
        subject: values.subject,
        examType: values.examType || '阶段测评',
        examName: values.examName,
        examDate: values.examDate,
        score: Number(values.score),
        fullScore: Number(values.fullScore),
        averageScore: Number(values.averageScore || 0),
        teacherComment: values.teacherComment ?? ''
      };
      if (editing) return putData<StudentScoreRecord>(`/students/${student.id}/scores/${editing.id}`, body);
      return postData<StudentScoreRecord>(`/students/${student.id}/scores`, body);
    },
    onSuccess: () => {
      message.success(editing ? '成绩已修正' : '成绩已录入');
      setOpen(false);
      setEditing(null);
      form.resetFields();
      queryClient.invalidateQueries({ queryKey });
      queryClient.invalidateQueries({ queryKey: ['students', student.id, 'detail'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败，请检查成绩信息。')
  });

  function openCreate() {
    setEditing(null);
    form.setFieldsValue({
      subject: undefined as unknown as string,
      examType: '阶段测评',
      examName: '',
      examDate: new Date().toISOString().slice(0, 10),
      score: 0,
      fullScore: 100,
      averageScore: 0,
      teacherComment: ''
    });
    setOpen(true);
  }

  function openEdit(record: StudentScoreRecord) {
    setEditing(record);
    form.setFieldsValue({
      subject: record.subject,
      examType: record.examType || '阶段测评',
      examName: record.examName,
      examDate: record.examDate,
      score: record.score,
      fullScore: record.fullScore,
      averageScore: record.averageScore ?? 0,
      teacherComment: record.teacherComment ?? ''
    });
    setOpen(true);
  }

  if (scores.isLoading) return <Skeleton active />;
  if (scores.error) return <Alert type="error" message="成绩记录加载失败，请稍后重试。" />;

  const summaries = scores.data ?? [];
  const rows = summaries.flatMap((summary) => summary.records.map((record) => ({ ...record, subject: summary.subject })));

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center' }}>
        <Typography.Text type="secondary">用于给家长展示课前和阶段成绩变化。</Typography.Text>
        {canEdit && <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>录入成绩</Button>}
      </div>
      {summaries.length === 0 ? (
        <Empty description="还没有成绩记录。" />
      ) : (
        <CardList
          rows={summaries}
          rowKey={(record) => record.subject}
          emptyText="还没有成绩记录。"
          renderCard={(summary) => {
            const latest = summary.latestRecord;
            const first = summary.firstRecord;
            const improvementText = !latest || !first || latest.id === first.id
              ? '暂无对比'
              : `${summary.improvement >= 0 ? '+' : ''}${summary.improvement} 分`;
            return (
              <InfoCard
                title={`${summary.subject} · ${latest?.score ?? '-'} / ${latest?.fullScore ?? '-'}`}
                subtitle={summary.description}
                status={<Tag color={summary.improvement > 0 ? 'green' : summary.improvement < 0 ? 'orange' : 'blue'}>{improvementText}</Tag>}
                fields={[
                  { label: '最近测评', value: latest ? `${latest.examType || '阶段测评'} · ${latest.examName} · ${latest.examDate}` : '-' },
                  { label: '首次测评', value: first ? `${first.examType || '阶段测评'} · ${first.examName} · ${first.score} 分` : '-' },
                  { label: '百分比变化', value: latest && first && latest.id !== first.id ? `${summary.improvementPct >= 0 ? '+' : ''}${summary.improvementPct}%` : '-' },
                  { label: '问题点', value: summary.problemPoint || '-' },
                  { label: '下一步', value: summary.nextStep || '-' }
                ]}
              />
            );
          }}
        />
      )}
      {rows.length > 0 && (
        <Table
          rowKey="id"
          size="small"
          dataSource={rows}
          pagination={false}
          columns={[
            { title: '日期', dataIndex: 'examDate', width: 110 },
            { title: '学科', dataIndex: 'subject', width: 80 },
            { title: '类型', dataIndex: 'examType', width: 90, render: (value) => value || '阶段测评' },
            { title: '考试/测评', dataIndex: 'examName' },
            { title: '分数', width: 100, render: (_, record) => `${record.score}/${record.fullScore}` },
            { title: '平均分', dataIndex: 'averageScore', width: 90, render: (value) => value || '-' },
            { title: '老师建议', dataIndex: 'teacherComment', render: (value) => <RichTextPreview value={value} /> },
            {
              title: '操作',
              width: 70,
              render: (_, record) => canEdit ? <ActionButton tooltip="修正" icon={<EditOutlined />} onClick={() => openEdit(record)} /> : null
            }
          ]}
        />
      )}
      <Modal
        title={editing ? '修正成绩' : '录入成绩'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saveScore.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveScore.mutate(values)}>
          <Form.Item name="examName" label="考试/测评名称" rules={[{ required: true, message: '请输入考试或测评名称' }]}>
            <Input placeholder="例如：入学测评 / 期中考试" />
          </Form.Item>
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item name="examType" label="考试类型" rules={[{ required: true, message: '请选择考试类型' }]} style={{ width: '34%' }}>
              <Select options={examTypeOptions} />
            </Form.Item>
            <Form.Item name="subject" label="学科" rules={[{ required: true, message: '请选择学科' }]} style={{ width: '33%' }}>
              <Select placeholder="学科" options={subjectOptions(student.grade)} />
            </Form.Item>
            <Form.Item name="examDate" label="考试日期" rules={[{ required: true, message: '请选择考试日期' }]} style={{ width: '33%' }}>
              <Input type="date" />
            </Form.Item>
          </Space.Compact>
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item name="score" label="分数" rules={[{ required: true, message: '请输入分数' }]} style={{ width: '33%' }}>
              <InputNumber min={0} precision={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="fullScore" label="满分" rules={[{ required: true, message: '请输入满分' }]} style={{ width: '33%' }}>
              <InputNumber min={1} precision={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="averageScore" label="平均分" style={{ width: '34%' }}>
              <InputNumber min={0} precision={0} style={{ width: '100%' }} />
            </Form.Item>
          </Space.Compact>
          <Form.Item name="teacherComment" label="老师建议">
            <RichTextInput placeholder="用家长能看懂的话说明下一步怎么补。" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
}

function StudentProfile({
  detail,
  writable,
  generatingBindCode,
  onGenerateBindCode
}: {
  detail: StudentDetail;
  writable: boolean;
  generatingBindCode: boolean;
  onGenerateBindCode: () => void;
}) {
  const codeExpired = Boolean(detail.student.bindCodeExpiresAt && detail.student.bindCodeExpiresAt < new Date().toISOString().slice(0, 10));
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Descriptions column={2} bordered size="small">
        <Descriptions.Item label="姓名">{detail.student.name}</Descriptions.Item>
        <Descriptions.Item label="年级">{detail.student.grade}</Descriptions.Item>
        <Descriptions.Item label="手机号">{detail.student.phone}</Descriptions.Item>
        <Descriptions.Item label="学校">{detail.student.schoolName || '-'}</Descriptions.Item>
        <Descriptions.Item label="家长称呼">{detail.student.guardianName || '-'}</Descriptions.Item>
        <Descriptions.Item label="公众号">{detail.student.officialAccountOpenId ? '已关联' : '未关联'}</Descriptions.Item>
        <Descriptions.Item label="微信绑定">{detail.student.bindStatus}</Descriptions.Item>
        <Descriptions.Item label="账号状态">{detail.student.accountStatus}</Descriptions.Item>
        <Descriptions.Item label="最近学习">{detail.student.lastStudyAt || '-'}</Descriptions.Item>
        <Descriptions.Item label="最近提交">
          {detail.student.lastSubmissionStatus
            ? <Space size={4}><Tag color={submissionStatusColor(detail.student.lastSubmissionStatus)}>{detail.student.lastSubmissionStatus}</Tag><Typography.Text type="secondary">{detail.student.lastSubmittedAt || '-'}</Typography.Text></Space>
            : '-'}
        </Descriptions.Item>
        <Descriptions.Item label="备注" span={2}>{detail.student.remark || '-'}</Descriptions.Item>
      </Descriptions>
      <Card size="small" title="关联其他家长">
        <Space direction="vertical" size={6} style={{ width: '100%' }}>
          {detail.student.bindCode ? (
            <>
              <Typography.Text
                copyable={{ text: detail.student.bindCode, tooltips: ['复制绑定码', '已复制'] }}
                style={{ fontSize: 22, fontWeight: 600, letterSpacing: 4, fontFamily: 'monospace' }}
              >
                {detail.student.bindCode}
              </Typography.Text>
              <Typography.Text type={codeExpired ? 'danger' : 'secondary'}>
                {codeExpired ? '已过期，需要重新生成。' : `有效期至 ${detail.student.bindCodeExpiresAt}`}
              </Typography.Text>
              <Typography.Text type="secondary">
                分享给其他家长（比如妈妈也想用自己的手机号关联），对方在小程序"我的 - 关联其他孩子"里输入这个码即可，不需要占用您已绑定的微信。
              </Typography.Text>
            </>
          ) : (
            <Typography.Text type="secondary">还没有生成过绑定码，生成后可以分享给其他家长关联这个学生。</Typography.Text>
          )}
          {writable && (
            <Button size="small" loading={generatingBindCode} onClick={onGenerateBindCode}>
              {detail.student.bindCode ? '重新生成（旧码立即失效）' : '生成绑定码'}
            </Button>
          )}
        </Space>
      </Card>
      <CardList
        rows={detail.grants}
        rowKey={(record) => record.packageId}
        emptyText="暂未开通学习套餐"
        renderCard={(record) => (
          <InfoCard
            title={record.packageName}
            subtitle={`${record.startsAt || '-'} 至 ${record.effectiveUntil || '-'}`}
            status={tagStatus(record.permissionState)}
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

function RichTextInput({ value, onChange, placeholder }: { value?: string; onChange?: (value: string) => void; placeholder?: string }) {
  const insert = (before: string, after = '') => {
    const current = value || '';
    onChange?.(`${current}${before}${after}`);
  };
  return (
    <Space direction="vertical" size={8} style={{ width: '100%' }}>
      <Space wrap>
        <Button size="small" onClick={() => insert('<strong>', '</strong>')}>B</Button>
        <Button size="small" onClick={() => insert('<ul><li>', '</li></ul>')}>列表</Button>
        <Button size="small" onClick={() => insert('<span style="color:#ef4444">', '</span>')}>重点</Button>
        <Button size="small" onClick={() => insert('<img src="" alt="建议配图" />')}>图片</Button>
      </Space>
      <Input.TextArea rows={4} value={value} onChange={(event) => onChange?.(event.target.value)} placeholder={placeholder} />
    </Space>
  );
}

function RichTextPreview({ value }: { value?: string }) {
  const content = sanitizeRichText(value || '');
  if (!content) return <Typography.Text type="secondary">-</Typography.Text>;
  return (
    <div
      className="rich-text-preview"
      dangerouslySetInnerHTML={{ __html: content }}
    />
  );
}

function sanitizeRichText(value: string) {
  return value
    .replace(/<script[\s\S]*?>[\s\S]*?<\/script>/gi, '')
    .replace(/\son\w+="[^"]*"/gi, '')
    .replace(/\son\w+='[^']*'/gi, '')
    .replace(/\s(href|src)=["']\s*javascript:[^"']*["']/gi, '')
    .replace(/<(?!\/?(strong|b|ul|ol|li|span|img|br|p)\b)[^>]+>/gi, '')
    .replace(/<span([^>]*)style=["'][^"']*color\s*:\s*(#[0-9a-fA-F]{3,6}|[a-zA-Z]+)[^"']*["']([^>]*)>/gi, '<span style="color:$2">')
    .replace(/<span(?![^>]*style=)[^>]*>/gi, '<span>')
    .replace(/<img([^>]*)src=["'](https?:\/\/[^"']+)["']([^>]*)>/gi, '<img src="$2" alt="老师建议配图" />')
    .replace(/<img(?![^>]*src=)[^>]*>/gi, '');
}

function compactParams(filters: StudentFilters) {
  return Object.fromEntries(Object.entries(filters).filter(([, value]) => value)) as Record<string, string>;
}

function uniqueOptions(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).map((value) => ({ label: value, value }));
}

function packageOptionLabel(item: StudyPackage) {
  return [item.name, item.subject, item.semester, item.packageType].filter(Boolean).join(' · ');
}

function grantBody(student: Student | null, values: GrantFormValues): GrantCreateRequest {
  return {
    studentId: student?.id,
    packageId: values.packageId,
    startsAt: values.startsAt,
    endsAt: values.endsAt
  };
}

function InputDate(props: { disabled?: boolean; min?: string; value?: string; onChange?: (event: any) => void }) {
  return <input className="ant-input" type="date" disabled={props.disabled} min={props.min} value={props.value || ''} onChange={props.onChange} />;
}

function tagStatus(value: string) {
  const color = value === '生效中' ? 'green' : value === '未开始' ? 'orange' : 'default';
  return <Tag color={color}>{value}</Tag>;
}

function submissionStatusColor(status?: string) {
  if (status === '已批改') return 'green';
  if (status === '待复核') return 'blue';
  if (status === '待批改') return 'orange';
  return 'default';
}
