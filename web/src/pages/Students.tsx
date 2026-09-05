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
  Popconfirm,
  Popover,
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
import { BellOutlined, CheckCircleOutlined, DeleteOutlined, EditOutlined, EyeOutlined, ImportOutlined, LockOutlined, PlusOutlined, StopOutlined, UnlockOutlined, UploadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { KeyboardEvent } from 'react';
import { deleteData, getData, postData, postForm, putData } from '../services/http';
import { FormDrawer } from '../components/FormDrawer';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../components/ListViews';
import { gradeOptions as curriculumGradeOptions, subjectOptions } from '../utils/curriculum';
import type {
  CurrentUser,
  DirectGrantReplaceRequest,
  DirectGrantResult,
  DirectGrantSelection,
  GrantRevokeResult,
  LearningSpace,
  Student,
  StudentDetail,
  StudentImportResult,
  StudentOpeningCell,
  StudentOpeningScope,
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

const studentTextCollator = new Intl.Collator('zh-CN', { numeric: true, sensitivity: 'base' });
const gradeOrder = ['学前班', '一年级', '二年级', '三年级', '四年级', '五年级', '六年级', '初一', '初二', '初三', '高一', '高二', '高三'];

function compareStudentText(left?: string, right?: string) {
  return studentTextCollator.compare(left ?? '', right ?? '');
}

function compareStudentGrades(left: string, right: string) {
  const leftOrder = gradeOrder.indexOf(left);
  const rightOrder = gradeOrder.indexOf(right);
  if (leftOrder >= 0 && rightOrder >= 0) return leftOrder - rightOrder;
  return compareStudentText(left, right);
}

function canWrite(user: CurrentUser) {
  return user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));
}

function canManageScores(user: CurrentUser) {
  return user.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role));
}

function bindStatusText(status: string) {
  return status === '已绑定' ? '已关联微信' : '待关联微信';
}

function packageStatusLabel(student: Student) {
  if (student.followUpStatus === '待跟进') return '待跟进';
  return (student.openedPackages?.length ?? 0) > 0 ? '已开通' : '未开通';
}

