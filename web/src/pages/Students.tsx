import {
  Alert,
  Badge,
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
  Tooltip,
  Typography,
  Upload,
  message
} from 'antd';
import type { TableColumnsType, UploadFile } from 'antd';
import { BellOutlined, EditOutlined, EyeOutlined, ImportOutlined, PlusOutlined, UnlockOutlined, UploadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { getData, postData, postForm, putData } from '../services/http';
import { FormDrawer } from '../components/FormDrawer';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../components/ListViews';
import { gradeOptions as curriculumGradeOptions, subjectOptions } from '../utils/curriculum';
import type {
  CurrentUser,
  DirectGrantReplaceRequest,
  DirectGrantResult,
  DirectGrantSelection,
  LearningSpace,
  Student,
  StudentDetail,
  StudentImportResult,
  StudentPackageRef,
  StudentRemindResult,
  StudentScoreRecord,
  StudentScoreSummary,
  StudentScoreUpsertRequest,
	StudentUpsertRequest,
	SubjectMetadata,
	Teacher,
	TutoringAssignment,
	TutoringAssignmentSummary,
	TutoringAssignmentCreateRequest,
	ScheduleClass,
	LessonFeedback
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
  followUpState?: string;
};

function canWrite(user: CurrentUser) {
  return user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));
}

function canManageScores(user: CurrentUser) {
  return user.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role));
}

function bindStatusText(status: string) {
  return status === '已绑定' ? '已关联微信' : '待关联微信';
}

function packageStatusTag(student: Student) {
  if (student.followUpStatus === '待跟进') return <Tag color="red">待跟进</Tag>;
  if ((student.openedPackages?.length ?? 0) > 0) return <Tag color="green">已开通</Tag>;
  return <Tag>未开通</Tag>;
}

function packageStatusInfo(student: Student) {
  return (
    <Space direction="vertical" size={2}>
      {packageStatusTag(student)}
      <ExpiryReminder endTime={student.effectiveUntil} />
    </Space>
  );
}

function TutoringTeacherNames({ assignments }: { assignments?: TutoringAssignmentSummary[] }) {
  if (!assignments?.length) return <Typography.Text type="secondary">未分配老师</Typography.Text>;
  const visible = assignments.slice(0, 2);
  return (
    <Tooltip
      title={(
        <Space direction="vertical" size={4}>
          {assignments.map((item) => (
            <Typography.Text key={`${item.teacherId}-${item.subjectName}-${item.levelCode}-${item.role}`} style={{ color: '#fff' }}>
              {item.teacherName} · {item.role === 'primary' ? '主辅导' : '协作'} · {item.subjectName} {item.levelCode}级 · {item.startsAt} 起
            </Typography.Text>
          ))}
        </Space>
      )}
    >
      <Space size={[4, 4]} wrap>
        {visible.map((item) => <Tag key={`${item.teacherId}-${item.subjectName}-${item.levelCode}-${item.role}`} color={item.role === 'primary' ? 'blue' : 'default'}>{item.teacherName}</Tag>)}
        {assignments.length > visible.length && <Tag>+{assignments.length - visible.length}</Tag>}
      </Space>
    </Tooltip>
  );
}

