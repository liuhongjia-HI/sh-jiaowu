import { CalendarOutlined, CloseCircleOutlined, DeleteOutlined, EditOutlined, LeftOutlined, PlusOutlined, ReloadOutlined, RightOutlined, SaveOutlined, SettingOutlined, TableOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Drawer, Empty, Form, Input, InputNumber, Modal, Popconfirm, Segmented, Select, Skeleton, Space, Table, Tag, Typography, message } from 'antd';
import type { TableColumnsType } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { getData, postData, putData } from '../services/http';
import { ActionButton } from '../components/ListViews';
import { gradeOptions, subjectOptions } from '../utils/curriculum';
import type { AvailabilitySlot, Course, CurrentUser, ScheduleCandidate, ScheduleClass, Student, Teacher } from '../types/starline';

type CandidateFormValues = {
  subject: string;
  grade: string;
  classType: string;
  durationMinutes: number;
  startDate: string;
  endDate: string;
};

type AvailabilityFormValues = {
  ownerKey: string;
  slots: AvailabilitySlot[];
};

type ScheduleClassFormValues = {
  courseId: string;
  teacherId: string;
  campusId: string;
  roomName: string;
  classType: string;
  durationMinutes: number;
  dayOfWeek: number;
  startTime: string;
  endTime: string;
  startDate: string;
  endDate: string;
  studentIds: string[];
};

type ScheduleFilters = {
  grade?: string;
  subject?: string;
  teacherId?: string;
  studentId?: string;
  campusId?: string;
  courseId?: string;
  classType?: string;
  status?: string;
};

type ScheduleMoveTarget = {
  dayOfWeek: number;
  label: string;
  startTime?: string;
  endTime?: string;
  startDate?: string;
  endDate?: string;
};

type CandidateLevel = 'full' | 'ready' | 'short';
type CourseLookup = Record<string, Course>;
type TimelineKind = 'class' | 'candidate' | 'availability';
type TimelineItem = {
  id: string;
  kind: TimelineKind;
  dayOfWeek: number;
  startTime: string;
  endTime: string;
  subject: string;
  title: string;
  subtitle: string;
  meta: string;
  status?: string;
  classType?: string;
  countText?: string;
  record: ScheduleClass | ScheduleCandidate | AvailabilitySlot;
};
type TimelineLayoutItem = TimelineItem & {
  column: number;
  columns: number;
  leftPct?: number;
  widthPct?: number;
};
type WeekDay = {
  key: string;
  date: Date;
  day: number;
  dayOfWeek: number;
  weekLabel: string;
  label: string;
};

const weekOptions = [
  { label: '周一', value: 1 },
  { label: '周二', value: 2 },
  { label: '周三', value: 3 },
  { label: '周四', value: 4 },
  { label: '周五', value: 5 },
  { label: '周六', value: 6 },
  { label: '周日', value: 7 }
];

const classTypeOptions = ['1V1', '1V2', '1V3', '1V4'].map((value) => ({ label: value, value }));
const timelineSlotMinutes = 30;
const timelineSlotHeight = 44;
const defaultTimelineStart = 14 * 60;
const defaultTimelineEnd = 22 * 60;