function packageStatusTag(student: Student) {
  const status = packageStatusLabel(student);
  if (status === '待跟进') return <Tag color="red">{status}</Tag>;
  if (status === '已开通') return <Tag color="green">{status}</Tag>;
  return <Tag>{status}</Tag>;
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
  const [keywordInput, setKeywordInput] = useState('');
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
  const [selectedStudentIDs, setSelectedStudentIDs] = useState<string[]>([]);
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

  const toggleStudentStatus = useMutation({
    mutationFn: (student: Student) => putData<Student>(`/students/${student.id}`, {
      name: student.name,
      phone: student.phone,
      grade: student.grade,
      schoolName: student.schoolName ?? '',
      guardianName: student.guardianName ?? '',
      remark: student.remark ?? '',
      accountStatus: student.accountStatus === '停用' ? '正常' : '停用'
    }),
    onSuccess: (_, student) => {
      message.success(student.accountStatus === '停用' ? '学生账号已恢复' : '学生账号已停用');
      queryClient.invalidateQueries({ queryKey: ['students'] });
      if (selected?.id === student.id) queryClient.invalidateQueries({ queryKey: ['students', student.id, 'detail'] });
    },
    onError: (err: Error) => message.error(err.message || '状态更新失败，请稍后重试。')
  });

  const cleanupTestStudents = useMutation({
    mutationFn: () => postData<{ deletedCount: number }>('/students/cleanup-test-data', {}),
    onSuccess: (result) => {
      message.success(result.deletedCount ? `已清理 ${result.deletedCount} 条测试学生数据` : '没有找到可清理的测试学生数据');
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
    onError: (err: Error) => message.error(err.message || '清理失败，请稍后重试。')
  });

  const batchDeleteStudents = useMutation({
    mutationFn: (studentIDs: string[]) => postData<{ deletedCount: number }>('/students/batch-delete', { studentIds: studentIDs }),
    onSuccess: (result) => {
      message.success(`已删除 ${result.deletedCount} 名学生`);
      setSelectedStudentIDs([]);
      if (selected && selectedStudentIDs.includes(selected.id)) setSelected(null);
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
    onError: (err: Error) => message.error(err.message || '删除失败，请稍后重试。')
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

  const revokePackageGrant = useMutation({
    mutationFn: ({ studentId, packageId }: { studentId: string; packageId: string }) => deleteData<GrantRevokeResult>(`/grants/${encodeURIComponent(packageId)}?studentId=${encodeURIComponent(studentId)}`),
    onSuccess: (result) => {
      message.success(`已撤销 ${selected?.name ?? '该学生'} 的套餐“${result.packageName}”。`);
      queryClient.invalidateQueries({ queryKey: ['students'] });
      queryClient.invalidateQueries({ queryKey: ['students', selected?.id, 'detail'] });
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
    },
    onError: (err: Error) => message.error(err.message || '撤销套餐开通失败，请稍后重试。')
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
  const hasActiveFilters = Object.values(filters).some(Boolean);

  function applyQuickFilter(nextFilters: StudentFilters) {
    setKeywordInput('');
    setFilters(nextFilters);
  }

  function activateQuickFilter(event: KeyboardEvent<HTMLElement>, onActivate: () => void) {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    onActivate();
  }

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

  function studentActions(record: Student) {
    const disabled = record.accountStatus === '停用';
    return (
      <Space size={4}>
        <ActionButton tooltip="查看" icon={<EyeOutlined />} onClick={() => openStudentDetail(record)} />
        {writable && record.accountStatus === '正常' && <ActionButton tooltip="课程开通" icon={<UnlockOutlined />} onClick={() => openDirectGrant(record)} />}
        {writable && <ActionButton tooltip="提醒" icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)} />}
        {writable && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />}
        {writable && <ActionButton tooltip={disabled ? '恢复' : '停用'} icon={disabled ? <CheckCircleOutlined /> : <StopOutlined />} loading={toggleStudentStatus.isPending} onClick={() => Modal.confirm({ title: `${disabled ? '恢复' : '停用'}学生账号`, content: `确定${disabled ? '恢复' : '停用'}“${record.name}”的账号吗？`, okText: '确定', cancelText: '取消', okButtonProps: disabled ? undefined : { danger: true }, onOk: () => toggleStudentStatus.mutateAsync(record) })} />}
      </Space>
    );
  }

  const columns: TableColumnsType<Student> = [
    { title: '年级', dataIndex: 'grade', width: 88, sorter: (left, right) => compareStudentGrades(left.grade, right.grade) },
    { title: '学校', dataIndex: 'schoolName', width: 130, ellipsis: true, sorter: (left, right) => compareStudentText(left.schoolName, right.schoolName), render: (value) => value || '-' },
    {
      title: '学生',
      dataIndex: 'name',
      width: writable ? 184 : 170,
      sorter: (left, right) => compareStudentText(left.name, right.name),
      render: (value, record) => <Space direction="vertical" size={0}><Badge dot={record.followUpStatus === '待跟进'} color="#ff4d4f"><Typography.Text strong className="student-name">{value}</Typography.Text></Badge><Typography.Text type="secondary">{record.phone}</Typography.Text>{studentActions(record)}</Space>
    },
    { title: '家长姓名', dataIndex: 'guardianName', width: 110, ellipsis: true, sorter: (left, right) => compareStudentText(left.guardianName, right.guardianName), render: (value) => value || '-' },
    {
      title: '套餐状态',
      width: 144,
      sorter: (left, right) => compareStudentText(packageStatusLabel(left), packageStatusLabel(right)),
      render: (_, record) => packageStatusInfo(record)
    },
    {
      title: '课程',
      dataIndex: 'openedPackageRefs',
      width: 220,
      sorter: (left, right) => compareStudentText(publicPackageRefs(left.openedPackageRefs).map((item) => item.packageName).join('、'), publicPackageRefs(right.openedPackageRefs).map((item) => item.packageName).join('、')),
      render: (values: StudentPackageRef[], record) => <PackageLinks values={values} onOpen={() => openStudentDetail(record, 'courses')} />
    },
    {
      title: '辅导老师',
      dataIndex: 'activeTutoringAssignments',
      width: 190,
      sorter: (left, right) => compareStudentText(left.activeTutoringAssignments?.map((item) => item.teacherName).join('、'), right.activeTutoringAssignments?.map((item) => item.teacherName).join('、')),
      render: (values: TutoringAssignmentSummary[]) => <TutoringTeacherNames assignments={values} />
    }
  ];

  const studentRowSelection = writable ? {
    selectedRowKeys: selectedStudentIDs,
    onChange: (keys: React.Key[]) => setSelectedStudentIDs(keys.map(String))
  } : undefined;

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
    setStudentDrawerTab('courses');
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

  function revokePackageOpening(packageId: string, packageName: string) {
    if (!selected) return;
    Modal.confirm({
      title: '确认撤销套餐开通？',
      content: `将撤销 ${selected.name} 的“${packageName}”套餐权限。该套餐覆盖的其他课程内容也会一并收回，但不会影响套餐设置或其他学生。`,
      okText: '确认撤销',
      okButtonProps: { danger: true },
      cancelText: '暂不撤销',
      onOk: () => revokePackageGrant.mutateAsync({ studentId: selected.id, packageId })
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
            <Button danger icon={<DeleteOutlined />} loading={cleanupTestStudents.isPending} onClick={() => Modal.confirm({ title: '清理测试学生数据', content: '将删除标记为测试的学生及其关联数据，操作不可恢复。确定继续吗？', okText: '确认清理', cancelText: '取消', okButtonProps: { danger: true }, onOk: () => cleanupTestStudents.mutateAsync() })}>清理测试数据</Button>
            <Button icon={<ImportOutlined />} onClick={() => setImportOpen(true)}>批量导入</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增学生</Button>
          </Space>
        )}
      </div>

      <div className="student-stat-grid">
        <Card
          hoverable
          className={!hasActiveFilters ? 'student-stat-card-active' : undefined}
          role="button"
          tabIndex={0}
          aria-label="查看全部学生"
          aria-pressed={!hasActiveFilters}
          onClick={() => applyQuickFilter({})}
          onKeyDown={(event) => activateQuickFilter(event, () => applyQuickFilter({}))}
        >
          <Statistic title="学生总数（点击查看全部）" value={stats.total} />
        </Card>
        <Card
          hoverable
          className={filters.packageState === '已开通' ? 'student-stat-card-active' : undefined}
          role="button"
          tabIndex={0}
          aria-label="筛选已开通课程的学生"
          aria-pressed={filters.packageState === '已开通'}
          onClick={() => applyQuickFilter(filters.packageState === '已开通' ? {} : { packageState: '已开通' })}
          onKeyDown={(event) => activateQuickFilter(event, () => applyQuickFilter(filters.packageState === '已开通' ? {} : { packageState: '已开通' }))}
        >
          <Statistic title="已开通课程（点击筛选）" value={stats.opened} />
        </Card>
        <Card
          hoverable
          className={filters.followUpState === '待跟进' ? 'student-stat-card-active' : undefined}
          role="button"
          tabIndex={0}
          aria-label="筛选待跟进学生"
          aria-pressed={filters.followUpState === '待跟进'}
          onClick={() => applyQuickFilter(filters.followUpState === '待跟进' ? {} : { followUpState: '待跟进' })}
          onKeyDown={(event) => activateQuickFilter(event, () => applyQuickFilter(filters.followUpState === '待跟进' ? {} : { followUpState: '待跟进' }))}
        >
          <Statistic title="待跟进（点击筛选）" value={stats.waiting} valueStyle={{ color: stats.waiting ? '#cf1322' : undefined }} />
        </Card>
        <Card><Statistic title="已停用" value={stats.disabled} /></Card>
      </div>

      <Card>
        <div className="list-panel">
          <div className="list-toolbar">
            <Space wrap>
              <Input.Search
                value={keywordInput}
                placeholder="搜索姓名或手机号"
                allowClear
                onChange={(event) => {
                  const keyword = event.target.value;
                  setKeywordInput(keyword);
                  if (!keyword) setFilters((prev) => ({ ...prev, keyword: undefined }));
                }}
                onSearch={(keyword) => setFilters((prev) => ({ ...prev, keyword }))}
                style={{ width: 240 }}
              />
              <Select allowClear value={filters.grade} placeholder="年级" options={gradeOptions} style={{ width: 140 }} onChange={(grade) => setFilters((prev) => ({ ...prev, grade }))} />
              <Select allowClear value={filters.accountStatus} placeholder="账号状态" options={accountOptions} style={{ width: 140 }} onChange={(accountStatus) => setFilters((prev) => ({ ...prev, accountStatus }))} />
              <Select allowClear value={filters.learningStatus} placeholder="学习状态" options={learningOptions} style={{ width: 150 }} onChange={(learningStatus) => setFilters((prev) => ({ ...prev, learningStatus }))} />
              <Select
                allowClear
                value={filters.packageState}
                placeholder="套餐开通状态"
                options={[{ label: '已开通', value: '已开通' }, { label: '未开通', value: '未开通' }]}
                style={{ width: 140 }}
                onChange={(packageState) => setFilters((prev) => ({ ...prev, packageState }))}
              />
              <Select allowClear value={filters.followUpState} placeholder="跟进状态" options={[{ label: '待跟进', value: '待跟进' }]} style={{ width: 140 }} onChange={(followUpState) => setFilters((prev) => ({ ...prev, followUpState }))} />
              {writable && selectedStudentIDs.length > 0 && <Popconfirm title={`确定删除选中的 ${selectedStudentIDs.length} 名学生吗？`} description="学生账号及其学习、授权和关联数据将一并删除，操作不可恢复。" okText="确认删除" cancelText="取消" okButtonProps={{ danger: true, loading: batchDeleteStudents.isPending }} onConfirm={() => batchDeleteStudents.mutate(selectedStudentIDs)}><Button danger icon={<DeleteOutlined />}>批量删除（{selectedStudentIDs.length}）</Button></Popconfirm>}
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
                      {writable && record.accountStatus === '正常' && <ActionButton tooltip="课程开通" icon={<UnlockOutlined />} onClick={() => openDirectGrant(record)} />}
                      {writable && <ActionButton tooltip="提醒" icon={<BellOutlined />} loading={remindStudent.isPending} onClick={() => remindStudent.mutate(record)} />}
                      {writable && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />}
                      {writable && <ActionButton tooltip={record.accountStatus === '停用' ? '恢复' : '停用'} icon={record.accountStatus === '停用' ? <CheckCircleOutlined /> : <StopOutlined />} loading={toggleStudentStatus.isPending} onClick={() => Modal.confirm({ title: `${record.accountStatus === '停用' ? '恢复' : '停用'}学生账号`, content: `确定${record.accountStatus === '停用' ? '恢复' : '停用'}“${record.name}”的账号吗？`, okText: '确定', cancelText: '取消', okButtonProps: record.accountStatus === '停用' ? undefined : { danger: true }, onOk: () => toggleStudentStatus.mutateAsync(record) })} />}
                    </>
                  )}
                />
              )}
            />
          ) : (
            rows.length === 0 ? <Empty description="还没有学生，先新增学生或批量导入。" /> : <div className="student-table-scroll"><Table className="student-table" rowKey="id" rowSelection={studentRowSelection} columns={columns} dataSource={rows} rowClassName={(record) => record.followUpStatus === '待跟进' ? 'student-follow-up-row' : ''} tableLayout="fixed" scroll={{ x: 1160 }} pagination={{ pageSize: 8 }} sortDirections={['ascend', 'descend']} /></div>
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

      <Drawer
        className="student-detail-drawer"
        title={selected ? <StudentDrawerTitle student={selected} /> : '学生详情'}
        width="min(760px, 100vw)"
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
      >
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
                  />
                )
              },
			  {
                key: 'courses',
                label: '课程开通',
                children: selected ? (
                  <CourseOpeningPanel
                    detail={detail.data}
                    student={selected}
                    writable={writable}
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
                    onRevokePackage={revokePackageOpening}
                  />
                ) : null
              },
			  {
				key: 'tutoring',
				label: writable ? '辅导老师' : '我的辅导关系',
				children: selected ? <TutoringAssignmentPanel student={selected} writable={writable} teachers={teachers.data ?? []} subjects={subjects.data ?? []} learningSpaces={learningSpaces.data ?? []} /> : null
			  },
			  { key: 'lesson-feedback', label: '课后反馈', children: selected ? <LessonFeedbackPanel student={selected} user={user} /> : null },
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
  detail
}: {
  detail: StudentDetail;
}) {
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
    </Space>
  );
}