export default function Students({ user }: { user: CurrentUser }) {
  const [filters, setFilters] = useState<StudentFilters>({});
  const [studentForm] = Form.useForm<StudentFormValues>();
  const [editing, setEditing] = useState<Student | null>(null);
  const [studentDrawerOpen, setStudentDrawerOpen] = useState(false);
  const [selected, setSelected] = useState<Student | null>(null);
  const [studentDrawerTab, setStudentDrawerTab] = useState('profile');
  const [directSelections, setDirectSelections] = useState<DirectGrantSelection[]>([]);
  const [initialDirectSelections, setInitialDirectSelections] = useState<DirectGrantSelection[]>([]);
  const [directStartsAt, setDirectStartsAt] = useState('');
  const [directEndsAt, setDirectEndsAt] = useState('');
  const [directPeriodChanged, setDirectPeriodChanged] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const queryClient = useQueryClient();
  const writable = canWrite(user);
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:students');

  const students = useQuery({
    queryKey: ['students', filters],
    queryFn: () => getData<Student[]>('/students', compactParams(filters))
  });
  const allStudents = useQuery({ queryKey: ['students', 'summary'], queryFn: () => getData<Student[]>('/students') });
  const learningSpaces = useQuery({ queryKey: ['learning-spaces'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') });
	const teachers = useQuery({ queryKey: ['teachers', 'tutoring-assignment'], enabled: writable, queryFn: () => getData<Teacher[]>('/teachers') });
	const subjects = useQuery({ queryKey: ['subjects', 'tutoring-assignment'], enabled: writable, queryFn: () => getData<SubjectMetadata[]>('/subjects') });
  const detail = useQuery({
    queryKey: ['students', selected?.id, 'detail'],
    enabled: Boolean(selected),
    queryFn: () => getData<StudentDetail>(`/students/${selected?.id}`)
  });

  useEffect(() => {
    if (!selected || detail.data?.student.id !== selected.id) return;
    const selections = detail.data.grants
      .filter((grant) => grant.isDirect && grant.permissionState !== '已到期')
      .flatMap((grant) => grant.learningSpaceIds.map((learningSpaceId) => ({
        learningSpaceId,
        contentTypeCodes: grant.contentTypes.map(contentTypeCode)
      })));
    setDirectSelections(selections);
    setInitialDirectSelections(selections);
  }, [detail.data, selected]);

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
      message.success(editing ? '学生信息已保存' : '学生已新增');
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
      const body: DirectGrantReplaceRequest = {
        studentId: selected.id,
        selections: directSelections.filter((selection) => selection.contentTypeCodes.length > 0),
        startsAt: directPeriodChanged ? (directStartsAt || undefined) : undefined,
        endsAt: directPeriodChanged ? (directEndsAt || undefined) : undefined
      };
      return putData<DirectGrantResult>('/grants/direct', body);
    },
    onSuccess: (result) => {
      message.success(result.learningSpaces.length ? `已保存 ${result.studentName} 的直接开通内容。` : `已取消 ${result.studentName} 的全部直接开通内容。`);
      queryClient.invalidateQueries({ queryKey: ['students'] });
      queryClient.invalidateQueries({ queryKey: ['students', selected?.id, 'detail'] });
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
    },
    onError: (err: Error) => message.error(err.message || '保存失败，请检查课程范围和学习内容。')
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
  const allRows = allStudents.data ?? [];
  const gradeOptions = useMemo(() => uniqueOptions(rows.map((item) => item.grade)), [rows]);
  const learningOptions = useMemo(() => uniqueOptions(rows.map((item) => item.learningStatus)), [rows]);
  const accountOptions = useMemo(() => uniqueOptions(rows.map((item) => item.accountStatus)), [rows]);
  const stats = useMemo(() => ({
    total: allRows.length,
    opened: allRows.filter((item) => (item.openedPackages?.length ?? 0) > 0).length,
    waiting: allRows.filter((item) => item.followUpStatus === '待跟进').length,
    disabled: allRows.filter((item) => item.accountStatus === '停用').length
  }), [allRows]);

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
    { title: '年级', dataIndex: 'grade', width: 88 },
    { title: '学校', dataIndex: 'schoolName', width: 130, ellipsis: true, render: (value) => value || '-' },
    {
      title: '学生',
      dataIndex: 'name',
      width: 170,
      render: (value, record) => <Space direction="vertical" size={0}><Badge dot={record.followUpStatus === '待跟进'} color="#ff4d4f"><Typography.Text strong className="student-name">{value}</Typography.Text></Badge><Typography.Text type="secondary">{record.phone}</Typography.Text></Space>
    },
    {
      title: '操作',
      width: writable ? 176 : 64,
      render: (_, record) => (
        <Space size={4}>
          <ActionButton tooltip="查看" icon={<EyeOutlined />} onClick={() => openStudentDetail(record)} />
          {writable && record.accountStatus === '正常' && <ActionButton tooltip="开通学习内容" icon={<UnlockOutlined />} onClick={() => openDirectGrant(record)} />}
          {writable && <ActionButton tooltip="提醒" icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)} />}
          {writable && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />}
        </Space>
      )
    },
    { title: '家长姓名', dataIndex: 'guardianName', width: 110, ellipsis: true, render: (value) => value || '-' },
    {
      title: '微信关联',
      width: 108,
      render: (_, record) => <Space direction="vertical" size={2}><Tag color={record.officialAccountOpenId ? 'green' : 'orange'}>{record.officialAccountOpenId ? '公众号已关联' : '公众号未关联'}</Tag><Tag color={record.bindStatus === '已绑定' ? 'green' : 'orange'}>{bindStatusText(record.bindStatus)}</Tag></Space>
    },
    { title: '套餐状态', width: 144, render: (_, record) => packageStatusInfo(record) },
    {
      title: '课程',
      dataIndex: 'openedPackageRefs',
      width: 220,
      render: (values: StudentPackageRef[], record) => <PackageLinks values={values} onOpen={() => openStudentDetail(record, 'courses')} />
    },
    {
      title: '辅导老师',
      dataIndex: 'activeTutoringAssignments',
      width: 190,
      render: (values: TutoringAssignmentSummary[]) => <TutoringTeacherNames assignments={values} />
    }
  ];

  function openStudentDetail(student: Student, tab = 'profile') {
    setSelected(student);
    setStudentDrawerTab(tab);
    setDirectSelections([]);
    setInitialDirectSelections([]);
    setDirectStartsAt('');
    setDirectEndsAt('');
    setDirectPeriodChanged(false);
  }

  function openDirectGrant(student: Student) {
    setSelected(student);
    setStudentDrawerTab('direct-grant');
    setDirectSelections([]);
    setInitialDirectSelections([]);
    setDirectStartsAt(toDateTimeInput(new Date()));
    setDirectEndsAt('');
    setDirectPeriodChanged(false);
  }

  function submitDirectGrant() {
    const previous = selectionKeys(initialDirectSelections);
    const next = selectionKeys(directSelections);
    const removed = [...previous].filter((key) => !next.has(key));
    if (removed.length === 0) {
      createDirectGrant.mutate();
      return;
    }
    Modal.confirm({
      title: '确认取消已开通内容？',
      content: `将取消 ${removed.length} 项直接开通内容；这不会影响套餐或其他学生。`,
      okText: '确认取消',
      okButtonProps: { danger: true },
      cancelText: '暂不取消',
      onOk: () => createDirectGrant.mutate()
    });
  }

  if (students.isLoading) return <Skeleton active />;
  if (students.error) return <Alert type="error" message="学生列表加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>{writable ? '学生管理' : '我的学生'}</Typography.Title>
          <Typography.Text type="secondary">{writable ? '先确定辅导老师，再排课、批改和跟进；所有未开通课程的学生会自动归入待跟进。' : '这里只显示已分配给我的学生，避免跨班或跨老师查看。'}</Typography.Text>
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
        <Card hoverable onClick={() => setFilters((prev) => prev.followUpState === '待跟进' ? {} : { followUpState: '待跟进' })}><Statistic title="待跟进（点击筛选）" value={stats.waiting} valueStyle={{ color: stats.waiting ? '#cf1322' : undefined }} /></Card>
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
                placeholder="套餐开通状态"
                options={[{ label: '已开通', value: '已开通' }, { label: '未开通', value: '未开通' }]}
                style={{ width: 140 }}
                onChange={(packageState) => setFilters((prev) => ({ ...prev, packageState }))}
              />
              <Select allowClear value={filters.followUpState} placeholder="跟进状态" options={[{ label: '待跟进', value: '待跟进' }]} style={{ width: 140 }} onChange={(followUpState) => setFilters((prev) => ({ ...prev, followUpState }))} />
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
                  className={record.followUpStatus === '待跟进' ? 'student-follow-up-card' : undefined}
                  title={<Badge dot={record.followUpStatus === '待跟进'} color="#ff4d4f">{record.name}</Badge>}
                  subtitle={`${record.grade} · ${record.phone}`}
                  status={<Space direction="vertical" size={2} align="end"><Space size={4}><Tag color={record.accountStatus === '正常' ? 'green' : record.accountStatus === '停用' ? 'default' : 'orange'}>{record.accountStatus}</Tag>{packageStatusTag(record)}</Space>{packageExpiryReminder(record.effectiveUntil) && <ExpiryReminder endTime={record.effectiveUntil} />}</Space>}
                  fields={[
                    { label: '微信关联', value: <Tag color={record.bindStatus === '已绑定' ? 'green' : 'orange'}>{bindStatusText(record.bindStatus)}</Tag> },
                    { label: '家长姓名', value: record.guardianName || '-' },
                    { label: '学校', value: record.schoolName || '-' },
                    { label: '公众号', value: <Tag color={record.officialAccountOpenId ? 'green' : 'orange'}>{record.officialAccountOpenId ? '已关联' : '未关联'}</Tag> },
                    { label: '辅导老师', value: <TutoringTeacherNames assignments={record.activeTutoringAssignments} /> },
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
                  tags={<PackageLinks values={record.openedPackageRefs} onOpen={() => openStudentDetail(record, 'courses')} />}
                  actions={(
                    <>
                      <ActionButton tooltip="查看" icon={<EyeOutlined />} onClick={() => openStudentDetail(record)} />
                      {writable && record.accountStatus === '正常' && <ActionButton tooltip="开通学习内容" icon={<UnlockOutlined />} onClick={() => openDirectGrant(record)} />}
                      {writable && <ActionButton tooltip="提醒" icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)} />}
                      {writable && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />}
                    </>
                  )}
                />
              )}
            />
          ) : (
            rows.length === 0 ? <Empty description="还没有学生，先新增学生或批量导入。" /> : <div className="student-table-scroll"><Table className="student-table" rowKey="id" columns={columns} dataSource={rows} rowClassName={(record) => record.followUpStatus === '待跟进' ? 'student-follow-up-row' : ''} tableLayout="fixed" scroll={{ x: 1280 }} pagination={{ pageSize: 8 }} /></div>
          )}
        </div>
      </Card>

      <FormDrawer
        title={editing ? '编辑学生' : '新增学生'}
        open={studentDrawerOpen}
        onCancel={() => setStudentDrawerOpen(false)}
        onSubmit={() => studentForm.submit()}
        submitText={editing ? '保存' : '确定'}
        submitting={saveStudent.isPending}
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
          <Form.Item name="guardianName" label="家长姓名">
            <Input placeholder="例如：王女士" />
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
                  />
                )
              },
			  { key: 'courses', label: '开通课程', children: <StudentCourses detail={detail.data} /> },
			  {
				key: 'tutoring',
				label: writable ? '辅导老师' : '我的辅导关系',
				children: selected ? <TutoringAssignmentPanel student={selected} writable={writable} teachers={teachers.data ?? []} subjects={subjects.data ?? []} learningSpaces={learningSpaces.data ?? []} /> : null
			  },
			  { key: 'lesson-feedback', label: '课后反馈', children: selected ? <LessonFeedbackPanel student={selected} user={user} /> : null },
			  ...(writable ? [{
                key: 'direct-grant',
                label: '开通学习内容',
                children: selected ? (
                  <DirectGrantPanel
                    student={selected}
                    learningSpaces={availableDirectLearningSpaces}
                    loadingLearningSpaces={learningSpaces.isLoading}
                    learningSpacesError={Boolean(learningSpaces.error)}
                    selections={directSelections}
                    startsAt={directStartsAt}
                    endsAt={directEndsAt}
                    submitting={createDirectGrant.isPending}
                    onSelectionsChange={setDirectSelections}
                    onStartsAtChange={(value) => { setDirectStartsAt(value); setDirectPeriodChanged(true); }}
                    onEndsAtChange={(value) => { setDirectEndsAt(value); setDirectPeriodChanged(true); }}
                    onSubmit={submitDirectGrant}
                  />
                ) : null
              }] : []),
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

