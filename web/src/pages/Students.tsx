import {
  Alert,
  Button,
  Card,
  Checkbox,
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
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getData, postData, postForm, putData } from '../services/http';
import { FormDrawer } from '../components/FormDrawer';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../components/ListViews';
import { gradeOptions as curriculumGradeOptions, subjectOptions } from '../utils/curriculum';
import type {
  CurrentUser,
  DirectGrantCreateRequest,
  DirectGrantResult,
  LearningSpace,
  Student,
  StudentDetail,
  StudentImportResult,
  StudentPackageRef,
  StudentRemindResult,
  StudentScoreRecord,
  StudentScoreSummary,
  StudentScoreUpsertRequest,
  StudentUpsertRequest
} from '../types/starline';

type StudentFormValues = {
  name: string;
  phone: string;
  grade: string;
  schoolName: string;
  guardianName: string;
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

function canWrite(user: CurrentUser) {
  return user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));
}

function canManageScores(user: CurrentUser) {
  return user.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role));
}

export default function Students({ user }: { user: CurrentUser }) {
  const navigate = useNavigate();
  const [filters, setFilters] = useState<StudentFilters>({});
  const [studentForm] = Form.useForm<StudentFormValues>();
  const [editing, setEditing] = useState<Student | null>(null);
  const [studentDrawerOpen, setStudentDrawerOpen] = useState(false);
  const [selected, setSelected] = useState<Student | null>(null);
  const [studentDrawerTab, setStudentDrawerTab] = useState('profile');
  const [directLearningSpaceIds, setDirectLearningSpaceIds] = useState<string[]>([]);
  const [directContentTypeCodes, setDirectContentTypeCodes] = useState<string[]>([]);
  const [directStartsAt, setDirectStartsAt] = useState('');
  const [directEndsAt, setDirectEndsAt] = useState('');
  const [importOpen, setImportOpen] = useState(false);
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const queryClient = useQueryClient();
  const writable = canWrite(user);
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:students');

  const students = useQuery({
    queryKey: ['students', filters],
    queryFn: () => getData<Student[]>('/students', compactParams(filters))
  });
  const learningSpaces = useQuery({ queryKey: ['learning-spaces'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') });
  const detail = useQuery({
    queryKey: ['students', selected?.id, 'detail'],
    enabled: Boolean(selected),
    queryFn: () => getData<StudentDetail>(`/students/${selected?.id}`)
  });

  const availableDirectLearningSpaces = useMemo(
    () => (learningSpaces.data ?? []).filter((item) => item.status === '启用' && item.grade === selected?.grade),
    [learningSpaces.data, selected?.grade]
  );

  const saveStudent = useMutation({
    mutationFn: (values: StudentFormValues) => {
      const body: StudentUpsertRequest = {
        name: values.name,
        phone: values.phone,
        grade: values.grade,
        schoolName: values.schoolName ?? '',
        guardianName: values.guardianName ?? '',
        remark: values.remark ?? '',
        accountStatus: editing ? (values.enabled ? '正常' : '停用') : undefined
      };
      if (editing) return putData<Student>(`/students/${editing.id}`, body);
      return postData<Student>('/students', body);
    },
    onSuccess: () => {
      message.success(editing?.accountStatus === '待审核' ? '审核结果已保存' : editing ? '学生信息已保存' : '学生已新增');
      setStudentDrawerOpen(false);
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

  const createDirectGrant = useMutation({
    mutationFn: () => {
      if (!selected) throw new Error('请先选择学生');
      const body: DirectGrantCreateRequest = {
        studentId: selected.id,
        learningSpaceIds: directLearningSpaceIds,
        contentTypeCodes: directContentTypeCodes,
        startsAt: directStartsAt || undefined,
        endsAt: directEndsAt || undefined
      };
      return postData<DirectGrantResult>('/grants/direct', body);
    },
    onSuccess: (result) => {
      message.success(`已为 ${result.studentName} 开通 ${result.learningSpaces.length} 个课程范围的${result.contentTypes.join('、')}。`);
      setDirectLearningSpaceIds([]);
      setDirectContentTypeCodes([]);
      setDirectStartsAt(toDateTimeInput(new Date()));
      setDirectEndsAt('');
      queryClient.invalidateQueries({ queryKey: ['students'] });
      queryClient.invalidateQueries({ queryKey: ['students', selected?.id, 'detail'] });
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
    },
    onError: (err: Error) => message.error(err.message || '开通失败，请检查课程范围和学习内容。')
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
  const reviewingStudentRequest = editing?.accountStatus === '待审核';
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
    studentForm.setFieldsValue({ name: '', phone: '', grade: '', schoolName: '', guardianName: '', remark: '', enabled: true });
    setStudentDrawerOpen(true);
  }

  function openEdit(student: Student) {
    setEditing(student);
    studentForm.setFieldsValue({
      name: student.name,
      phone: student.phone,
      grade: student.grade,
      schoolName: student.schoolName ?? '',
      guardianName: student.guardianName ?? '',
      remark: student.remark ?? '',
      enabled: student.accountStatus !== '停用'
    });
    setStudentDrawerOpen(true);
  }

  const columns: TableColumnsType<Student> = [
    {
      title: '学生',
      dataIndex: 'name',
      width: 170,
      render: (value, record) => <Space direction="vertical" size={0}><Typography.Text strong>{value}</Typography.Text><Typography.Text type="secondary">{record.phone}</Typography.Text></Space>
    },
    {
      title: '操作',
      width: writable ? 176 : 64,
      render: (_, record) => (
        <Space size={4}>
          <ActionButton tooltip="查看" icon={<EyeOutlined />} onClick={() => openStudentDetail(record)} />
          {writable && record.accountStatus === '正常' && <ActionButton tooltip="开通课程" icon={<UnlockOutlined />} onClick={() => openDirectGrant(record)} />}
          {writable && <ActionButton tooltip="提醒" icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)} />}
          {writable && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />}
        </Space>
      )
    },
    { title: '家长姓名', dataIndex: 'guardianName', width: 110, ellipsis: true, render: (value) => value || '-' },
    { title: '年级', dataIndex: 'grade', width: 88 },
    { title: '学校', dataIndex: 'schoolName', width: 130, ellipsis: true, render: (value) => value || '-' },
    {
      title: '关联状态',
      width: 108,
      render: (_, record) => <Space direction="vertical" size={2}><Tag color={record.officialAccountOpenId ? 'green' : 'orange'}>{record.officialAccountOpenId ? '公众号已关联' : '公众号未关联'}</Tag><Tag color={record.bindStatus === '已绑定' ? 'green' : 'orange'}>{record.bindStatus}</Tag></Space>
    },
    {
      title: '课程',
      dataIndex: 'openedPackageRefs',
      width: 220,
      render: (values: StudentPackageRef[]) => <PackageLinks values={values} onOpen={(packageId) => navigate(`/content?tab=materials&packageId=${encodeURIComponent(packageId)}`)} />
    },
    {
      title: '学习情况',
      width: 146,
      render: (_, record) => <Space direction="vertical" size={2}><Tag color={record.learningStatus.includes('未') ? 'orange' : 'green'}>{record.learningStatus}</Tag><Typography.Text type="secondary">连续 {record.streakDays} 天 · 均分 {record.averageScore ?? '-'}</Typography.Text></Space>
    },
    { title: '账号', dataIndex: 'accountStatus', width: 88, render: (value) => <Tag color={value === '正常' ? 'green' : value === '停用' ? 'default' : 'orange'}>{value}</Tag> },
    { title: '添加时间', dataIndex: 'createdAt', width: 150, ellipsis: true, render: (value) => value || '-' },
    { title: '最近学习', dataIndex: 'lastStudyAt', width: 150, ellipsis: true, render: (value) => value || '-' }
  ];

  function openStudentDetail(student: Student) {
    setSelected(student);
    setStudentDrawerTab('profile');
    setDirectLearningSpaceIds([]);
    setDirectContentTypeCodes([]);
    setDirectStartsAt('');
    setDirectEndsAt('');
  }

  function openDirectGrant(student: Student) {
    setSelected(student);
    setStudentDrawerTab('direct-grant');
    setDirectLearningSpaceIds([]);
    setDirectContentTypeCodes([]);
    setDirectStartsAt(toDateTimeInput(new Date()));
    setDirectEndsAt('');
  }

  if (students.isLoading) return <Skeleton active />;
  if (students.error) return <Alert type="error" message="学生列表加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>学生管理</Typography.Title>
          <Typography.Text type="secondary">查看学生状态，并按课程范围直接开通需要的学习内容。</Typography.Text>
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
        <Card><Statistic title="已开通课程" value={stats.opened} /></Card>
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
                    { label: '家长姓名', value: record.guardianName || '-' },
                    { label: '学校', value: record.schoolName || '-' },
                    { label: '公众号', value: <Tag color={record.officialAccountOpenId ? 'green' : 'orange'}>{record.officialAccountOpenId ? '已关联' : '未关联'}</Tag> },
                    { label: '学习状态', value: <Tag color={record.learningStatus.includes('未') ? 'orange' : 'green'}>{record.learningStatus}</Tag> },
                    { label: '连续学习', value: `${record.streakDays} 天` },
                    { label: '平均分', value: record.averageScore ?? '-' },
                    { label: '徽章', value: `${record.badgeCount} 枚` },
                    { label: '添加时间', value: record.createdAt || '-' },
                    { label: '最近学习', value: record.lastStudyAt || '-' },
                    {
                      label: '最近提交',
                      value: record.lastSubmissionStatus
                        ? <Space size={4}><Tag color={submissionStatusColor(record.lastSubmissionStatus)}>{record.lastSubmissionStatus}</Tag><Typography.Text type="secondary">{record.lastSubmittedAt || '-'}</Typography.Text></Space>
                        : '-'
                    }
                  ]}
                  tags={<PackageLinks values={record.openedPackageRefs} onOpen={(packageId) => navigate(`/content?tab=materials&packageId=${encodeURIComponent(packageId)}`)} />}
                  actions={(
                    <>
                      <ActionButton tooltip="查看" icon={<EyeOutlined />} onClick={() => openStudentDetail(record)} />
                      {writable && record.accountStatus === '正常' && <ActionButton tooltip="开通课程" icon={<UnlockOutlined />} onClick={() => openDirectGrant(record)} />}
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

      <FormDrawer
        title={reviewingStudentRequest ? '审核学生申请' : editing ? '编辑学生' : '新增学生'}
        open={studentDrawerOpen}
        onCancel={() => setStudentDrawerOpen(false)}
        onSubmit={() => studentForm.submit()}
        submitText={reviewingStudentRequest ? '提交审核' : editing ? '保存' : '确定'}
        submitting={saveStudent.isPending}
      >
        <Form form={studentForm} layout="vertical" onFinish={(values) => saveStudent.mutate(values)}>
          {reviewingStudentRequest && <Alert type="info" showIcon message="该学生由家长在小程序提交，审核通过后才会出现在家长的可切换列表中。" style={{ marginBottom: 16 }} />}
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
          <Form.Item name="guardianName" label="家长姓名">
            <Input placeholder="例如：王女士" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={3} placeholder="可填写家长沟通、分班或交接信息" />
          </Form.Item>
          {editing && (
            <Form.Item name="enabled" label={reviewingStudentRequest ? '审核通过并启用账号' : '启用账号'} valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </FormDrawer>

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
            activeKey={studentDrawerTab}
            onChange={setStudentDrawerTab}
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
                    onOpenPackage={(packageId) => navigate(`/content?tab=materials&packageId=${encodeURIComponent(packageId)}`)}
                  />
                )
              },
              {
                key: 'direct-grant',
                label: '开通学习内容',
                children: selected ? (
                  <DirectGrantPanel
                    student={selected}
                    learningSpaces={availableDirectLearningSpaces}
                    loadingLearningSpaces={learningSpaces.isLoading}
                    learningSpacesError={Boolean(learningSpaces.error)}
                    selectedLearningSpaceIds={directLearningSpaceIds}
                    selectedContentTypeCodes={directContentTypeCodes}
                    startsAt={directStartsAt}
                    endsAt={directEndsAt}
                    submitting={createDirectGrant.isPending}
                    onLearningSpaceIdsChange={setDirectLearningSpaceIds}
                    onContentTypeCodesChange={setDirectContentTypeCodes}
                    onStartsAtChange={setDirectStartsAt}
                    onEndsAtChange={setDirectEndsAt}
                    onSubmit={() => createDirectGrant.mutate()}
                  />
                ) : null
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
      <FormDrawer
        title={editing ? '修正成绩' : '录入成绩'}
        open={open}
        onCancel={() => setOpen(false)}
        onSubmit={() => form.submit()}
        submitting={saveScore.isPending}
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
      </FormDrawer>
    </Space>
  );
}

function StudentProfile({
  detail,
  writable,
  generatingBindCode,
  onGenerateBindCode,
  onOpenPackage
}: {
  detail: StudentDetail;
  writable: boolean;
  generatingBindCode: boolean;
  onGenerateBindCode: () => void;
  onOpenPackage: (packageId: string) => void;
}) {
  const codeExpired = Boolean(detail.student.bindCodeExpiresAt && detail.student.bindCodeExpiresAt < new Date().toISOString().slice(0, 10));
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Descriptions column={2} bordered size="small">
        <Descriptions.Item label="姓名">{detail.student.name}</Descriptions.Item>
        <Descriptions.Item label="年级">{detail.student.grade}</Descriptions.Item>
        <Descriptions.Item label="手机号">{detail.student.phone}</Descriptions.Item>
        <Descriptions.Item label="学校">{detail.student.schoolName || '-'}</Descriptions.Item>
        <Descriptions.Item label="家长姓名">{detail.student.guardianName || '-'}</Descriptions.Item>
        <Descriptions.Item label="公众号">{detail.student.officialAccountOpenId ? '已关联' : '未关联'}</Descriptions.Item>
        <Descriptions.Item label="微信绑定">{detail.student.bindStatus}</Descriptions.Item>
        <Descriptions.Item label="账号状态">{detail.student.accountStatus}</Descriptions.Item>
        <Descriptions.Item label="添加时间">{detail.student.createdAt || '-'}</Descriptions.Item>
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
        emptyText="暂未开通课程"
        renderCard={(record) => (
          <InfoCard
            title={record.packageId.startsWith('direct-')
              ? <Typography.Text strong>{record.packageName}</Typography.Text>
              : <Typography.Link onClick={() => onOpenPackage(record.packageId)}>{record.packageName}</Typography.Link>}
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

function DirectGrantPanel({
  student,
  learningSpaces,
  loadingLearningSpaces,
  learningSpacesError,
  selectedLearningSpaceIds,
  selectedContentTypeCodes,
  startsAt,
  endsAt,
  submitting,
  onLearningSpaceIdsChange,
  onContentTypeCodesChange,
  onStartsAtChange,
  onEndsAtChange,
  onSubmit
}: {
  student: Student;
  learningSpaces: LearningSpace[];
  loadingLearningSpaces: boolean;
  learningSpacesError: boolean;
  selectedLearningSpaceIds: string[];
  selectedContentTypeCodes: string[];
  startsAt: string;
  endsAt: string;
  submitting: boolean;
  onLearningSpaceIdsChange: (values: string[]) => void;
  onContentTypeCodesChange: (values: string[]) => void;
  onStartsAtChange: (value: string) => void;
  onEndsAtChange: (value: string) => void;
  onSubmit: () => void;
}) {
  const [selectedSubject, setSelectedSubject] = useState<string>();
  const subjectFilters = useMemo(() => {
    const counts = new Map<string, number>();
    learningSpaces.forEach((space) => counts.set(space.subject, (counts.get(space.subject) ?? 0) + 1));
    return Array.from(counts, ([subject, count]) => ({ subject, count }));
  }, [learningSpaces]);
  const visibleLearningSpaces = useMemo(
    () => selectedSubject ? learningSpaces.filter((space) => space.subject === selectedSubject) : learningSpaces,
    [learningSpaces, selectedSubject]
  );
  const canSubmit = selectedLearningSpaceIds.length > 0 && selectedContentTypeCodes.length > 0;
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message="按需开通学习内容"
        description={`先选择 ${student.name} 要学习的课程范围，再勾选需要开放的课程、习题或学习资料。可设置何时对家长可见；结束时间不填时按当前校历自动计算。`}
      />
      <div>
        <Typography.Text strong>课程范围</Typography.Text>
        <Typography.Paragraph type="secondary" style={{ margin: '4px 0 10px' }}>
          只展示该学生所在年级可开通的课程范围。
        </Typography.Paragraph>
        <Space direction="vertical" size={8} style={{ width: '100%', marginBottom: 12 }}>
          <Space wrap size={[8, 8]}>
            <Tag color="blue">当前年级：{student.grade}</Tag>
            <Typography.Text type="secondary">已选 {selectedLearningSpaceIds.length} 个课程范围</Typography.Text>
          </Space>
          <div role="group" aria-label="科目筛选">
            <Space wrap size={[8, 8]}>
              <Typography.Text>科目</Typography.Text>
              <Button size="small" type={!selectedSubject ? 'primary' : 'default'} onClick={() => setSelectedSubject(undefined)}>
                全部（{learningSpaces.length}）
              </Button>
              {subjectFilters.map(({ subject, count }) => (
                <Button
                  key={subject}
                  size="small"
                  type={selectedSubject === subject ? 'primary' : 'default'}
                  onClick={() => setSelectedSubject(subject)}
                >
                  {subject}（{count}）
                </Button>
              ))}
            </Space>
          </div>
        </Space>
        {loadingLearningSpaces ? <Skeleton active paragraph={{ rows: 3 }} /> : learningSpacesError ? (
          <Alert type="error" showIcon message="课程范围加载失败，请关闭抽屉后重试。" />
        ) : learningSpaces.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该年级还没有可开通的课程范围。" />
        ) : visibleLearningSpaces.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该科目暂无可开通课程范围。" />
        ) : (
          <Checkbox.Group
            aria-label="课程范围"
            value={selectedLearningSpaceIds}
            onChange={(values) => onLearningSpaceIdsChange(values.filter((value): value is string => typeof value === 'string'))}
          >
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              {visibleLearningSpaces.map((space) => (
                <Checkbox key={space.id} value={space.id}>
                  {space.name}
                </Checkbox>
              ))}
            </Space>
          </Checkbox.Group>
        )}
      </div>
      <div>
        <Typography.Text strong>学习内容</Typography.Text>
        <Typography.Paragraph type="secondary" style={{ margin: '4px 0 10px' }}>
          本次勾选的类型会立即开通；已经开通的内容不会因取消勾选而关闭。
        </Typography.Paragraph>
        <Checkbox.Group
          aria-label="学习内容"
          value={selectedContentTypeCodes}
          onChange={(values) => onContentTypeCodesChange(values.filter((value): value is string => typeof value === 'string'))}
        >
          <Space wrap size={[20, 8]}>
            <Checkbox value="course">课程</Checkbox>
            <Checkbox value="question">习题</Checkbox>
            <Checkbox value="handout">学习资料</Checkbox>
          </Space>
        </Checkbox.Group>
      </div>
      <div>
        <Typography.Text strong>生效时间</Typography.Text>
        <Typography.Paragraph type="secondary" style={{ margin: '4px 0 10px' }}>
          开通时间默认现在。未到开通时间的课程不会在学生端显示；结束时间留空时按当前校历到期。
        </Typography.Paragraph>
        <Space direction="vertical" size={8} style={{ width: '100%' }}>
          <label>
            <Typography.Text>开通时间</Typography.Text>
            <input aria-label="开通时间" className="ant-input" type="datetime-local" value={startsAt} onChange={(event) => onStartsAtChange(event.target.value)} style={{ marginTop: 4 }} />
          </label>
          <label>
            <Typography.Text>结束时间（选填）</Typography.Text>
            <input aria-label="结束时间" className="ant-input" type="datetime-local" min={startsAt} value={endsAt} onChange={(event) => onEndsAtChange(event.target.value)} style={{ marginTop: 4 }} />
          </label>
        </Space>
      </div>
      <Button
        type="primary"
        icon={<UnlockOutlined />}
        loading={submitting}
        disabled={!canSubmit || loadingLearningSpaces || learningSpacesError}
        onClick={onSubmit}
      >
        确认开通
      </Button>
    </Space>
  );
}

function toDateTimeInput(value: Date) {
  const offset = value.getTimezoneOffset() * 60_000;
  return new Date(value.getTime() - offset).toISOString().slice(0, 16);
}

function PackageLinks({ values, onOpen }: { values?: StudentPackageRef[]; onOpen: (packageId: string) => void }) {
  if (!values?.length) return <Typography.Text type="secondary">暂未开通课程</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {values.map((item) => (
        item.packageId.startsWith('direct-')
          ? <Tag key={item.packageId} color="blue">{item.packageName}</Tag>
          : <Typography.Link key={item.packageId} onClick={() => onOpen(item.packageId)}><Tag color="blue">{item.packageName}</Tag></Typography.Link>
      ))}
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