function StudentDrawerTitle({ student }: { student: Student }) {
  return (
    <div className="student-drawer-title">
      <span className="student-drawer-title-name">{student.name}</span>
      <Space size={6} wrap>
        <Tag color="blue">{student.grade}</Tag>
        <Typography.Text type="secondary">{student.phone}</Typography.Text>
        <Tag color={student.accountStatus === '正常' ? 'green' : 'default'}>{student.accountStatus}</Tag>
      </Space>
    </div>
  );
}

function CourseOpeningPanel({
  detail,
  student,
  writable,
  loadingLearningSpaces,
  learningSpacesError,
  selections,
  startsAt,
  endsAt,
  submitting,
  onSelectionsChange,
  onStartsAtChange,
  onEndsAtChange,
  onSubmit,
  onRevokePackage
}: {
  detail: StudentDetail;
  student: Student;
  writable: boolean;
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
  onRevokePackage: (packageId: string, packageName: string) => void;
}) {
  if (!writable) {
    return <CourseOpeningMatrix matrix={detail.openingMatrix} selections={selections} onSelectionsChange={onSelectionsChange} readOnly />;
  }
  return (
    <DirectGrantPanel
      student={student}
      matrix={detail.openingMatrix}
      loadingLearningSpaces={loadingLearningSpaces}
      learningSpacesError={learningSpacesError}
      selections={selections}
      startsAt={startsAt}
      endsAt={endsAt}
      submitting={submitting}
      onSelectionsChange={onSelectionsChange}
      onStartsAtChange={onStartsAtChange}
      onEndsAtChange={onEndsAtChange}
      onSubmit={onSubmit}
      onRevokePackage={onRevokePackage}
    />
  );
}