type TutoringAction = { kind: 'end' | 'transfer'; assignment: TutoringAssignment };

function LessonFeedbackPanel({ student, user }: { student: Student; user: CurrentUser }) {
	const [form] = Form.useForm<Pick<LessonFeedback, 'summary' | 'homework' | 'nextStep'>>();
	const [selectedClass, setSelectedClass] = useState<ScheduleClass | null>(null);
	const client = useQueryClient();
	const canWrite = user.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role));
	const classes = useQuery({ queryKey: ['schedule-classes', 'feedback'], enabled: canWrite, queryFn: () => getData<ScheduleClass[]>('/schedule-classes') });
	const eligible = (classes.data ?? []).filter((item) => item.students.some((row) => row.id === student.id) && item.auditStatus === '已通过' && item.status !== '已取消');
	const feedbacks = useQuery({ queryKey: ['lesson-feedbacks', selectedClass?.id], enabled: Boolean(selectedClass), queryFn: () => getData<LessonFeedback[]>(`/schedule-classes/${selectedClass?.id}/feedbacks`) });
	const save = useMutation({
		mutationFn: (values: Pick<LessonFeedback, 'summary' | 'homework' | 'nextStep'>) => {
			if (!selectedClass) throw new Error('请选择课次');
			return putData<LessonFeedback>(`/schedule-classes/${selectedClass.id}/feedbacks`, { studentId: student.id, ...values });
		},
		onSuccess: () => {
			message.success('课后反馈已保存，并同步到家长可见的成长记录。');
			client.invalidateQueries({ queryKey: ['lesson-feedbacks', selectedClass?.id] });
			client.invalidateQueries({ queryKey: ['students', student.id, 'detail'] });
		},
		onError: (error) => message.error(error instanceof Error ? error.message : '保存失败，请稍后重试。')
	});
	useEffect(() => {
		if (!selectedClass || feedbacks.isLoading) return;
		const existing = (feedbacks.data ?? []).find((row) => row.studentId === student.id);
		form.setFieldsValue(existing ? { summary: existing.summary, homework: existing.homework, nextStep: existing.nextStep } : { summary: '', homework: '', nextStep: '' });
	}, [feedbacks.data, feedbacks.isLoading, form, selectedClass, student.id]);

	function chooseClass(id: string) {
		const item = eligible.find((row) => row.id === id) ?? null;
		setSelectedClass(item);
	}

	if (!canWrite) return <Empty description="课后反馈由授课老师填写。" />;
	return <Space direction="vertical" size="middle" style={{ width: '100%' }}>
		<Typography.Text type="secondary">选择已通过审核的课次，为这名学生记录课堂表现、课后任务和下一步建议。</Typography.Text>
		<Select placeholder="选择课次" value={selectedClass?.id} loading={classes.isLoading} onChange={chooseClass} options={eligible.map((item) => ({ value: item.id, label: `${item.lessonDate} · ${item.courseName} · ${item.teacherName}` }))} />
		{eligible.length === 0 && !classes.isLoading && <Empty description="暂无可填写反馈的已生效课次。" />}
		{selectedClass && <Form form={form} layout="vertical" onFinish={(values) => save.mutate(values)}>
			<Form.Item name="summary" label="课堂表现" rules={[{ required: true, message: '请填写本节课表现' }]}><Input.TextArea rows={3} placeholder="用家长能理解的话说明收获和需关注的地方" /></Form.Item>
			<Form.Item name="homework" label="课后任务"><Input.TextArea rows={2} /></Form.Item>
			<Form.Item name="nextStep" label="下一步建议"><Input.TextArea rows={2} /></Form.Item>
			<Button type="primary" loading={save.isPending} onClick={() => form.submit()}>保存并同步成长记录</Button>
		</Form>}
		{feedbacks.data?.find((row) => row.studentId === student.id) && <Alert type="success" message={`已保存，最近更新于 ${feedbacks.data.find((row) => row.studentId === student.id)?.updatedAt}`} />}
	</Space>;
}