export default function Scheduling({ user }: { user: CurrentUser }) {
  const [availabilityForm] = Form.useForm<AvailabilityFormValues>();
  const [candidateForm] = Form.useForm<CandidateFormValues>();
  const [editForm] = Form.useForm<ScheduleClassFormValues>();
  const [candidateRequest, setCandidateRequest] = useState<CandidateFormValues | null>(null);
  const [selectedCandidate, setSelectedCandidate] = useState<ScheduleCandidate | null>(null);
  const [selectedStudentIds, setSelectedStudentIds] = useState<string[]>([]);
  const [selectedCampusId, setSelectedCampusId] = useState(user.campusId || 'campus-main');
  const [editingClass, setEditingClass] = useState<ScheduleClass | null>(null);
  const [creatingClass, setCreatingClass] = useState(false);
  const [availabilityOpen, setAvailabilityOpen] = useState(false);
  const [moreSettingsOpen, setMoreSettingsOpen] = useState(false);
  const [viewMode, setViewMode] = useState<'week' | 'month' | 'list'>('week');
  const [classGradeFilter, setClassGradeFilter] = useState<string>();
  const [classSubjectFilter, setClassSubjectFilter] = useState<string>();
  const [classTeacherFilter, setClassTeacherFilter] = useState<string>();
  const [classStudentFilter, setClassStudentFilter] = useState<string>();
  const [classCampusFilter, setClassCampusFilter] = useState<string>();
  const [classCourseFilter, setClassCourseFilter] = useState<string>();
  const [classTypeFilter, setClassTypeFilter] = useState<string>();
  const [statusFilter, setStatusFilter] = useState<string>('已确认');
  const [selectedWeekStart, setSelectedWeekStart] = useState<Date>(() => startOfWeek(new Date()));
  const [calendarMonth, setCalendarMonth] = useState<Date>(() => startOfMonth(new Date()));
  const [hiddenSubjects, setHiddenSubjects] = useState<string[]>([]);
  const queryClient = useQueryClient();
  const canCreateClass = user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));

  const teachers = useQuery({ queryKey: ['teachers'], queryFn: () => getData<Teacher[]>('/teachers') });
  const students = useQuery({ queryKey: ['students'], queryFn: () => getData<Student[]>('/students') });
  const courses = useQuery({ queryKey: ['courses'], queryFn: () => getData<Course[]>('/courses') });
  const classes = useQuery({ queryKey: ['schedule-classes'], queryFn: () => getData<ScheduleClass[]>('/schedule-classes') });
  const availabilityOverview = useQuery({ queryKey: ['availability-overview'], queryFn: () => getData<AvailabilitySlot[]>('/availability/overview') });

  const ownerKey = Form.useWatch('ownerKey', availabilityForm);
  const gradeWatch = Form.useWatch('grade', candidateForm);
  const editingClassType = Form.useWatch('classType', editForm);
  const editingStudentIDs = Form.useWatch('studentIds', editForm) ?? [];
  const owner = parseOwnerKey(ownerKey);
  const availability = useQuery({
    queryKey: ['availability', owner?.ownerType, owner?.ownerId],
    enabled: Boolean(owner),
    queryFn: () => getData<AvailabilitySlot[]>('/availability', { ownerType: owner?.ownerType ?? '', ownerId: owner?.ownerId ?? '' })
  });

  const candidates = useQuery({
    queryKey: ['schedule-candidates', candidateRequest],
    enabled: Boolean(candidateRequest),
    queryFn: () => postData<ScheduleCandidate[]>('/scheduling/candidates', candidateRequest)
  });

  const saveAvailability = useMutation({
    mutationFn: (values: AvailabilityFormValues) => {
      const parsed = parseOwnerKey(values.ownerKey);
      if (!parsed) throw new Error('请选择老师或学生');
      return putData<AvailabilitySlot[]>('/availability', {
        ownerType: parsed.ownerType,
        ownerId: parsed.ownerId,
        slots: values.slots ?? []
      });
    },
    onSuccess: () => {
      message.success('可上课时间已保存');
      queryClient.invalidateQueries({ queryKey: ['availability'] });
      queryClient.invalidateQueries({ queryKey: ['availability-overview'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: () => message.error('保存失败，请检查星期和时间段。')
  });

  const createClass = useMutation({
    mutationFn: () => {
      if (!selectedCandidate || !candidateRequest) throw new Error('请选择候选方案');
      return postData<ScheduleClass>('/schedule-classes', {
        courseId: selectedCandidate.courseId,
        teacherId: selectedCandidate.teacherId,
        campusId: selectedCampusId,
        roomName: '',
        classType: selectedCandidate.classType,
        durationMinutes: candidateRequest.durationMinutes,
        dayOfWeek: selectedCandidate.dayOfWeek,
        startTime: selectedCandidate.startTime,
        endTime: selectedCandidate.endTime,
        startDate: candidateRequest.startDate,
        endDate: candidateRequest.endDate,
        studentIds: selectedStudentIds
      });
    },
    onSuccess: () => {
      message.success('已确认成班，课表已生成');
      setSelectedCandidate(null);
      setSelectedStudentIds([]);
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: () => message.error('确认排课失败，请检查人数和时间冲突。')
  });

  const cancelClass = useMutation({
    mutationFn: (id: string) => postData<ScheduleClass>(`/schedule-classes/${id}/cancel`, {}),
    onSuccess: () => {
      message.success('课程已取消，可重新生成候选排课');
      setEditingClass(null);
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: () => message.error('取消课程失败，请稍后重试。')
  });

  const createManualClass = useMutation({
    mutationFn: (values: ScheduleClassFormValues) => postData<ScheduleClass>('/schedule-classes', values),
    onSuccess: () => {
      message.success('课程已创建，课表已更新');
      setCreatingClass(false);
      editForm.resetFields();
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: () => message.error('创建课程失败，请检查学生、老师和时间冲突。')
  });

  const updateClass = useMutation({
    mutationFn: (values: ScheduleClassFormValues) => {
      if (!editingClass) throw new Error('请选择要调整的课程');
      return putData<ScheduleClass>(`/schedule-classes/${editingClass.id}`, values);
    },
    onSuccess: () => {
      message.success('调课已保存');
      setEditingClass(null);
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: () => message.error('调课失败，请检查可上课时间和冲突。')
  });

  const moveClass = useMutation({
    mutationFn: ({ record, target }: { record: ScheduleClass; target: ScheduleMoveTarget }) =>
      putData<ScheduleClass>(`/schedule-classes/${record.id}`, scheduleClassPayload(record, target)),
    onSuccess: () => {
      message.success('调课已保存');
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: () => message.error('调课失败，请检查老师、学生时间冲突。')
  });

  const ownerOptions = useMemo(() => {
    const teacherOptions = (teachers.data ?? []).map((item) => ({ label: `老师 · ${teacherOptionLabel(item)}`, value: `teacher:${item.id}` }));
    const studentOptions = (students.data ?? []).map((item) => ({ label: `学生 · ${studentDisplayName(item)}`, value: `student:${item.id}` }));
    if (user.roles.includes('teacher')) return teacherOptions.filter((item) => item.value === `teacher:${user.userId}`);
    return [...teacherOptions, ...studentOptions];
  }, [teachers.data, students.data, user]);

  const courseOptions = (courses.data ?? []).map((item) => ({ label: `${item.name} · ${item.grade}/${item.subject}`, value: item.id }));
  const teacherOptions = (teachers.data ?? []).map((item) => ({ label: teacherOptionLabel(item), value: item.id }));
  const studentOptions = (students.data ?? []).map((item) => ({ label: studentOptionLabel(item), value: item.id }));
  const campusOptions = uniqueScheduleCampuses(classes.data ?? []).map((value) => ({ label: value, value }));
  const classSubjectOptions = subjectOptions(classGradeFilter);
  const courseById = useMemo(() => Object.fromEntries((courses.data ?? []).map((item) => [item.id, item])), [courses.data]);
  const teacherById = useMemo(() => Object.fromEntries((teachers.data ?? []).map((item) => [item.id, item])), [teachers.data]);
  const studentById = useMemo(() => Object.fromEntries((students.data ?? []).map((item) => [item.id, item])), [students.data]);
  const statusOptions = [
    { label: '已确认', value: '已确认' },
    { label: '已取消', value: '已取消' },
    { label: '全部', value: '全部' }
  ];
  const classFilters = useMemo<ScheduleFilters>(() => ({
    grade: classGradeFilter,
    subject: classSubjectFilter,
    teacherId: classTeacherFilter,
    studentId: classStudentFilter,
    campusId: classCampusFilter,
    courseId: classCourseFilter,
    classType: classTypeFilter,
    status: statusFilter
  }), [classGradeFilter, classSubjectFilter, classTeacherFilter, classStudentFilter, classCampusFilter, classCourseFilter, classTypeFilter, statusFilter]);
  const filteredClasses = useMemo(() => filterClasses(classes.data ?? [], classFilters, courseById), [classes.data, classFilters, courseById]);
  const allCandidates = candidates.data ?? [];
  const readyCandidates = useMemo(() => allCandidates.filter((item) => candidateLevel(item) !== 'short'), [allCandidates]);
  const shortCandidates = useMemo(() => allCandidates.filter((item) => candidateLevel(item) === 'short'), [allCandidates]);
  const scheduleSubjects = useMemo(() => uniqueScheduleSubjects(classes.data ?? [], allCandidates, courseById), [classes.data, allCandidates, courseById]);
  const subjectVisibleClasses = useMemo(
    () => filteredClasses.filter((item) => !hiddenSubjects.includes(scheduleClassSubject(item, courseById) || '其他')),
    [filteredClasses, hiddenSubjects, courseById]
  );
  const selectedWeekDays = useMemo(() => buildWeekDays(selectedWeekStart), [selectedWeekStart]);
  const selectedWeekClasses = useMemo(
    () => subjectVisibleClasses.filter((item) => selectedWeekDays.some((day) => scheduleClassOccursOn(item, day.date))),
    [subjectVisibleClasses, selectedWeekDays]
  );
  const classesByDay = useMemo(() => groupScheduleItems(selectedWeekClasses), [selectedWeekClasses]);
  const candidatesByDay = useMemo(() => groupScheduleItems(readyCandidates), [readyCandidates]);
  const availabilityByDay = useMemo(() => groupScheduleItems(availabilityOverview.data ?? []), [availabilityOverview.data]);
  const availabilitySummary = useMemo(() => availabilityStats(availabilityOverview.data ?? []), [availabilityOverview.data]);
  const activeClassCount = subjectVisibleClasses.filter((item) => item.status !== '已取消').length;
  const totalConfirmedClassCount = (classes.data ?? []).filter((item) => item.status === '已确认').length;
  const hasClassFilters = Boolean(classGradeFilter || classSubjectFilter || classTeacherFilter || classStudentFilter || classCampusFilter || classCourseFilter || classTypeFilter || hiddenSubjects.length > 0 || statusFilter !== '已确认');
  const classResultNote = scheduleResultNote(subjectVisibleClasses, totalConfirmedClassCount, hasClassFilters, classGradeFilter);
  const recommendedCount = readyCandidates.length;
  const emptyTips = candidateEmptyTips(candidateRequest, allCandidates);

  useEffect(() => {
    if (ownerOptions.length > 0 && !availabilityForm.getFieldValue('ownerKey')) {
      availabilityForm.setFieldValue('ownerKey', ownerOptions[0].value);
    }
  }, [availabilityForm, ownerOptions]);

  useEffect(() => {
    if (availability.data) availabilityForm.setFieldValue('slots', availability.data);
  }, [availability.data, availabilityForm]);

  // 切换年级后，若已选学科在该年级不开设，则清空学科，避免排出无效组合。
  useEffect(() => {
    const subject = candidateForm.getFieldValue('subject');
    if (subject && !subjectOptions(gradeWatch).some((item) => item.value === subject)) {
      candidateForm.setFieldValue('subject', undefined);
    }
  }, [candidateForm, gradeWatch]);

  useEffect(() => {
    if (classSubjectFilter && !subjectOptions(classGradeFilter).some((item) => item.value === classSubjectFilter)) {
      setClassSubjectFilter(undefined);
    }
  }, [classGradeFilter, classSubjectFilter]);

  // 打开「维护可上课时间」抽屉，并预选某个师生，便于一键协调缺时间的对象。
  function openAvailabilityFor(ownerType: 'teacher' | 'student', ownerId: string) {
    availabilityForm.setFieldValue('ownerKey', `${ownerType}:${ownerId}`);
    setAvailabilityOpen(true);
    setTimeout(() => availability.refetch());
  }

  function openEdit(record: ScheduleClass) {
    setCreatingClass(false);
    setEditingClass(record);
    editForm.setFieldsValue({
      courseId: record.courseId,
      teacherId: record.teacherId,
      campusId: record.campusId || user.campusId || 'campus-main',
      roomName: '',
      classType: record.classType,
      durationMinutes: record.durationMinutes,
      dayOfWeek: record.dayOfWeek,
      startTime: record.startTime,
      endTime: record.endTime,
      startDate: record.startDate,
      endDate: record.endDate,
      studentIds: record.students.map((student) => student.id)
    });
  }

  function openCreateClassForDay(dayOfWeek: number) {
    if (!canCreateClass) return;
    setEditingClass(null);
    setCreatingClass(true);
    editForm.setFieldsValue({
      courseId: undefined,
      teacherId: undefined,
      campusId: user.campusId || 'campus-main',
      roomName: '',
      classType: '1V1',
      durationMinutes: 90,
      dayOfWeek,
      startTime: '19:00',
      endTime: '20:30',
      startDate: new Date().toISOString().slice(0, 10),
      endDate: '',
      studentIds: []
    });
  }

  function confirmMoveClass(record: ScheduleClass, target: ScheduleMoveTarget) {
    const isSameTarget = record.dayOfWeek === target.dayOfWeek &&
      (!target.startTime || (record.startTime === target.startTime && record.endTime === target.endTime)) &&
      (!target.startDate || (record.startDate === target.startDate && record.endDate === (target.endDate || target.startDate)));
    if (!canCreateClass || record.status === '已取消' || isSameTarget) return;
    Modal.confirm({
      title: '确认调课',
      content: `将「${record.name}」调整到${target.label}。`,
      okText: '确认调整',
      cancelText: '取消',
      onOk: () => moveClass.mutate({ record, target })
    });
  }

  if (teachers.isLoading || students.isLoading || courses.isLoading || classes.isLoading || availabilityOverview.isLoading) return <Skeleton active />;
  if (teachers.error || students.error || courses.error || classes.error || availabilityOverview.error) return <Alert type="error" message="排课数据加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>排课管理</Typography.Title>
          <Typography.Text type="secondary">选学科和年级，系统按「同年级同学科」匹配师生填报的时间，凑出可排方案再确认成班。</Typography.Text>
        </div>
        <Space wrap>
          <Button icon={<SaveOutlined />} onClick={() => setAvailabilityOpen(true)}>维护可上课时间</Button>
          <ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => queryClient.invalidateQueries()} />
        </Space>
      </div>

      <Card title="查找可排时间">
        {canCreateClass ? (
          <Form
            form={candidateForm}
            layout="vertical"
            className="schedule-search-form"
            initialValues={{ classType: '1V4', durationMinutes: 90, startDate: new Date().toISOString().slice(0, 10) }}
            onFinish={(values) => {
              setSelectedCandidate(null);
              setSelectedStudentIds([]);
              setCandidateRequest(values);
            }}
          >
            <div className="schedule-search-main">
              <Form.Item name="grade" label="年级" rules={[{ required: true, message: '请选择年级' }]}>
                <Select showSearch optionFilterProp="label" placeholder="选择年级" options={gradeOptions()} />
              </Form.Item>
              <Form.Item name="subject" label="学科" rules={[{ required: true, message: '请选择学科' }]}>
                <Select showSearch optionFilterProp="label" placeholder="选择学科" options={subjectOptions(gradeWatch)} />
              </Form.Item>
              <Form.Item name="classType" label="班型">
                <Select options={classTypeOptions} />
              </Form.Item>
              <Button type="primary" htmlType="submit" icon={<CalendarOutlined />} loading={candidates.isFetching}>查找可排时间</Button>
            </div>

            <Button type="link" icon={<SettingOutlined />} className="schedule-more-toggle" onClick={() => setMoreSettingsOpen((value) => !value)}>
              {moreSettingsOpen ? '收起更多设置' : '更多设置'}
            </Button>

            {moreSettingsOpen && (
              <div className="schedule-more-settings">
                <Form.Item name="durationMinutes" label="课长">
                  <InputNumber min={30} step={30} addonAfter="分钟" style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="startDate" label="开始日期">
                  <Input placeholder="2026-06-01" />
                </Form.Item>
                <Form.Item name="endDate" label="结束日期">
                  <Input placeholder="2026-08-31" />
                </Form.Item>
              </div>
            )}
          </Form>
        ) : (
          <Alert type="info" message="当前账号可维护自己的可授课时间，并查看已确认课表。候选排班和确认成班由教务处理。" />
        )}
      </Card>

      {canCreateClass && candidateRequest && readyCandidates.length > 0 && (
        <Card title="可排方案" extra={<Typography.Text type="secondary">满班优先，点「确认成班」即可生成课表</Typography.Text>}>
          <div className="schedule-candidate-list">
            {readyCandidates.map((candidate) => (
              <CandidateCard
                key={candidate.id}
                candidate={candidate}
                teacher={teacherById[candidate.teacherId]}
                selected={candidate.id === selectedCandidate?.id}
                onPick={() => {
                  setSelectedCandidate(candidate);
                  setSelectedStudentIds(candidate.availableStudents.slice(0, candidate.capacity).map((student) => student.id));
                  setSelectedCampusId(user.campusId || 'campus-main');
                }}
              />
            ))}
          </div>
        </Card>
      )}

      {canCreateClass && candidateRequest && shortCandidates.length > 0 && (
        <Card title="协调建议" extra={<Typography.Text type="secondary">时间凑不齐成班，按下方提示协调师生时间后重新查找</Typography.Text>}>
          <CoordinationPanel candidates={shortCandidates} teacherById={teacherById} onCoordinate={openAvailabilityFor} />
        </Card>
      )}

      <Card
        title="周排班工作台"
        extra={(
          <Segmented
            value={viewMode}
            onChange={(value) => setViewMode(value as 'week' | 'month' | 'list')}
            options={[
              { label: '课表视图', value: 'week', icon: <CalendarOutlined /> },
              { label: '月视图', value: 'month', icon: <CalendarOutlined /> },
              { label: '列表视图', value: 'list', icon: <TableOutlined /> }
            ]}
          />
        )}
      >
        <div className="schedule-workbench">
          <div className="schedule-summary-grid">
            <div className="schedule-summary-item">
              <span>已确认课程</span>
              <strong>{activeClassCount}</strong>
            </div>
            <div className="schedule-summary-item">
              <span>可上课时间</span>
              <strong>{availabilitySummary.total}</strong>
            </div>
            <div className="schedule-summary-item">
              <span>推荐方案</span>
              <strong>{recommendedCount}</strong>
            </div>
            <div className="schedule-summary-item">
              <span>待协调时段</span>
              <strong>{shortCandidates.length}</strong>
            </div>
          </div>

          <div className="schedule-outlook-shell">
            <aside className="schedule-outlook-sidebar">
              <div className="schedule-sidebar-section">
                <div className="schedule-sidebar-head">
                  <strong>{calendarMonth.getFullYear()} 年 {calendarMonth.getMonth() + 1} 月</strong>
                  <Space size={4}>
                    <ActionButton tooltip="上个月" icon={<LeftOutlined />} onClick={() => setCalendarMonth(addMonths(calendarMonth, -1))} />
                    <ActionButton tooltip="下个月" icon={<RightOutlined />} onClick={() => setCalendarMonth(addMonths(calendarMonth, 1))} />
                  </Space>
                </div>
                <MiniMonthCalendar
                  month={calendarMonth}
                  selectedWeekStart={selectedWeekStart}
                  onPickDate={(date) => {
                    setSelectedWeekStart(startOfWeek(date));
                    setCalendarMonth(startOfMonth(date));
                  }}
                />
              </div>

              <div className="schedule-sidebar-section">
                <div className="schedule-sidebar-head">
                  <strong>学科日历</strong>
                  {hiddenSubjects.length > 0 && <Button type="link" size="small" onClick={() => setHiddenSubjects([])}>全部显示</Button>}
                </div>
                <div className="schedule-subject-list">
                  {scheduleSubjects.map((subject) => {
                    const color = subjectColor(subject);
                    const hidden = hiddenSubjects.includes(subject);
                    return (
                      <button
                        type="button"
                        className={`schedule-subject-toggle ${hidden ? 'is-muted' : ''}`}
                        key={subject}
                        style={{ '--subject-color': color.accent } as CSSProperties}
                        onClick={() => setHiddenSubjects((values) => values.includes(subject) ? values.filter((item) => item !== subject) : [...values, subject])}
                      >
                        <i />
                        <span>{subject}</span>
                      </button>
                    );
                  })}
                </div>
              </div>

              <div className="schedule-sidebar-section">
                <div className="schedule-sidebar-head">
                  <strong>筛选</strong>
                  {hasClassFilters && (
                    <Button
                      type="link"
                      size="small"
                      onClick={() => {
                        setClassGradeFilter(undefined);
                        setClassSubjectFilter(undefined);
                        setClassTeacherFilter(undefined);
                        setClassStudentFilter(undefined);
                        setClassCampusFilter(undefined);
                        setClassCourseFilter(undefined);
                        setClassTypeFilter(undefined);
                        setHiddenSubjects([]);
                        setStatusFilter('已确认');
                      }}
                    >
                      清空
                    </Button>
                  )}
                </div>
                <div className="schedule-sidebar-filters">
                  <Select allowClear showSearch optionFilterProp="label" placeholder="全部年级" options={gradeOptions()} value={classGradeFilter} onChange={setClassGradeFilter} />
                  <Select allowClear showSearch optionFilterProp="label" placeholder="全部学科" options={classSubjectOptions} value={classSubjectFilter} onChange={setClassSubjectFilter} />
                  <Select allowClear showSearch optionFilterProp="label" placeholder="全部老师" options={teacherOptions} value={classTeacherFilter} onChange={setClassTeacherFilter} />
                  <Select allowClear showSearch optionFilterProp="label" placeholder="全部学生" options={studentOptions} value={classStudentFilter} onChange={setClassStudentFilter} />
                  <Select allowClear showSearch optionFilterProp="label" placeholder="全部校区" options={campusOptions} value={classCampusFilter} onChange={setClassCampusFilter} />
                  <Select allowClear showSearch optionFilterProp="label" placeholder="全部课程" options={courseOptions} value={classCourseFilter} onChange={setClassCourseFilter} />
                  <Select allowClear placeholder="全部班型" options={classTypeOptions} value={classTypeFilter} onChange={setClassTypeFilter} />
                  <Select options={statusOptions} value={statusFilter} onChange={setStatusFilter} />
                </div>
              </div>
            </aside>

            <div className="schedule-outlook-main">
              <div className="schedule-result-note">
                <Typography.Text type={subjectVisibleClasses.length === 0 && hasClassFilters ? 'warning' : 'secondary'}>
                  {classResultNote}
                </Typography.Text>
                <Typography.Text type="secondary">
                  已收集 {availabilitySummary.teacherCount} 个老师时间、{availabilitySummary.studentCount} 个学生时间。
                </Typography.Text>
              </div>

              <div className="schedule-legend">
                <span><i className="legend-dot legend-available-teacher" />老师可授课</span>
                <span><i className="legend-dot legend-available-student" />学生可上课</span>
                <span><i className="legend-dot legend-candidate" />可排方案</span>
                <span><i className="legend-dot legend-confirmed" />已确认</span>
                <span><i className="legend-dot legend-canceled" />已取消</span>
              </div>

              {viewMode === 'week' ? (
                <ScheduleWeekTimeline
                  loading={classes.isFetching || candidates.isFetching || availabilityOverview.isFetching}
                  weekDays={selectedWeekDays}
                  selectedWeekStart={selectedWeekStart}
                  availabilityByDay={availabilityByDay}
                  candidatesByDay={candidatesByDay}
                  classesByDay={classesByDay}
                  courseById={courseById}
                  teacherById={teacherById}
                  studentById={studentById}
                  candidateRequest={candidateRequest}
                  selectedCandidateId={selectedCandidate?.id}
                  emptyTips={emptyTips}
                  canManage={canCreateClass}
                  onPreviousWeek={() => setSelectedWeekStart(addDays(selectedWeekStart, -7))}
                  onNextWeek={() => setSelectedWeekStart(addDays(selectedWeekStart, 7))}
                  onToday={() => {
                    const today = new Date();
                    setSelectedWeekStart(startOfWeek(today));
                    setCalendarMonth(startOfMonth(today));
                  }}
                  onPickCandidate={(record) => {
                    setSelectedCandidate(record);
                    setSelectedStudentIds(record.availableStudents.slice(0, record.capacity).map((student) => student.id));
                    setSelectedCampusId(user.campusId || 'campus-main');
                  }}
                  onEditClass={openEdit}
                  onMoveClass={confirmMoveClass}
                  onCreateClass={openCreateClassForDay}
                />
              ) : viewMode === 'month' ? (
                <MonthScheduleBoard
                  classes={subjectVisibleClasses}
                  courseById={courseById}
                  teacherById={teacherById}
                  canManage={canCreateClass}
                  onEditClass={openEdit}
                  onMoveClass={confirmMoveClass}
                />
              ) : (
                subjectVisibleClasses.length === 0 ? (
                  <Empty
                    description={hasClassFilters ? '没有符合筛选条件的课程。' : '还没有已确认课程。'}
                  >
                    {hasClassFilters && (
                      <Button onClick={() => {
                        setClassTeacherFilter(undefined);
                        setClassCourseFilter(undefined);
                        setClassGradeFilter(undefined);
                        setClassSubjectFilter(undefined);
                        setClassTypeFilter(undefined);
                        setHiddenSubjects([]);
                        setStatusFilter('已确认');
                      }}
                      >
                        清空筛选
                      </Button>
                    )}
                  </Empty>
                ) : (
                  <Table rowKey="id" dataSource={subjectVisibleClasses} pagination={false} columns={classColumns(courseById, teacherById, canCreateClass, openEdit, (id) => cancelClass.mutate(id), cancelClass.isPending)} />
                )
              )}
            </div>
          </div>
        </div>
      </Card>

      <Drawer
        title="维护可上课时间"
        open={availabilityOpen}
        width={560}
        onClose={() => setAvailabilityOpen(false)}
        destroyOnHidden={false}
        extra={<Button type="primary" icon={<SaveOutlined />} loading={saveAvailability.isPending} onClick={() => availabilityForm.submit()}>保存时间</Button>}
      >
        <Form form={availabilityForm} layout="vertical" onFinish={(values) => saveAvailability.mutate(values)}>
          <Form.Item name="ownerKey" label="对象" rules={[{ required: true, message: '请选择老师或学生' }]}>
            <Select options={ownerOptions} onChange={() => setTimeout(() => availability.refetch())} />
          </Form.Item>
          {availability.isFetching ? <Skeleton active paragraph={{ rows: 3 }} /> : (
            <Form.List name="slots">
              {(fields, { add, remove }) => (
                <div className="schedule-slot-list">
                  {fields.map((field) => (
                    <div className="schedule-slot-row" key={field.key}>
                      <Form.Item name={[field.name, 'dayOfWeek']} rules={[{ required: true, message: '请选择星期' }]}>
                        <Select placeholder="星期" options={weekOptions} />
                      </Form.Item>
                      <Form.Item name={[field.name, 'startTime']} rules={[{ required: true, message: '请输入开始时间' }]}>
                        <Input placeholder="19:00" />
                      </Form.Item>
                      <Form.Item name={[field.name, 'endTime']} rules={[{ required: true, message: '请输入结束时间' }]}>
                        <Input placeholder="20:30" />
                      </Form.Item>
                      <ActionButton danger tooltip="删除" icon={<DeleteOutlined />} onClick={() => remove(field.name)} />
                    </div>
                  ))}
                  <Button icon={<PlusOutlined />} onClick={() => add({ dayOfWeek: 3, startTime: '19:00', endTime: '20:30' })}>添加时间段</Button>
                </div>
              )}
            </Form.List>
          )}
        </Form>
      </Drawer>

      <Drawer
        title="确认这个时间"
        open={Boolean(selectedCandidate)}
        width={480}
        onClose={() => setSelectedCandidate(null)}
        extra={<Button type="primary" loading={createClass.isPending} onClick={() => createClass.mutate()}>确认成班</Button>}
      >
        {selectedCandidate && (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <div className="schedule-action-summary">
              <Tag color={candidateLevelMeta(candidateLevel(selectedCandidate)).color}>{candidateLevelMeta(candidateLevel(selectedCandidate)).label}</Tag>
              <Typography.Title level={5}>{courseSubjectGradeText(courseById[selectedCandidate.courseId], selectedCandidate.courseName)}</Typography.Title>
              <Typography.Text type="secondary">{selectedCandidate.courseName}</Typography.Text>
              <Typography.Text>{weekLabel(selectedCandidate.dayOfWeek)} {selectedCandidate.startTime}-{selectedCandidate.endTime}</Typography.Text>
              <Typography.Text type="secondary">{teacherDisplay(selectedCandidate.teacherName, courseById[selectedCandidate.courseId], teacherById[selectedCandidate.teacherId])} · {selectedCandidate.classType} · {selectedCandidate.studentCount}/{selectedCandidate.capacity}</Typography.Text>
              <Typography.Text type="secondary">学生：{studentDisplayNames(selectedCandidate.availableStudents)}</Typography.Text>
            </div>
            <Form layout="vertical">
              <Form.Item label="学生">
                <Select
                  mode="multiple"
                  value={selectedStudentIds}
                  onChange={setSelectedStudentIds}
                  options={selectedCandidate.availableStudents.map((student) => ({ label: studentOptionLabel(student), value: student.id }))}
                  maxCount={selectedCandidate.capacity}
                  style={{ width: '100%' }}
                />
              </Form.Item>
              <Form.Item label="校区">
                <Input value={selectedCampusId} onChange={(event) => setSelectedCampusId(event.target.value)} placeholder="campus-main" />
              </Form.Item>
            </Form>
          </Space>
        )}
      </Drawer>

      <Drawer
        title={creatingClass ? '新建课程' : '课程详情'}
        open={Boolean(editingClass) || creatingClass}
        width={560}
        onClose={() => {
          setEditingClass(null);
          setCreatingClass(false);
        }}
        extra={(editingClass || creatingClass) && (
          <Space>
            {editingClass && (
              <Popconfirm title="取消这节课？" description="取消后该时间不再占用，可重新排课。" okText="取消课程" cancelText="保留" onConfirm={() => cancelClass.mutate(editingClass.id)}>
                <Button danger loading={cancelClass.isPending}>取消课程</Button>
              </Popconfirm>
            )}
            <Button type="primary" loading={updateClass.isPending || createManualClass.isPending} onClick={() => editForm.submit()}>
              {creatingClass ? '创建课程' : '保存调课'}
            </Button>
          </Space>
        )}
      >
        <Form form={editForm} layout="vertical" onFinish={(values) => {
          if (creatingClass) {
            createManualClass.mutate(values);
            return;
          }
          updateClass.mutate(values);
        }}>
          <Form.Item name="courseId" label="课程" rules={[{ required: true, message: '请选择课程' }]}>
            <Select showSearch optionFilterProp="label" options={courseOptions} />
          </Form.Item>
          <Form.Item name="teacherId" label="老师" rules={[{ required: true, message: '请选择老师' }]}>
            <Select showSearch optionFilterProp="label" options={teacherOptions} />
          </Form.Item>
          <Form.Item name="campusId" label="校区">
            <Input placeholder="campus-main" />
          </Form.Item>
          <Space.Compact block>
            <Form.Item name="classType" rules={[{ required: true, message: '请选择班型' }]} style={{ width: '32%' }}>
              <Select options={classTypeOptions} />
            </Form.Item>
            <Form.Item name="durationMinutes" rules={[{ required: true, message: '请输入课长' }]} style={{ width: '34%' }}>
              <InputNumber min={30} step={30} addonAfter="分钟" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="dayOfWeek" rules={[{ required: true, message: '请选择星期' }]} style={{ width: '34%' }}>
              <Select options={weekOptions} />
            </Form.Item>
          </Space.Compact>
          <Space.Compact block>
            <Form.Item name="startTime" rules={[{ required: true, message: '请输入开始时间' }]} style={{ width: '50%' }}>
              <Input placeholder="19:00" />
            </Form.Item>
            <Form.Item name="endTime" rules={[{ required: true, message: '请输入结束时间' }]} style={{ width: '50%' }}>
              <Input placeholder="20:30" />
            </Form.Item>
          </Space.Compact>
          <Space.Compact block>
            <Form.Item name="startDate" style={{ width: '50%' }}>
              <Input placeholder="开始日期" />
            </Form.Item>
            <Form.Item name="endDate" style={{ width: '50%' }}>
              <Input placeholder="结束日期" />
            </Form.Item>
          </Space.Compact>
          <Form.Item
            name="studentIds"
            label={`学生（已选 ${editingStudentIDs.length}/${classCapacity(editingClassType)}）`}
            rules={[{ required: true, message: '请选择学生' }]}
          >
            <Select
              mode="multiple"
              showSearch
              optionFilterProp="label"
              maxCount={classCapacity(editingClassType)}
              options={studentOptions}
            />
          </Form.Item>
          <Typography.Text type="secondary">可直接搜索并增减学生；保存时会校验同年级、开通学科、可上课时间和冲突。</Typography.Text>
        </Form>
      </Drawer>
    </div>
  );
}

function CandidateCard({ candidate, teacher, selected, onPick }: { candidate: ScheduleCandidate; teacher?: Teacher; selected: boolean; onPick: () => void }) {
  const level = candidateLevel(candidate);
  const meta = candidateLevelMeta(level);
  return (
    <div className={`schedule-candidate-card ${selected ? 'is-selected' : ''}`}>
      <div className="schedule-candidate-head">
        <strong>{weekLabel(candidate.dayOfWeek)} {candidate.startTime}-{candidate.endTime}</strong>
        <Tag color={meta.color}>{meta.label}</Tag>
      </div>
      <div className="schedule-candidate-meta">{candidate.subject} · {candidate.grade} · {candidate.classType}</div>
      <div className="schedule-candidate-meta">{teacherDisplay(candidate.teacherName, undefined, teacher)}</div>
      <div className="schedule-candidate-meta">学生（{candidate.studentCount}/{candidate.capacity}）：{studentDisplayNames(candidate.availableStudents)}</div>
      <Button type="primary" size="small" onClick={onPick}>确认成班</Button>
    </div>
  );
}

function CoordinationPanel({ candidates, teacherById, onCoordinate }: { candidates: ScheduleCandidate[]; teacherById: Record<string, Teacher>; onCoordinate: (ownerType: 'teacher' | 'student', ownerId: string) => void }) {
  const rows = [...candidates].sort((left, right) => right.studentCount - left.studentCount).slice(0, 6);
  return (
    <div className="schedule-coordination-list">
      {rows.map((candidate) => (
        <div className="schedule-coordination-item" key={candidate.id}>
          <div className="schedule-coordination-head">
            <strong>{weekLabel(candidate.dayOfWeek)} {candidate.startTime}-{candidate.endTime}</strong>
            <Tag color="orange">还差 {Math.max(minimumStudentCount(candidate.classType) - candidate.studentCount, 1)} 人成班</Tag>
            <Typography.Text type="secondary">{teacherDisplay(candidate.teacherName, undefined, teacherById[candidate.teacherId])} · {candidate.subject}/{candidate.grade} · {candidate.classType}</Typography.Text>
          </div>
          <div className="schedule-coordination-body">
            <div>已可上（{candidate.studentCount}）：{studentDisplayNames(candidate.availableStudents)}</div>
            <div className="schedule-coordination-missing">
              <span>待协调：</span>
              {candidate.missingStudents.length === 0 ? (
                <Typography.Text type="secondary">该时段暂无其他同学科同年级学生</Typography.Text>
              ) : (
                <Space size={[8, 8]} wrap>
                  {candidate.missingStudents.map((student) => (
                    <Button key={student.id} size="small" onClick={() => onCoordinate('student', student.id)}>协调 {studentDisplayName(student)} 时间</Button>
                  ))}
                </Space>
              )}
            </div>
            <Button type="link" size="small" style={{ paddingLeft: 0 }} onClick={() => onCoordinate('teacher', candidate.teacherId)}>调整 {candidate.teacherName} 可授课时间</Button>
          </div>
        </div>
      ))}
    </div>
  );
}

function ScheduleWeekTimeline({
  loading,
  weekDays,
  selectedWeekStart,
  availabilityByDay,
  candidatesByDay,
  classesByDay,
  courseById,
  teacherById,
  studentById,
  candidateRequest,
  selectedCandidateId,
  emptyTips,
  canManage,
  onPreviousWeek,
  onNextWeek,
  onToday,
  onPickCandidate,
  onEditClass,
  onMoveClass,
  onCreateClass
}: {
  loading: boolean;
  weekDays: WeekDay[];
  selectedWeekStart: Date;
  availabilityByDay: Record<number, AvailabilitySlot[]>;
  candidatesByDay: Record<number, ScheduleCandidate[]>;
  classesByDay: Record<number, ScheduleClass[]>;
  courseById: CourseLookup;
  teacherById: Record<string, Teacher>;
  studentById: Record<string, Student>;
  candidateRequest: CandidateFormValues | null;
  selectedCandidateId?: string;
  emptyTips: string[];
  canManage: boolean;
  onPreviousWeek: () => void;
  onNextWeek: () => void;
  onToday: () => void;
  onPickCandidate: (record: ScheduleCandidate) => void;
  onEditClass: (record: ScheduleClass) => void;
  onMoveClass: (record: ScheduleClass, target: ScheduleMoveTarget) => void;
  onCreateClass: (dayOfWeek: number) => void;
}) {
  const timelineItems = useMemo(() => buildTimelineItems(weekDays, availabilityByDay, candidatesByDay, classesByDay, courseById, teacherById, studentById), [weekDays, availabilityByDay, candidatesByDay, classesByDay, courseById, teacherById, studentById]);
  const hasAnyItem = timelineItems.length > 0;
  const timelineRange = useMemo(() => buildTimelineRange(timelineItems), [timelineItems]);
  const rows = useMemo(() => buildTimelineRows(timelineRange.start, timelineRange.end), [timelineRange]);
  const boardHeight = ((timelineRange.end - timelineRange.start) / timelineSlotMinutes) * timelineSlotHeight;
  const itemsByDay = useMemo(() => {
    return weekDays.reduce<Record<number, TimelineLayoutItem[]>>((result, day) => {
      result[day.dayOfWeek] = layoutOverlappingItems(timelineItems.filter((item) => item.dayOfWeek === day.dayOfWeek));
      return result;
    }, {});
  }, [weekDays, timelineItems]);

  if (loading) return <Skeleton active paragraph={{ rows: 6 }} />;

  return (
    <div className="schedule-timeline-wrap">
      {!hasAnyItem && (
        <ScheduleEmptyTips
          description={candidateRequest ? '暂时没有可展示的排课结果。' : '选择课程和老师后，查找可排时间。'}
          tips={emptyTips}
          compact
        />
      )}
      {candidateRequest && !hasGroupedItems(candidatesByDay) && (
        <ScheduleEmptyTips description="没有找到推荐时间。" tips={emptyTips} compact />
      )}
      <div className="schedule-timeline-toolbar">
        <Space size={8}>
          <Button icon={<LeftOutlined />} onClick={onPreviousWeek} />
          <Button onClick={onToday}>今天</Button>
          <Button icon={<RightOutlined />} onClick={onNextWeek} />
        </Space>
        <div>
          <Typography.Title level={4}>{formatWeekRange(selectedWeekStart)}</Typography.Title>
          <Typography.Text type="secondary">拖动已确认课程到目标时间，松开后确认保存；点击课程可快捷调整学生。</Typography.Text>
        </div>
      </div>

      <div className="schedule-timeline-scroll">
        <div className="schedule-timeline-grid" style={{ '--timeline-height': `${boardHeight}px` } as CSSProperties}>
          <div className="schedule-time-gutter schedule-day-head-spacer" />
          {weekDays.map((day) => (
            <div className="schedule-day-head" key={day.key}>
              <strong>{day.day}</strong>
              <span>{day.weekLabel}</span>
              <small>{(classesByDay[day.dayOfWeek] ?? []).filter((item) => item.status !== '已取消').length} 节课</small>
            </div>
          ))}
          <div className="schedule-time-gutter schedule-time-axis" style={{ height: boardHeight }}>
            {rows.map((row) => (
              <div className="schedule-time-label" key={row.minute} style={{ top: row.top }}>
                {row.label}
              </div>
            ))}
          </div>
          {weekDays.map((day) => {
            const dayItems = itemsByDay[day.dayOfWeek] ?? [];
            return (
              <div
                className="schedule-day-column"
                key={day.key}
                style={{ height: boardHeight }}
                onDragOver={(event) => canManage ? event.preventDefault() : undefined}
                onDrop={(event) => {
                  const classID = event.dataTransfer.getData('text/schedule-class-id');
                  const record = Object.values(classesByDay).flat().find((item) => item.id === classID);
                  if (!record) return;
                  const bounds = event.currentTarget.getBoundingClientRect();
                  const offsetMinutes = Math.round((event.clientY - bounds.top) / timelineSlotHeight) * timelineSlotMinutes;
                  const startMinute = clampTimelineStart(timelineRange.start + offsetMinutes, record.durationMinutes);
                  const startTime = formatMinute(startMinute);
                  const endTime = formatMinute(startMinute + record.durationMinutes);
                  onMoveClass(record, {
                    dayOfWeek: day.dayOfWeek,
                    startTime,
                    endTime,
                    label: `${day.label} ${startTime}-${endTime}`
                  });
                }}
                onDoubleClick={(event) => {
                  if (event.currentTarget === event.target && canManage) onCreateClass(day.dayOfWeek);
                }}
              >
                {rows.map((row) => <span className="schedule-time-line" key={row.minute} style={{ top: row.top }} />)}
                {dayItems.length === 0 && (
                  <button type="button" className="schedule-day-empty-slot" onClick={() => canManage ? onCreateClass(day.dayOfWeek) : undefined}>
                    {canManage ? '新建课程' : '暂无安排'}
                  </button>
                )}
                {dayItems.map((item) => (
                  <TimelineBlock
                    key={`${item.kind}-${item.id}`}
                    item={item}
                    rangeStart={timelineRange.start}
                    selectedCandidateId={selectedCandidateId}
                    canManage={canManage}
                    onPickCandidate={onPickCandidate}
                    onEditClass={onEditClass}
                  />
                ))}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function ScheduleEmptyTips({ description, tips, compact = false }: { description: string; tips: string[]; compact?: boolean }) {
  return (
    <div className={compact ? 'schedule-empty-tips compact' : 'schedule-empty-tips'}>
      <Empty description={description} />
      {tips.length > 0 && (
        <div className="schedule-tip-list">
          {tips.map((tip) => <div key={tip}>{tip}</div>)}
        </div>
      )}
    </div>
  );
}

function MiniMonthCalendar({ month, selectedWeekStart, onPickDate }: { month: Date; selectedWeekStart: Date; onPickDate: (date: Date) => void }) {
  const days = useMemo(() => buildMiniMonthDays(month), [month]);
  const selectedKey = localDateText(selectedWeekStart);
  return (
    <div className="schedule-mini-calendar">
      {['一', '二', '三', '四', '五', '六', '日'].map((item) => <span className="schedule-mini-week" key={item}>{item}</span>)}
      {days.map((day) => {
        const weekSelected = localDateText(startOfWeek(day.date)) === selectedKey;
        return (
          <button
            type="button"
            className={`schedule-mini-day ${day.inMonth ? '' : 'is-outside'} ${weekSelected ? 'is-week-selected' : ''} ${day.isToday ? 'is-today' : ''}`}
            key={day.key}
            onClick={() => onPickDate(day.date)}
          >
            {day.date.getDate()}
          </button>
        );
      })}
    </div>
  );
}

function TimelineBlock({
  item,
  rangeStart,
  selectedCandidateId,
  canManage,
  onPickCandidate,
  onEditClass
}: {
  item: TimelineLayoutItem;
  rangeStart: number;
  selectedCandidateId?: string;
  canManage: boolean;
  onPickCandidate: (record: ScheduleCandidate) => void;
  onEditClass: (record: ScheduleClass) => void;
}) {
  const start = timeToMinutes(item.startTime);
  const end = Math.max(timeToMinutes(item.endTime), start + timelineSlotMinutes);
  const color = subjectColor(item.subject);
  const top = ((start - rangeStart) / timelineSlotMinutes) * timelineSlotHeight;
  const height = Math.max(34, ((end - start) / timelineSlotMinutes) * timelineSlotHeight - 4);
  const widthPct = item.widthPct ?? (100 / item.columns);
  const leftPct = item.leftPct ?? ((100 / item.columns) * item.column);
  const width = `calc(${widthPct}% - 4px)`;
  const left = `calc(${leftPct}% + 2px)`;
  const style = {
    top,
    height,
    left,
    width,
    '--subject-bg': color.bg,
    '--subject-border': color.border,
    '--subject-accent': color.accent,
    '--subject-text': color.text
  } as CSSProperties;
  const title = `${item.startTime}-${item.endTime} ${item.title} ${item.subtitle}`;
  const className = [
    'schedule-timeline-block',
    `is-${item.kind}`,
    item.status === '已取消' ? 'is-canceled' : '',
    item.kind === 'candidate' && item.id === selectedCandidateId ? 'is-selected' : ''
  ].filter(Boolean).join(' ');
  const content = (
    <>
      <div className="schedule-timeline-time">{item.startTime}-{item.endTime}</div>
      <strong>{item.title}</strong>
      <span>{item.subtitle}</span>
      <small>{item.meta}</small>
      <div className="schedule-timeline-tags">
        {item.classType && <Tag>{item.classType}</Tag>}
        {item.countText && <Tag>{item.countText}</Tag>}
        {item.status && <Tag color={item.status === '已取消' ? 'default' : 'green'}>{item.status}</Tag>}
      </div>
    </>
  );

  if (item.kind === 'class') {
    const record = item.record as ScheduleClass;
    return (
      <button
        type="button"
        className={className}
        style={style}
        title={title}
        draggable={canManage && record.status !== '已取消'}
        onDragStart={(event) => event.dataTransfer.setData('text/schedule-class-id', record.id)}
        onClick={() => canManage && record.status !== '已取消' ? onEditClass(record) : undefined}
      >
        {content}
      </button>
    );
  }

  if (item.kind === 'candidate') {
    return (
      <button
        type="button"
        className={className}
        style={style}
        title={title}
        onClick={() => onPickCandidate(item.record as ScheduleCandidate)}
      >
        {content}
        <span className="schedule-timeline-action">确认这个时间</span>
      </button>
    );
  }

  return (
    <div className={className} style={style} title={title}>
      {content}
    </div>
  );
}

function MonthScheduleBoard({
  classes,
  courseById,
  teacherById,
  canManage,
  onEditClass,
  onMoveClass
}: {
  classes: ScheduleClass[];
  courseById: CourseLookup;
  teacherById: Record<string, Teacher>;
  canManage: boolean;
  onEditClass: (record: ScheduleClass) => void;
  onMoveClass: (record: ScheduleClass, target: ScheduleMoveTarget) => void;
}) {
  const days = useMemo(() => buildMonthDays(new Date()), []);
  if (classes.length === 0) return <Empty description="还没有可展示的课程。" />;
  return (
    <div className="month-board">
      {days.map((day) => {
        const dayClasses = classes.filter((item) => scheduleClassOccursOn(item, day.date));
        return (
          <div
            className="month-day"
            key={day.key}
            onDragOver={(event) => canManage ? event.preventDefault() : undefined}
            onDrop={(event) => {
              const classID = event.dataTransfer.getData('text/schedule-class-id');
              const record = classes.find((item) => item.id === classID);
              if (record) onMoveClass(record, {
                dayOfWeek: day.dayOfWeek,
                label: day.label,
                startDate: day.key,
                endDate: day.key
              });
            }}
          >
            <div className="month-day-head">
              <strong>{day.day}</strong>
              <span>{day.weekLabel}</span>
            </div>
            <div className="month-day-body">
              {dayClasses.length === 0 ? (
                <span className="month-day-empty">暂无课程</span>
              ) : sortByStartTime(dayClasses).map((item) => {
                const course = courseById[item.courseId];
                return (
                  <button
                    type="button"
                    className={`month-class ${item.status === '已取消' ? 'is-canceled' : ''}`}
                    key={item.id}
                    draggable={canManage && item.status !== '已取消'}
                    onDragStart={(event) => event.dataTransfer.setData('text/schedule-class-id', item.id)}
                    onClick={() => canManage && item.status !== '已取消' ? onEditClass(item) : undefined}
                  >
                    <span>{item.startTime}</span>
                    <strong>{courseSubjectGradeText(course, item.courseName)}</strong>
                    <small>{teacherDisplay(item.teacherName, course, teacherById[item.teacherId])}</small>
                  </button>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function parseOwnerKey(value?: string) {
  if (!value) return null;
  const [ownerType, ownerId] = value.split(':');
  if ((ownerType !== 'teacher' && ownerType !== 'student') || !ownerId) return null;
  return { ownerType, ownerId };
}

function weekLabel(day: number) {
  return weekOptions.find((item) => item.value === day)?.label ?? `周${day}`;
}

function classColumns(courseById: CourseLookup, teacherById: Record<string, Teacher>, canManage: boolean, onEdit: (record: ScheduleClass) => void, onCancel: (id: string) => void, canceling: boolean): TableColumnsType<ScheduleClass> {
  const columns: TableColumnsType<ScheduleClass> = [
    { title: '班级', dataIndex: 'name', width: 160 },
    { title: '科目/年级', width: 130, render: (_, record) => courseSubjectGradeText(courseById[record.courseId], record.courseName) },
    { title: '课程', dataIndex: 'courseName', ellipsis: true },
    { title: '老师', width: 220, render: (_, record) => teacherDisplay(record.teacherName, courseById[record.courseId], teacherById[record.teacherId]) },
    { title: '时间', width: 160, render: (_, record) => `${weekLabel(record.dayOfWeek)} ${record.startTime}-${record.endTime}` },
    { title: '班型', dataIndex: 'classType', width: 90 },
    { title: '学生', render: (_, record) => tagList(record.students.map(studentDisplayName), 'blue') },
    { title: '状态', dataIndex: 'status', width: 100, render: (value) => <Tag color={value === '已取消' ? 'default' : 'green'}>{value}</Tag> }
  ];
  if (canManage) {
    columns.push({
      title: '操作',
      width: 92,
      render: (_, record) => record.status === '已取消' ? <Typography.Text type="secondary">-</Typography.Text> : (
        <Space size={4}>
          <ActionButton tooltip="调课" icon={<EditOutlined />} onClick={() => onEdit(record)} />
          <Popconfirm title="取消这节课？" description="取消后该时间不再占用，可重新排课。" okText="取消课程" cancelText="保留" onConfirm={() => onCancel(record.id)}>
            <ActionButton danger tooltip="取消课程" icon={<CloseCircleOutlined />} loading={canceling} />
          </Popconfirm>
        </Space>
      )
    });
  }
  return columns;
}

function tagList(values: string[], color: string) {
  if (values.length === 0) return <Typography.Text type="secondary">-</Typography.Text>;
  return <Space size={[4, 4]} wrap>{values.map((value) => <Tag color={color} key={value}>{value}</Tag>)}</Space>;
}

function courseSubjectGradeText(course: Course | undefined, fallbackName: string) {
  if (!course) return fallbackName;
  return [course.subject, course.grade].filter(Boolean).join(' · ') || course.name || fallbackName;
}

function teacherOptionLabel(teacher: Teacher) {
  const scope = teacherScopeText(teacher);
  return scope ? `${teacher.name} · ${scope}` : teacher.name;
}

function teacherDisplay(teacherName: string, course: Course | undefined, teacher?: Teacher) {
  const scope = teacher ? teacherScopeText(teacher) : '';
  if (scope) return `教师：${teacherName} · ${scope}`;
  return course?.grade ? `教师：${teacherName} · ${course.grade}` : `教师：${teacherName}`;
}

function teacherScopeText(teacher: Teacher) {
  const grades = shortList(teacher.grades ?? []);
  const subjects = shortList(teacher.subjects ?? []);
  if (grades && subjects) return `${grades}/${subjects}`;
  return grades || subjects;
}

function availabilityOwnerDisplayName(slot: AvailabilitySlot, teacherById: Record<string, Teacher>, studentById: Record<string, Student>) {
  if (slot.ownerType === 'teacher') {
    const teacher = teacherById[slot.ownerId];
    return teacher ? teacherOptionLabel(teacher) : slot.ownerName;
  }
  const student = studentById[slot.ownerId];
  return student ? studentDisplayName(student) : slot.ownerName;
}

function studentOptionLabel(student: { name: string; grade?: string; openedPackages?: string[] }) {
  const base = studentDisplayName(student);
  if (!student.openedPackages || student.openedPackages.length === 0) return base;
  return `${base} · ${shortList(student.openedPackages, 1)}`;
}

function studentDisplayName(student: { name: string; grade?: string }) {
  return student.grade ? `${student.name}（${student.grade}）` : student.name;
}

function studentDisplayNames(students: { name: string; grade?: string }[]) {
  if (students.length === 0) return '暂无学生';
  const values = students.map(studentDisplayName);
  if (values.length <= 3) return values.join('、');
  return `${values.slice(0, 3).join('、')} 等 ${values.length} 人`;
}

function buildTimelineItems(
  weekDays: WeekDay[],
  availabilityByDay: Record<number, AvailabilitySlot[]>,
  candidatesByDay: Record<number, ScheduleCandidate[]>,
  classesByDay: Record<number, ScheduleClass[]>,
  courseById: CourseLookup,
  teacherById: Record<string, Teacher>,
  studentById: Record<string, Student>
) {
  return weekDays.flatMap<TimelineItem>((day) => {
    const availabilityItems = aggregateAvailabilitySlots(availabilityByDay[day.dayOfWeek] ?? [], teacherById, studentById).map<TimelineItem>((group) => ({
      id: group.id,
      kind: 'availability',
      dayOfWeek: day.dayOfWeek,
      startTime: group.startTime,
      endTime: group.endTime,
      subject: group.ownerType === 'teacher' ? '老师可授课' : '学生可上课',
      title: group.ownerType === 'teacher' ? '老师可授课' : '学生可上课',
      subtitle: group.subtitle,
      meta: '可排课时间',
      countText: group.countText,
      record: group.record
    }));
    const candidateItems = sortByStartTime(candidatesByDay[day.dayOfWeek] ?? []).map<TimelineItem>((candidate) => {
      const course = courseById[candidate.courseId];
      const level = candidateLevelMeta(candidateLevel(candidate));
      return {
        id: candidate.id,
        kind: 'candidate',
        dayOfWeek: day.dayOfWeek,
        startTime: candidate.startTime,
        endTime: candidate.endTime,
        subject: candidate.subject || course?.subject || '其他',
        title: courseSubjectGradeText(course, candidate.courseName),
        subtitle: candidate.courseName,
        meta: `${teacherDisplay(candidate.teacherName, course, teacherById[candidate.teacherId])} · 学生：${studentDisplayNames(candidate.availableStudents)}`,
        classType: candidate.classType,
        countText: `${candidate.studentCount}/${candidate.capacity}`,
        status: level.label,
        record: candidate
      };
    });
    const classItems = sortByStartTime(classesByDay[day.dayOfWeek] ?? []).map<TimelineItem>((item) => {
      const course = courseById[item.courseId];
      return {
        id: item.id,
        kind: 'class',
        dayOfWeek: day.dayOfWeek,
        startTime: item.startTime,
        endTime: item.endTime,
        subject: scheduleClassSubject(item, courseById) || '其他',
        title: item.name || courseSubjectGradeText(course, item.courseName),
        subtitle: courseSubjectGradeText(course, item.courseName),
        meta: `${teacherDisplay(item.teacherName, course, teacherById[item.teacherId])} · 学生：${studentDisplayNames(item.students)}`,
        classType: item.classType,
        countText: `${item.students.length}/${item.capacity}`,
        status: item.status,
        record: item
      };
    });
    return [...availabilityItems, ...candidateItems, ...classItems];
  });
}

function aggregateAvailabilitySlots(slots: AvailabilitySlot[], teacherById: Record<string, Teacher>, studentById: Record<string, Student>) {
  const groups = new Map<string, { slots: AvailabilitySlot[]; names: string[] }>();
  slots.forEach((slot) => {
    const key = `${slot.ownerType}-${slot.dayOfWeek}-${slot.startTime}-${slot.endTime}`;
    const group = groups.get(key) ?? { slots: [], names: [] };
    group.slots.push(slot);
    group.names.push(availabilityOwnerDisplayName(slot, teacherById, studentById));
    groups.set(key, group);
  });
  return Array.from(groups.values()).map((group) => {
    const first = group.slots[0];
    const names = group.names;
    const shortNames = names.length <= 2 ? names.join('、') : `${names.slice(0, 2).join('、')} 等`;
    return {
      id: `${first.ownerType}-${first.dayOfWeek}-${first.startTime}-${first.endTime}`,
      ownerType: first.ownerType,
      startTime: first.startTime,
      endTime: first.endTime,
      subtitle: names.length === 1 ? shortNames : `${names.length} 人：${shortNames}`,
      countText: names.length > 1 ? `${names.length}人` : undefined,
      record: first
    };
  });
}

function layoutOverlappingItems(items: TimelineItem[]) {
  const primary = items.filter((item) => item.kind !== 'availability');
  const availability = items.filter((item) => item.kind === 'availability');
  if (primary.length === 0) return layoutOverlappingGroup(availability);
  const primaryItems = layoutOverlappingGroup(primary).map((item) => ({
    ...item,
    leftPct: (76 / item.columns) * item.column,
    widthPct: 76 / item.columns
  }));
  const availabilityItems = layoutOverlappingGroup(availability).map((item) => ({
    ...item,
    leftPct: 78 + (20 / item.columns) * item.column,
    widthPct: 20 / item.columns
  }));
  return [...availabilityItems, ...primaryItems].sort((left, right) => timeToMinutes(left.startTime) - timeToMinutes(right.startTime));
}

function layoutOverlappingGroup(items: TimelineItem[]) {
  const sorted = [...items].sort((left, right) => {
    const diff = timeToMinutes(left.startTime) - timeToMinutes(right.startTime);
    return diff || timeToMinutes(left.endTime) - timeToMinutes(right.endTime);
  });
  const result: TimelineLayoutItem[] = [];
  let group: TimelineItem[] = [];
  let groupEnd = -1;

  function flushGroup() {
    if (group.length === 0) return;
    const columnEnds: number[] = [];
    const laidOut = group.map((item) => {
      const start = timeToMinutes(item.startTime);
      const end = Math.max(timeToMinutes(item.endTime), start + timelineSlotMinutes);
      let column = columnEnds.findIndex((value) => value <= start);
      if (column === -1) {
        column = columnEnds.length;
        columnEnds.push(end);
      } else {
        columnEnds[column] = end;
      }
      return { ...item, column, columns: 1 };
    });
    const columns = Math.max(1, columnEnds.length);
    laidOut.forEach((item) => result.push({ ...item, columns }));
    group = [];
    groupEnd = -1;
  }

  sorted.forEach((item) => {
    const start = timeToMinutes(item.startTime);
    const end = Math.max(timeToMinutes(item.endTime), start + timelineSlotMinutes);
    if (group.length > 0 && start >= groupEnd) flushGroup();
    group.push(item);
    groupEnd = Math.max(groupEnd, end);
  });
  flushGroup();
  return result;
}

function buildTimelineRange(items: TimelineItem[]) {
  const minutes = items.flatMap((item) => [timeToMinutes(item.startTime), timeToMinutes(item.endTime)]).filter((value) => Number.isFinite(value));
  if (minutes.length === 0) return { start: defaultTimelineStart, end: defaultTimelineEnd };
  const min = Math.min(defaultTimelineStart, ...minutes);
  const max = Math.max(defaultTimelineEnd, ...minutes);
  return {
    start: Math.max(0, Math.floor(min / 60) * 60),
    end: Math.min(24 * 60, Math.ceil(max / 60) * 60)
  };
}

function buildTimelineRows(start: number, end: number) {
  const rows = [];
  for (let minute = start; minute <= end; minute += timelineSlotMinutes) {
    rows.push({
      minute,
      top: ((minute - start) / timelineSlotMinutes) * timelineSlotHeight,
      label: formatMinute(minute)
    });
  }
  return rows;
}

function timeToMinutes(value: string) {
  const match = /^(\d{1,2}):(\d{2})$/.exec(value || '');
  if (!match) return defaultTimelineStart;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  if (!Number.isFinite(hour) || !Number.isFinite(minute)) return defaultTimelineStart;
  return Math.max(0, Math.min(24 * 60, hour * 60 + minute));
}

function clampTimelineStart(startMinute: number, durationMinutes: number) {
  const maxStart = Math.max(0, 23 * 60 + 30 - durationMinutes);
  return Math.max(0, Math.min(maxStart, startMinute));
}

function classCapacity(classType?: string) {
  const match = /1V([1-4])/.exec(classType || '');
  return match ? Number(match[1]) : 4;
}

function formatMinute(value: number) {
  const hour = Math.floor(value / 60);
  const minute = value % 60;
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
}

function subjectColor(subject: string) {
  const normalized = subject.toLowerCase();
  if (normalized.includes('eng') || subject.includes('英文') || subject.includes('英语')) return { bg: '#d8ebfb', border: '#a9cdea', accent: '#2f7dcc', text: '#14395d' };
  if (normalized.includes('math') || subject.includes('数学')) return { bg: '#fff5c9', border: '#f1d673', accent: '#d4a900', text: '#594700' };
  if (normalized.includes('geo') || normalized.includes('his') || subject.includes('地理') || subject.includes('历史')) return { bg: '#d8f0f5', border: '#9ed4df', accent: '#32899b', text: '#164e59' };
  if (normalized.includes('sci') || subject.includes('科学') || subject.includes('物理') || subject.includes('化学') || subject.includes('生物')) return { bg: '#dfe6ff', border: '#adbdf4', accent: '#3556c4', text: '#1e2d68' };
  if (normalized.includes('chn') || subject.includes('语文') || subject.includes('中文')) return { bg: '#efd9f6', border: '#d7a9e6', accent: '#9b43bd', text: '#552066' };
  if (subject.includes('老师')) return { bg: '#eef9f1', border: '#b8e2c4', accent: '#32975a', text: '#19542f' };
  if (subject.includes('学生')) return { bg: '#eef6ff', border: '#b9d8fb', accent: '#347fc4', text: '#1e4d78' };
  const palette = [
    { bg: '#f1e9dc', border: '#d8c5a5', accent: '#9a6b2f', text: '#553712' },
    { bg: '#e5f3e8', border: '#b8d9c1', accent: '#3d8d58', text: '#1f5130' },
    { bg: '#f6e2e2', border: '#e5b5b5', accent: '#b24b4b', text: '#672323' },
    { bg: '#e8edf4', border: '#c3ccd9', accent: '#5d6f89', text: '#2e3b4f' }
  ];
  const index = Math.abs([...subject].reduce((sum, char) => sum + char.charCodeAt(0), 0)) % palette.length;
  return palette[index];
}

function shortList(values: string[], limit = 2) {
  const cleaned = values.filter(Boolean);
  if (cleaned.length === 0) return '';
  if (cleaned.length <= limit) return cleaned.join('、');
  return `${cleaned.slice(0, limit).join('、')}等`;
}

function scheduleResultNote(items: ScheduleClass[], totalConfirmedCount: number, hasFilters: boolean, grade?: string) {
  const gradeText = grade ? `${grade}课程` : '课程';
  if (items.length === 0) {
    return hasFilters ? `没有符合筛选条件的${gradeText}，可清空筛选查看全部已确认课程。` : '还没有已确认课程，确认候选时间后会显示在这里。';
  }
  const confirmedCount = items.filter((item) => item.status === '已确认').length;
  const canceledCount = items.filter((item) => item.status === '已取消').length;
  if (canceledCount > 0) {
    return `当前显示 ${items.length} 节${gradeText}，其中已确认 ${confirmedCount} 节、已取消 ${canceledCount} 节。全部已确认课程 ${totalConfirmedCount} 节。`;
  }
  return `当前显示 ${confirmedCount} 节已确认${gradeText}，全部已确认课程 ${totalConfirmedCount} 节。`;
}

function availabilityStats(items: AvailabilitySlot[]) {
  return {
    total: items.length,
    teacherCount: items.filter((item) => item.ownerType === 'teacher').length,
    studentCount: items.filter((item) => item.ownerType === 'student').length
  };
}

function uniqueScheduleCampuses(items: ScheduleClass[]) {
  const values = items
    .map((item) => item.campusId || 'campus-main')
    .filter(Boolean);
  return Array.from(new Set(values)).sort();
}

function uniqueScheduleSubjects(classes: ScheduleClass[], candidates: ScheduleCandidate[], courseById: CourseLookup) {
  const values = [
    ...classes.map((item) => scheduleClassSubject(item, courseById)),
    ...candidates.map((item) => item.subject || courseById[item.courseId]?.subject),
    '老师可授课',
    '学生可上课'
  ].map((value) => value || '其他');
  return Array.from(new Set(values)).sort((left, right) => {
    const priority = ['Eng', '英文', '英语', 'Math', '数学', 'Geo/His', 'Sci', '科学', '综合科学', 'CHN', '语文'];
    const leftIndex = priority.findIndex((item) => left.includes(item));
    const rightIndex = priority.findIndex((item) => right.includes(item));
    if (leftIndex !== -1 || rightIndex !== -1) return (leftIndex === -1 ? 99 : leftIndex) - (rightIndex === -1 ? 99 : rightIndex);
    return left.localeCompare(right, 'zh-Hans-CN');
  });
}

function filterClasses(items: ScheduleClass[], filters: ScheduleFilters, courseById: CourseLookup) {
  return items.filter((item) =>
    (!filters.grade || scheduleClassGrade(item, courseById) === filters.grade) &&
    (!filters.subject || scheduleClassSubject(item, courseById) === filters.subject) &&
    (!filters.teacherId || item.teacherId === filters.teacherId) &&
    (!filters.studentId || item.students.some((student) => student.id === filters.studentId)) &&
    (!filters.campusId || (item.campusId || 'campus-main') === filters.campusId) &&
    (!filters.courseId || item.courseId === filters.courseId) &&
    (!filters.classType || item.classType === filters.classType) &&
    (!filters.status || filters.status === '全部' || item.status === filters.status)
  );
}

function scheduleClassGrade(item: ScheduleClass, courseById: CourseLookup) {
  return courseById[item.courseId]?.grade || item.students[0]?.grade || '';
}

function scheduleClassSubject(item: ScheduleClass, courseById: CourseLookup) {
  return courseById[item.courseId]?.subject || '';
}

function groupScheduleItems<T extends { dayOfWeek: number; startTime: string }>(items: T[]) {
  return items.reduce<Record<number, T[]>>((result, item) => {
    result[item.dayOfWeek] = [...(result[item.dayOfWeek] ?? []), item].sort((left, right) => left.startTime.localeCompare(right.startTime));
    return result;
  }, {});
}

function scheduleClassPayload(record: ScheduleClass, target: ScheduleMoveTarget = { dayOfWeek: record.dayOfWeek, label: weekLabel(record.dayOfWeek) }): ScheduleClassFormValues {
  return {
    courseId: record.courseId,
    teacherId: record.teacherId,
    campusId: record.campusId || 'campus-main',
    roomName: '',
    classType: record.classType,
    durationMinutes: record.durationMinutes,
    dayOfWeek: target.dayOfWeek,
    startTime: target.startTime ?? record.startTime,
    endTime: target.endTime ?? record.endTime,
    startDate: target.startDate ?? record.startDate,
    endDate: target.endDate ?? record.endDate,
    studentIds: record.students.map((student) => student.id)
  };
}

function buildWeekDays(weekStart: Date): WeekDay[] {
  return Array.from({ length: 7 }, (_, index) => {
    const date = addDays(weekStart, index);
    const dayOfWeek = date.getDay() === 0 ? 7 : date.getDay();
    return {
      key: localDateText(date),
      date,
      day: date.getDate(),
      dayOfWeek,
      weekLabel: weekLabel(dayOfWeek),
      label: `${date.getMonth() + 1}月${date.getDate()}日 ${weekLabel(dayOfWeek)}`
    };
  });
}

function buildMonthDays(base: Date) {
  const first = new Date(base.getFullYear(), base.getMonth(), 1);
  const last = new Date(base.getFullYear(), base.getMonth() + 1, 0);
  const start = new Date(first);
  start.setDate(first.getDate() - ((first.getDay() + 6) % 7));
  const days = [];
  for (let index = 0; index < 42; index++) {
    const date = new Date(start);
    date.setDate(start.getDate() + index);
    const dayOfWeek = date.getDay() === 0 ? 7 : date.getDay();
    days.push({
      key: localDateText(date),
      date,
      day: date.getDate(),
      dayOfWeek,
      weekLabel: weekLabel(dayOfWeek),
      label: `${date.getMonth() + 1}月${date.getDate()}日 ${weekLabel(dayOfWeek)}`,
      inMonth: date.getMonth() === base.getMonth()
    });
  }
  return days.filter((day) => day.inMonth || day.date >= first && day.date <= last);
}

function buildMiniMonthDays(base: Date) {
  const first = new Date(base.getFullYear(), base.getMonth(), 1);
  const start = startOfWeek(first);
  const todayKey = localDateText(new Date());
  return Array.from({ length: 42 }, (_, index) => {
    const date = addDays(start, index);
    const key = localDateText(date);
    return {
      key,
      date,
      inMonth: date.getMonth() === base.getMonth(),
      isToday: key === todayKey
    };
  });
}

function scheduleClassOccursOn(item: ScheduleClass, date: Date) {
  const dayOfWeek = date.getDay() === 0 ? 7 : date.getDay();
  if (item.dayOfWeek !== dayOfWeek) return false;
  const dateText = localDateText(date);
  if (item.startDate && dateText < item.startDate) return false;
  if (item.endDate && dateText > item.endDate) return false;
  return true;
}

function localDateText(date: Date) {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

function startOfWeek(date: Date) {
  const result = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const offset = (result.getDay() + 6) % 7;
  result.setDate(result.getDate() - offset);
  return result;
}

function startOfMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addDays(date: Date, days: number) {
  const result = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  result.setDate(result.getDate() + days);
  return result;
}

function addMonths(date: Date, months: number) {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

function formatWeekRange(weekStart: Date) {
  const weekEnd = addDays(weekStart, 6);
  if (weekStart.getFullYear() === weekEnd.getFullYear()) {
    return `${weekStart.getFullYear()} 年 ${weekStart.getMonth() + 1} 月 ${weekStart.getDate()} 日 - ${weekEnd.getMonth() + 1} 月 ${weekEnd.getDate()} 日`;
  }
  return `${weekStart.getFullYear()} 年 ${weekStart.getMonth() + 1} 月 ${weekStart.getDate()} 日 - ${weekEnd.getFullYear()} 年 ${weekEnd.getMonth() + 1} 月 ${weekEnd.getDate()} 日`;
}

function hasGroupedItems<T>(groups: Record<number, T[]>) {
  return Object.values(groups).some((items) => items.length > 0);
}

function sortByStartTime<T extends { startTime: string }>(items: T[]) {
  return [...items].sort((left, right) => left.startTime.localeCompare(right.startTime));
}

function minimumStudentCount(classType: string) {
  if (classType === '1V1') return 1;
  if (classType === '1V2') return 2;
  if (classType === '1V3') return 2;
  if (classType === '1V4') return 3;
  return 1;
}

function candidateLevel(candidate: ScheduleCandidate): CandidateLevel {
  if (candidate.studentCount >= candidate.capacity) return 'full';
  if (candidate.studentCount >= minimumStudentCount(candidate.classType)) return 'ready';
  return 'short';
}

function candidateLevelMeta(level: CandidateLevel) {
  if (level === 'full') return { label: '满班推荐', color: 'green' };
  if (level === 'ready') return { label: '可开班', color: 'blue' };
  return { label: '人数不足', color: 'default' };
}

function candidateEmptyTips(request: CandidateFormValues | null, candidates: ScheduleCandidate[]) {
  if (!request) return ['先选择学科和年级，再查找可排时间。'];
  if (candidates.length > 0) return [];
  return [
    '确认该学科 + 年级已有开通学生，且老师授课范围覆盖该学科年级。',
    '让相关老师和学生补充可上课时间后再查找。',
    '可缩短课长或更换班型，更容易凑齐时间。'
  ];
}