const openingContentLabels: Record<StudentOpeningCell['contentTypeCode'], string> = {
  course: '课程',
  handout: '讲义',
  question: '习题',
  download: '下载讲义'
};

function CourseOpeningMatrix({
  matrix,
  selections,
  onSelectionsChange,
  onRevokePackage,
  readOnly = false
}: {
  matrix: StudentOpeningScope[];
  selections: DirectGrantSelection[];
  onSelectionsChange: (values: DirectGrantSelection[]) => void;
  onRevokePackage?: (packageId: string, packageName: string) => void;
  readOnly?: boolean;
}) {
  const selectionFor = (learningSpaceId: string) => selections.find((selection) => selection.learningSpaceId === learningSpaceId)?.contentTypeCodes ?? [];
  const changeSelection = (learningSpaceId: string, contentTypeCode: StudentOpeningCell['contentTypeCode'], checked: boolean) => {
    const current = selectionFor(learningSpaceId);
    const nextValues = checked
      ? [...new Set(contentTypeCode === 'course' ? [...current, 'course', 'handout', 'question', 'download'] : [...current, contentTypeCode])]
      : current.filter((item) => item !== contentTypeCode && !(contentTypeCode === 'course' && ['handout', 'question', 'download'].includes(item)));
    const next = selections.filter((selection) => selection.learningSpaceId !== learningSpaceId);
    if (nextValues.length > 0) next.push({ learningSpaceId, contentTypeCodes: nextValues });
    onSelectionsChange(next);
  };

  return (
    <Space direction="vertical" size={10} style={{ width: '100%' }}>
      {matrix.map((scope) => (
        <Card size="small" key={scope.learningSpaceId} title={scope.name}>
          <Space wrap size={[20, 8]} aria-label={`${scope.name}课程开通`}>
            {scope.content.filter((cell) => cell.contentTypeCode !== 'download').map((cell) => {
              const directSelected = selectionFor(scope.learningSpaceId).includes(cell.contentTypeCode);
              const checked = cell.packageOpened ? cell.opened : directSelected;
              const locked = cell.packageOpened;
              const label = openingContentLabels[cell.contentTypeCode];
              const previewItems = cell.items.slice(0, 5);
              const sourceText = cell.packageOpened
                ? `由 ${cell.packageNames.join('、')} 开通`
                : directSelected ? '单独开通' : '尚未开通';
              return (
                <Popover
                  key={cell.contentTypeCode}
                  trigger={['hover', 'click', 'focus']}
                  title={`${label}明细`}
                  content={(
                    <Space direction="vertical" size={6} style={{ maxWidth: 300 }}>
                      <Typography.Text type="secondary">{sourceText}</Typography.Text>
                      <Typography.Text strong>{checked ? `已开通内容（${cell.items.length}）` : `可开通内容（${cell.items.length}）`}</Typography.Text>
                      {previewItems.length > 0 ? previewItems.map((item) => <Typography.Text key={item.id}>· {item.title}</Typography.Text>) : <Typography.Text type="secondary">暂未配置具体内容</Typography.Text>}
                      {cell.items.length > previewItems.length && <Typography.Text type="secondary">还有 {cell.items.length - previewItems.length} 项内容</Typography.Text>}
                      {!readOnly && locked && directSelected && (
                        <Button type="link" size="small" danger onClick={() => changeSelection(scope.learningSpaceId, cell.contentTypeCode, false)}>撤销单独开通</Button>
                      )}
                      {!readOnly && (cell.packageGrants ?? []).map((packageGrant) => (
                        <Button key={packageGrant.packageId} type="link" size="small" danger onClick={() => onRevokePackage?.(packageGrant.packageId, packageGrant.packageName)}>
                          撤销套餐“{packageGrant.packageName}”
                        </Button>
                      ))}
                    </Space>
                  )}
                >
                  <span aria-label={`${scope.name}${label}明细`} tabIndex={0}>
                    <Checkbox
                      checked={checked}
                      disabled={readOnly || locked}
                      onChange={(event) => changeSelection(scope.learningSpaceId, cell.contentTypeCode, event.target.checked)}
                    >
                      {label}
                    </Checkbox>
                    {locked && <Tooltip title={sourceText}><LockOutlined aria-label={`${label}由课程方案开通`} style={{ marginLeft: 4, color: '#8c8c8c' }} /></Tooltip>}
                  </span>
                </Popover>
              );
            })}
          </Space>
        </Card>
      ))}
    </Space>
  );
}