function TutoringAssignmentPanel({
  student,
  writable,
  teachers,
  subjects,
  learningSpaces
}: {
  student: Student;
  writable: boolean;
  teachers: Teacher[];
  subjects: SubjectMetadata[];
  learningSpaces: LearningSpace[];
}) {
  const [createForm] = Form.useForm<TutoringAssignmentCreateRequest>();
  const [actionForm] = Form.useForm<{ teacherId?: string; startsAt: string; reason: string }>();
  const [createOpen, setCreateOpen] = useState(false);
  const [action, setAction] = useState<TutoringAction | null>(null);
  const client = useQueryClient();
  const queryKey = ['students', student.id, 'tutoring-assignments'];
  const assignments = useQuery({ queryKey, queryFn: () => getData<TutoringAssignment[]>(`/students/${student.id}/tutoring-assignments`) });
  const selectedSubjectId = Form.useWatch('subjectId', createForm);
  const selectedLevelCode = Form.useWatch('levelCode', createForm);
  const studentSpaces = useMemo(() => learningSpaces.filter((space) => space.status === '启用' && space.grade === student.grade), [learningSpaces, student.grade]);
  const selectableSubjects = useMemo(() => subjects.filter((subject) => studentSpaces.some((space) => space.subject === subject.name)), [subjects, studentSpaces]);
  const selectedSubject = selectableSubjects.find((subject) => subject.id === selectedSubjectId);
  const levelOptions = useMemo(() => uniqueOptions(studentSpaces.filter((space) => !selectedSubject || space.subject === selectedSubject.name).map((space) => space.level || 'S')), [studentSpaces, selectedSubject]);
  const selectableTeachers = useMemo(() => teachers.filter((teacher) => teacher.accountStatus === '正常' && studentSpaces.some((space) => (!selectedSubject || space.subject === selectedSubject.name) && (!selectedLevelCode || (space.level || 'S') === selectedLevelCode) && teacher.learningSpaceIds.includes(space.id))), [teachers, studentSpaces, selectedSubject, selectedLevelCode]);
  const transferableTeachers = useMemo(() => {
    if (!action || action.kind !== 'transfer') return [];
    return teachers.filter((teacher) => teacher.accountStatus === '正常' && teacher.id !== action.assignment.teacherId && studentSpaces.some((space) => space.subject === action.assignment.subjectName && (space.level || 'S') === action.assignment.levelCode && teacher.learningSpaceIds.includes(space.id)));
  }, [action, teachers, studentSpaces]);
  const create = useMutation({
    mutationFn: (values: TutoringAssignmentCreateRequest) => postData<TutoringAssignment>(`/students/${student.id}/tutoring-assignments`, { ...values, role: values.role || 'primary' }),
    onSuccess: () => {
      message.success('辅导关系已生效，老师现在可以查看这名学生。');
      setCreateOpen(false);
      createForm.resetFields();
      client.invalidateQueries({ queryKey });
      client.invalidateQueries({ queryKey: ['students'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败，请检查老师的教学范围。')
  });
  const update = useMutation({
    mutationFn: (values: { teacherId?: string; startsAt: string; reason: string }) => {
      if (!action) throw new Error('请选择要处理的辅导关系');
      const path = action.kind === 'transfer' ? `/teacher-assignments/${action.assignment.id}/transfer` : `/teacher-assignments/${action.assignment.id}/end`;
      return postData<TutoringAssignment>(path, action.kind === 'transfer'
        ? { teacherId: values.teacherId, startsAt: values.startsAt, reason: values.reason, version: action.assignment.version }
        : { endsAt: values.startsAt, reason: values.reason, version: action.assignment.version });
    },
    onSuccess: () => {
      message.success(action?.kind === 'transfer' ? '已转交给新的辅导老师。' : '辅导关系已结束。');
      setAction(null);
      actionForm.resetFields();
      client.invalidateQueries({ queryKey });
      client.invalidateQueries({ queryKey: ['students'] });
      client.invalidateQueries({ queryKey: ['review'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '操作失败，请刷新后重试。')
  });

  function openCreate() {
    createForm.setFieldsValue({ teacherId: undefined as unknown as string, subjectId: undefined as unknown as string, levelCode: undefined as unknown as string, role: 'primary', startsAt: new Date().toISOString().slice(0, 10) });
    setCreateOpen(true);
  }

  function openAction(next: TutoringAction) {
    setAction(next);
    actionForm.setFieldsValue({ teacherId: undefined, startsAt: new Date().toISOString().slice(0, 10), reason: '' });
  }

  if (assignments.isLoading) return <Skeleton active />;
  if (assignments.error) return <Alert type="error" message="辅导关系加载失败，请稍后重试。" />;
  const rows = assignments.data ?? [];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12 }}>
        <Typography.Text type="secondary">{writable ? '主辅导老师决定学生归属、默认排课范围和新产生的批改任务；协作老师仅用于辅助记录。' : '只有有效辅导关系内的学生才会出现在我的工作范围。'}</Typography.Text>
        {writable && <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>分配老师</Button>}
      </div>
      {rows.length === 0 ? <Empty description={writable ? '还没有辅导老师，分配后才能进入老师的工作范围。' : '暂未找到有效辅导关系。'} /> : <CardList rows={rows} rowKey={(item) => item.id} emptyText="还没有辅导关系" renderCard={(item) => <InfoCard title={`${item.subjectName} · ${item.levelCode} · ${item.teacherName}`} subtitle={`${item.startsAt} 起${item.endsAt ? `，${item.endsAt} 结束` : ''}`} status={<Tag color={item.status === 'active' ? 'green' : 'default'}>{item.status === 'active' ? (item.role === 'primary' ? '主辅导老师' : '协作老师') : '已结束'}</Tag>} fields={[{ label: '分配人', value: item.assignedBy || '-' }, { label: '结束原因', value: item.endedReason || '-' }]} actions={writable && item.status === 'active' ? <Space size={4}><Button size="small" onClick={() => openAction({ kind: 'transfer', assignment: item })}>转交</Button><Button size="small" danger onClick={() => openAction({ kind: 'end', assignment: item })}>结束</Button></Space> : undefined} />} />}
      <FormDrawer title="分配辅导老师" open={createOpen} onCancel={() => setCreateOpen(false)} onSubmit={() => createForm.submit()} submitting={create.isPending} submitText="确认分配">
        <Form form={createForm} layout="vertical" onFinish={(values) => create.mutate(values)}>
          <Form.Item name="subjectId" label="辅导学科" rules={[{ required: true, message: '请选择学科' }]}><Select placeholder="先选择学科" options={selectableSubjects.map((subject) => ({ value: subject.id, label: subject.name }))} onChange={() => createForm.setFieldsValue({ levelCode: undefined as unknown as string, teacherId: undefined as unknown as string })} /></Form.Item>
          <Form.Item name="levelCode" label="课程等级" rules={[{ required: true, message: '请选择等级' }]}><Select placeholder="请选择等级" options={levelOptions} onChange={() => createForm.setFieldsValue({ teacherId: undefined as unknown as string })} /></Form.Item>
          <Form.Item name="teacherId" label="辅导老师" rules={[{ required: true, message: '请选择老师' }]}><Select placeholder="只显示覆盖该年级、学科和等级的老师" options={selectableTeachers.map((teacher) => ({ value: teacher.id, label: teacher.name }))} /></Form.Item>
          <Form.Item name="role" label="辅导角色" rules={[{ required: true }]}><Select options={[{ value: 'primary', label: '主辅导老师' }, { value: 'assistant', label: '协作老师' }]} /></Form.Item>
          <Form.Item name="startsAt" label="生效日期" rules={[{ required: true, message: '请选择生效日期' }]}><Input type="date" /></Form.Item>
        </Form>
      </FormDrawer>
      <FormDrawer title={action?.kind === 'transfer' ? '转交辅导关系' : '结束辅导关系'} open={Boolean(action)} onCancel={() => setAction(null)} onSubmit={() => actionForm.submit()} submitting={update.isPending} submitText={action?.kind === 'transfer' ? '确认转交' : '确认结束'}>
        <Form form={actionForm} layout="vertical" onFinish={(values) => update.mutate(values)}>
          {action?.kind === 'transfer' && <Form.Item name="teacherId" label="新的辅导老师" rules={[{ required: true, message: '请选择新的辅导老师' }]}><Select placeholder="只显示覆盖当前学生年级、学科和等级的老师" options={transferableTeachers.map((teacher) => ({ value: teacher.id, label: teacher.name }))} /></Form.Item>}
          <Form.Item name="startsAt" label={action?.kind === 'transfer' ? '转交生效日期' : '结束日期'} rules={[{ required: true, message: '请选择日期' }]}><Input type="date" /></Form.Item>
          <Form.Item name="reason" label={action?.kind === 'transfer' ? '转交原因' : '结束原因'} rules={[{ required: true, message: '请填写原因，方便后续交接追溯' }]}><Input.TextArea rows={3} placeholder="例如：老师请假、学科调整或学生结课" /></Form.Item>
        </Form>
      </FormDrawer>
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
    </Space>
  );
}

function StudentCourses({ detail }: { detail: StudentDetail }) {
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Text type="secondary">以下内容仅代表当前学生实际已开通的学习权限。</Typography.Text>
      <CardList
        rows={detail.grants}
        rowKey={(record) => record.packageId}
        emptyText="暂未开通课程"
        renderCard={(record) => (
          <InfoCard
            title={<Space size={6}><Typography.Text strong>{record.packageName}</Typography.Text>{record.isDirect && <Tag color="cyan">直接开通</Tag>}</Space>}
            subtitle={`${record.startsAt || '-'} 至 ${record.effectiveUntil || '-'}`}
            status={tagStatus(record.permissionState)}
            fields={[
              { label: '课程范围', value: record.learningSpaces.join('、') || '暂无' },
              { label: '开通类型', value: record.contentTypes.join('、') || '暂无' },
              { label: '课程', value: record.openCourses.join('、') || '暂无' },
              { label: '讲义', value: record.openMaterials.join('、') || '暂无' },
              { label: '题目', value: record.openHomework.join('、') || '暂无' }
            ]}
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
  selections,
  startsAt,
  endsAt,
  submitting,
  onSelectionsChange,
  onStartsAtChange,
  onEndsAtChange,
  onSubmit
}: {
  student: Student;
  learningSpaces: LearningSpace[];
  loadingLearningSpaces: boolean;
  learningSpacesError: boolean;
  selections: DirectGrantSelection[];
  startsAt: string;
  endsAt: string;
  submitting: boolean;
  onSelectionsChange: (values: DirectGrantSelection[]) => void;
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
  const selectedCount = selections.filter((selection) => selection.contentTypeCodes.length > 0).length;
  const selectionFor = (learningSpaceId: string) => selections.find((selection) => selection.learningSpaceId === learningSpaceId)?.contentTypeCodes ?? [];
  const changeSelection = (learningSpaceId: string, values: string[]) => {
    const next = selections.filter((selection) => selection.learningSpaceId !== learningSpaceId);
    if (values.length > 0) next.push({ learningSpaceId, contentTypeCodes: values });
    onSelectionsChange(next);
  };
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message="按需开通学习内容"
        description={`已开通的内容会自动勾选。取消勾选后保存，即可撤销 ${student.name} 的手动开通内容；不会影响套餐或其他学生。`}
      />
      <div>
        <Typography.Text strong>课程范围与学习内容</Typography.Text>
        <Typography.Paragraph type="secondary" style={{ margin: '4px 0 10px' }}>
          每个课程范围可单独开通课程、讲义和题目；取消该范围全部勾选会撤销该范围的直接开通。
        </Typography.Paragraph>
        <Space direction="vertical" size={8} style={{ width: '100%', marginBottom: 12 }}>
          <Space wrap size={[8, 8]}>
            <Tag color="blue">当前年级：{student.grade}</Tag>
            <Typography.Text type="secondary">已选 {selectedCount} 个课程范围</Typography.Text>
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
          <Space direction="vertical" size={10} style={{ width: '100%' }}>
            {visibleLearningSpaces.map((space) => (
              <Card size="small" key={space.id} title={space.name}>
                <Checkbox.Group
                  aria-label={`${space.name}学习内容`}
                  value={selectionFor(space.id)}
                  onChange={(values) => changeSelection(space.id, values.filter((value): value is string => typeof value === 'string'))}
                >
                  <Space wrap size={[20, 8]}>
                    <Checkbox value="course">课程</Checkbox>
                    <Checkbox value="handout">讲义</Checkbox>
                    <Checkbox value="question">题目</Checkbox>
                  </Space>
                </Checkbox.Group>
              </Card>
            ))}
          </Space>
        )}
      </div>
      <div>
        <Typography.Text strong>生效时间</Typography.Text>
        <Typography.Paragraph type="secondary" style={{ margin: '4px 0 10px' }}>
          新开通内容默认从现在生效；不修改时间时，已开通内容会保留原有效期。手动修改时间后，会统一更新本次保留内容的有效期。
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
        disabled={loadingLearningSpaces || learningSpacesError}
        onClick={onSubmit}
      >
        保存学习内容
      </Button>
    </Space>
  );
}

function toDateTimeInput(value: Date) {
  const offset = value.getTimezoneOffset() * 60_000;
  return new Date(value.getTime() - offset).toISOString().slice(0, 16);
}

function contentTypeCode(value: string) {
  if (value === '课程') return 'course';
  if (value === '学习资料') return 'handout';
  if (value === '题') return 'question';
  return value;
}

function selectionKeys(selections: DirectGrantSelection[]) {
  return new Set(selections.flatMap((selection) => selection.contentTypeCodes.map((contentTypeCode) => `${selection.learningSpaceId}:${contentTypeCode}`)));
}

function ExpiryReminder({ endTime }: { endTime?: string }) {
  const expiry = packageExpiryReminder(endTime);
  if (!expiry) return null;
  return <Typography.Text className={`student-package-expiry student-package-expiry-${expiry.level}`}>{expiry.text}</Typography.Text>;
}

function packageExpiryReminder(endTime?: string) {
  if (!endTime) return null;
  const normalized = endTime.includes('T') ? endTime : endTime.replace(' ', 'T');
  const parsed = new Date(normalized.length === 10 ? `${normalized}T23:59:59` : normalized);
  if (Number.isNaN(parsed.getTime())) return null;

  const remainingDays = Math.ceil((parsed.getTime() - Date.now()) / 86_400_000);
  const level = remainingDays <= 7 ? 'urgent' : remainingDays <= 14 ? 'warning' : 'normal';
  return { level, text: `结束时间：${endTime.slice(0, 10)}` };
}

function PackageLinks({ values, onOpen }: { values?: StudentPackageRef[]; onOpen: () => void }) {
  if (!values?.length) return <Typography.Text type="secondary">暂未开通课程</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {values.flatMap((item) => [
        ...(item.openCourses?.map((title) => ({ key: `course-${item.packageId}-${title}`, label: `课程：${title}`, color: 'blue' })) ?? []),
        ...(item.openMaterials?.map((title) => ({ key: `material-${item.packageId}-${title}`, label: `讲义：${title}`, color: 'green' })) ?? []),
        ...(item.openHomework?.map((title) => ({ key: `homework-${item.packageId}-${title}`, label: `题目：${title}`, color: 'purple' })) ?? []),
        ...(item.openCourses?.length || item.openMaterials?.length || item.openHomework?.length ? [] : [{ key: `package-${item.packageId}`, label: item.packageName, color: 'blue' }])
      ]).map((item) => (
        <Typography.Link key={item.key} onClick={onOpen}><Tag color={item.color}>{item.label}</Tag></Typography.Link>
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