function DirectGrantPanel({
  student,
  matrix,
  loadingLearningSpaces,
  learningSpacesError,
  selections,
  startsAt,
  endsAt,
  submitting,
  onSelectionsChange,
  onStartsAtChange,
  onEndsAtChange,
  onSubmit,
  onRevokePackage
}: {
  student: Student;
  matrix: StudentOpeningScope[];
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
  onRevokePackage: (packageId: string, packageName: string) => void;
}) {
  const [selectedSubject, setSelectedSubject] = useState<string>();
  const [selectedOnly, setSelectedOnly] = useState(false);
  const subjectFilters = useMemo(() => {
    const counts = new Map<string, number>();
    matrix.forEach((space) => counts.set(space.subject, (counts.get(space.subject) ?? 0) + 1));
    return Array.from(counts, ([subject, count]) => ({ subject, count }));
  }, [matrix]);
  const openedSpaceIds = useMemo(() => new Set(matrix.filter((space) => space.content.some((cell) => cell.opened || selections.find((selection) => selection.learningSpaceId === space.learningSpaceId)?.contentTypeCodes.includes(cell.contentTypeCode))).map((space) => space.learningSpaceId)), [matrix, selections]);
  const visibleMatrix = useMemo(() => {
    const filtered = matrix.filter((space) => (!selectedSubject || space.subject === selectedSubject) && (!selectedOnly || openedSpaceIds.has(space.learningSpaceId)));
    return [...filtered].sort((a, b) => Number(openedSpaceIds.has(b.learningSpaceId)) - Number(openedSpaceIds.has(a.learningSpaceId)));
  }, [matrix, selectedSubject, selectedOnly, openedSpaceIds]);
  const selectedContentCount = matrix.reduce((count, scope) => count + scope.content.filter((cell) => {
    const directSelected = selections.find((selection) => selection.learningSpaceId === scope.learningSpaceId)?.contentTypeCodes.includes(cell.contentTypeCode);
    return cell.opened || directSelected;
  }).length, 0);
  return (
    <div className="student-opening-panel">
      <Alert
        type="info"
        showIcon
        message="选择要开通的内容"
        description="套餐已开通的内容会自动勾选；可点击名称查看来源，误开套餐可撤销该学生的套餐权限。单独开通内容可直接勾选或取消。"
      />
      <div>
        <Typography.Text strong>课程范围</Typography.Text>
        <Typography.Paragraph type="secondary" style={{ margin: '4px 0 10px' }}>
          勾选课程、讲义或习题即可开通；经过或点击名称可查看具体内容。
        </Typography.Paragraph>
        <Space direction="vertical" size={8} style={{ width: '100%', marginBottom: 12 }}>
          <Space wrap size={[8, 8]}>
            <Tag color="blue">当前年级：{student.grade}</Tag>
            <Button type="link" size="small" aria-label="筛选已开通课程" onClick={() => { setSelectedSubject(undefined); setSelectedOnly(true); }}>已选 {selectedContentCount} 项内容</Button>
          </Space>
          <div role="group" aria-label="科目筛选">
            <Space wrap size={[8, 8]}>
              <Typography.Text>科目</Typography.Text>
              <Button size="small" type={!selectedSubject ? 'primary' : 'default'} onClick={() => setSelectedSubject(undefined)}>
                全部（{matrix.length}）
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
              <Button size="small" type={selectedOnly ? 'primary' : 'default'} onClick={() => setSelectedOnly((value) => !value)}>
                仅看已开通（{openedSpaceIds.size}）
              </Button>
            </Space>
          </div>
          {openedSpaceIds.size > 0 && (
            <Space wrap size={[6, 6]} aria-label="已选项目快捷筛选">
              <Typography.Text type="secondary">已选项目：</Typography.Text>
              {matrix.filter((space) => openedSpaceIds.has(space.learningSpaceId)).map((space) => (
                <Button key={space.learningSpaceId} size="small" type={selectedSubject === space.subject && selectedOnly ? 'primary' : 'default'} onClick={() => { setSelectedSubject(space.subject); setSelectedOnly(true); }}>
                  {space.name}
                </Button>
              ))}
            </Space>
          )}
        </Space>
        {loadingLearningSpaces ? <Skeleton active paragraph={{ rows: 3 }} /> : learningSpacesError ? (
          <Alert type="error" showIcon message="课程范围加载失败，请关闭抽屉后重试。" />
        ) : matrix.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该年级还没有可开通的课程范围。" />
        ) : visibleMatrix.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该科目暂无可开通课程范围。" />
        ) : (
          <Space direction="vertical" size={10} style={{ width: '100%' }}>
            <CourseOpeningMatrix matrix={visibleMatrix} selections={selections} onSelectionsChange={onSelectionsChange} onRevokePackage={onRevokePackage} />
          </Space>
        )}
      </div>
      <section className="student-opening-period">
        <Typography.Text strong>生效时间</Typography.Text>
        <Typography.Paragraph type="secondary" style={{ margin: '4px 0 10px' }}>
          有效期按当前校历的期中、期末节点自动计算，开通后立即生效，无需手动填写日期。
        </Typography.Paragraph>
        <Alert type="success" showIcon message="系统将自动使用当前校历节点" description="如需调整校历，请前往系统设置中的校历维护。" />
      </section>
      <div className="student-opening-actions">
        <div className="student-opening-actions-summary">
          <Typography.Text strong aria-live="polite">{selectedContentCount} 项内容将生效</Typography.Text>
          <Typography.Text type="secondary">确认后立即更新学生权限</Typography.Text>
        </div>
        <Button
          type="primary"
          icon={<UnlockOutlined />}
          loading={submitting}
          disabled={loadingLearningSpaces || learningSpacesError}
          onClick={onSubmit}
        >
          保存变更
        </Button>
      </div>
    </div>
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
  if (value === '下载讲义') return 'download';
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
  const visibleValues = publicPackageRefs(values);
  if (visibleValues.length === 0) {
    if (values?.length) return <Typography.Link onClick={onOpen}>已开通课程</Typography.Link>;
    return <Typography.Text type="secondary">暂未开通课程</Typography.Text>;
  }
  return (
    <Space size={[4, 4]} wrap>
      {visibleValues.map((item) => (
        <Typography.Link key={item.packageId} onClick={onOpen}><Tag color="blue">{item.packageName}</Tag></Typography.Link>
      ))}
    </Space>
  );
}

function publicPackageRefs(values?: StudentPackageRef[]) {
  return (values ?? []).filter((item) => !item.packageId.startsWith('direct-'));
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
  const rows = [
    ...detail.notices.map((record) => ({ kind: 'notice' as const, record })),
    ...detail.logs.map((record) => ({ kind: 'log' as const, record }))
  ];

  return (
    <CardList
      rows={rows}
      rowKey={(item) => `${item.kind}-${item.record.id}`}
      emptyText="还没有操作记录。"
      renderCard={(item) => item.kind === 'notice' ? (
        <InfoCard
          title={item.record.title}
          subtitle={item.record.summary}
          status={<Tag>{item.record.status}</Tag>}
        />
      ) : (
        <InfoCard
          title={item.record.action}
          subtitle={item.record.target}
          fields={[
            { label: '操作人', value: item.record.operator },
            { label: '时间', value: item.record.time }
          ]}
        />
      )}
    />
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
