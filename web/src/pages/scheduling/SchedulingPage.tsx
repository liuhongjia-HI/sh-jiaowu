import { CalendarOutlined, CloseCircleOutlined, DeleteOutlined, DownOutlined, EditOutlined, LeftOutlined, PlusOutlined, ReloadOutlined, RightOutlined, SaveOutlined, SettingOutlined, TableOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Drawer, Empty, Form, Input, InputNumber, Modal, Popconfirm, Segmented, Select, Skeleton, Space, Table, Tag, Typography, message } from 'antd';
import type { TableColumnsType } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { getData, postData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { gradeOptions, subjectOptions } from '../../utils/curriculum';
import type { AvailabilitySlot, Course, CurrentUser, ScheduleCandidate, ScheduleClass, Student, Teacher } from '../../types/starline';
import {
  addDays,
  addMonths,
  availabilityCovers,
  candidateLevel,
  candidateLevelMeta,
  classCapacity,
  formatWeekRange,
  isClockText,
  localDateText,
  minimumStudentCount,
  scheduleClassOccursOn,
  startOfMonth,
  startOfWeek,
  weekLabel,
  weekOptions
} from './scheduling-utils';
import { CandidatePanel, CoordinationPanel } from './CandidatePanel';
import {
  MiniMonthCalendar,
  MonthScheduleBoard,
  ScheduleDayResourceTimeline,
  ScheduleWeekTimeline,
  availabilityStats,
  buildMiniMonthDays,
  buildWeekDays,
  candidateEmptyTips,
  classColumns,
  findNearestClassDate,
  courseSubjectGradeText,
  filterClasses,
  groupScheduleItems,
  parseOwnerKey,
  scheduleClassPayload,
  scheduleClassSubject,
  scheduleResultNote,
  studentDisplayName,
  studentDisplayNames,
  studentOptionLabel,
  subjectColor,
  teacherDisplay,
  teacherOptionLabel,
  uniqueScheduleCampuses,
  uniqueScheduleSubjects
} from './SchedulingViews';

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
  expectedStudentCount: number;
  reservationNote?: string;
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

const classTypeOptions = ['1V1', '1V2', '1V3', '1V4'].map((value) => ({ label: value, value }));
export default function Scheduling({ user }: { user: CurrentUser }) {
  const [availabilityForm] = Form.useForm<AvailabilityFormValues>();
  const [candidateForm] = Form.useForm<CandidateFormValues>();
  const [editForm] = Form.useForm<ScheduleClassFormValues>();
  const [candidateRequest, setCandidateRequest] = useState<CandidateFormValues | null>(null);
  // 侧栏（迷你日历 / 学科日历 / 筛选）默认收起：它们是「偶尔用一次」的辅助工具，
  // 常驻展开会占掉 260px，把真正要看的排班网格挤窄一档。收起后留一条竖栏可随时展开，
  // 并在上面标出生效中的筛选条数——否则筛选被藏起来，结果变少却看不出原因。
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [selectedCandidate, setSelectedCandidate] = useState<ScheduleCandidate | null>(null);
  const [selectedStudentIds, setSelectedStudentIds] = useState<string[]>([]);
  const [selectedCampusId, setSelectedCampusId] = useState(user.campusId || 'campus-main');
  const [editingClass, setEditingClass] = useState<ScheduleClass | null>(null);
  const [creatingClass, setCreatingClass] = useState(false);
  const [availabilityOpen, setAvailabilityOpen] = useState(false);
  const [moreSettingsOpen, setMoreSettingsOpen] = useState(false);
  // 默认停在资源泳道日视图：这是排课场景真正读得清的密度，周视图退居总览。
  const [viewMode, setViewMode] = useState<'day' | 'week' | 'month' | 'list'>('day');
  const [classGradeFilter, setClassGradeFilter] = useState<string>();
  const [classSubjectFilter, setClassSubjectFilter] = useState<string>();
  const [classTeacherFilter, setClassTeacherFilter] = useState<string>();
  const [classStudentFilter, setClassStudentFilter] = useState<string>();
  const [classCampusFilter, setClassCampusFilter] = useState<string>();
  const [classCourseFilter, setClassCourseFilter] = useState<string>();
  const [classTypeFilter, setClassTypeFilter] = useState<string>();
  const [statusFilter, setStatusFilter] = useState<string>('全部');
  const [selectedWeekStart, setSelectedWeekStart] = useState<Date>(() => startOfWeek(new Date()));
  // 日视图选中的那一天。始终保持在 selectedWeekStart 所在周内，
  // 这样日/周视图共用同一份按 dayOfWeek 分组的数据，切换视图不用重新取数。
  const [selectedDate, setSelectedDate] = useState<Date>(() => new Date());
  const [calendarMonth, setCalendarMonth] = useState<Date>(() => startOfMonth(new Date()));
  const [hiddenSubjects, setHiddenSubjects] = useState<string[]>([]);
  // 图例是「偶尔来关掉某个学科」的工具，不是每次进页面都要读的信息，展开着会把下面的
  // 筛选区整块顶出首屏。默认折叠，折叠时在标题上直接写清「隐藏了几个学科」——
  // 否则收起后颜色对不上号，用户看到少了课程也不知道是自己关掉的。
  const [subjectLegendOpen, setSubjectLegendOpen] = useState(false);
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
  const editingTeacherId = Form.useWatch('teacherId', editForm);
  const editingDayOfWeek = Form.useWatch('dayOfWeek', editForm);
  const editingStartTime = Form.useWatch('startTime', editForm);
  const editingEndTime = Form.useWatch('endTime', editForm);
  const editingStartDate = Form.useWatch('startDate', editForm);
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
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败，请检查星期和时间段。')
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
        studentIds: selectedStudentIds,
        expectedStudentCount: selectedCandidate.capacity,
        reservationNote: ''
      });
    },
    onSuccess: (record) => {
      message.success(record.status === '待确认' ? '已锁定时间段，课程待确认' : '已确认成班，课表已生成');
      setSelectedCandidate(null);
      setSelectedStudentIds([]);
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '确认排课失败，请检查人数和时间冲突。')
  });

  const cancelClass = useMutation({
    mutationFn: (id: string) => postData<ScheduleClass>(`/schedule-classes/${id}/cancel`, {}),
    onSuccess: () => {
      message.success('课程已取消，可重新生成候选排课');
      setEditingClass(null);
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '取消课程失败，请稍后重试。')
  });

  const createManualClass = useMutation({
    mutationFn: (values: ScheduleClassFormValues) => postData<ScheduleClass>('/schedule-classes', values),
    onSuccess: (record) => {
      message.success(record.status === '待确认' ? '时间段已锁定，后续可补充学生' : '课程已创建，课表已更新');
      setCreatingClass(false);
      editForm.resetFields();
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '创建课程失败，请稍后重试。')
  });

  const updateClass = useMutation({
    mutationFn: (values: ScheduleClassFormValues) => {
      if (!editingClass) throw new Error('请选择要调整的课程');
      return putData<ScheduleClass>(`/schedule-classes/${editingClass.id}`, values);
    },
    onSuccess: (record) => {
      message.success(record.status === '待确认' ? '调课已保存，当前课程待确认' : '调课已保存');
      setEditingClass(null);
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '调课失败，请稍后重试。')
  });

  const moveClass = useMutation({
    mutationFn: ({ record, target }: { record: ScheduleClass; target: ScheduleMoveTarget }) =>
      putData<ScheduleClass>(`/schedule-classes/${record.id}`, scheduleClassPayload(record, target)),
    onSuccess: () => {
      message.success('调课已保存');
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-candidates'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '调课失败，请稍后重试。')
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
    { label: '全部状态', value: '全部' },
    { label: '待确认', value: '待确认' },
    { label: '已确认', value: '已确认' },
    { label: '已取消', value: '已取消' }
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
  // 建课前先按后端同一套规则核对可上课时间：谁没有覆盖该时段的时间，提交必然被拒。
  // 提前列出来并给出「维护时间」入口，比提交后收到一句报错再回头猜要快得多。
  const availabilityGaps = useMemo(() => {
    if (!isClockText(editingStartTime) || !isClockText(editingEndTime) || !editingDayOfWeek) return [];
    if (editingEndTime <= editingStartTime) return [];
    const date = (editingStartDate ?? '').trim();
    const slots = availabilityOverview.data ?? [];
    const covered = (ownerType: 'teacher' | 'student', ownerId: string) =>
      slots.some((slot) => slot.ownerType === ownerType && slot.ownerId === ownerId
        && availabilityCovers(slot, editingDayOfWeek, editingStartTime, editingEndTime, date));
    const gaps: { ownerType: 'teacher' | 'student'; ownerId: string; name: string }[] = [];
    if (editingTeacherId && !covered('teacher', editingTeacherId)) {
      const teacher = teacherById[editingTeacherId];
      gaps.push({ ownerType: 'teacher', ownerId: editingTeacherId, name: teacher ? teacher.name : '该老师' });
    }
    for (const studentId of editingStudentIDs) {
      if (covered('student', studentId)) continue;
      const student = studentById[studentId];
      gaps.push({ ownerType: 'student', ownerId: studentId, name: student ? studentDisplayName(student) : '该学生' });
    }
    return gaps;
  }, [availabilityOverview.data, editingDayOfWeek, editingStartTime, editingEndTime, editingStartDate, editingTeacherId, editingStudentIDs, teacherById, studentById]);
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
  // 迷你日历上标出这个月每天有几节课。数据源用 subjectVisibleClasses（跟着筛选走、不限于当前周），
  // 这样翻月份时标记也跟着走，日视图才有得可跳。
  const classCountByDate = useMemo(() => {
    const result: Record<string, number> = {};
    buildMiniMonthDays(calendarMonth).forEach((day) => {
      const count = subjectVisibleClasses.filter((item) => item.status !== '已取消' && scheduleClassOccursOn(item, day.date)).length;
      if (count > 0) result[day.key] = count;
    });
    return result;
  }, [calendarMonth, subjectVisibleClasses]);
  const nearestClassDate = useMemo(
    () => findNearestClassDate(subjectVisibleClasses, selectedDate),
    [subjectVisibleClasses, selectedDate]
  );
  const candidatesByDay = useMemo(() => groupScheduleItems(readyCandidates), [readyCandidates]);
  const availabilityByDay = useMemo(() => groupScheduleItems(availabilityOverview.data ?? []), [availabilityOverview.data]);
  const availabilitySummary = useMemo(() => availabilityStats(availabilityOverview.data ?? []), [availabilityOverview.data]);
  const activeClassCount = subjectVisibleClasses.filter((item) => item.status === '已确认').length;
  const totalConfirmedClassCount = (classes.data ?? []).filter((item) => item.status === '已确认').length;
  const activeFilterCount = [classGradeFilter, classSubjectFilter, classTeacherFilter, classStudentFilter, classCampusFilter, classCourseFilter, classTypeFilter]
    .filter(Boolean).length + (statusFilter !== '全部' ? 1 : 0) + (hiddenSubjects.length > 0 ? 1 : 0);
  const hasClassFilters = Boolean(classGradeFilter || classSubjectFilter || classTeacherFilter || classStudentFilter || classCampusFilter || classCourseFilter || classTypeFilter || hiddenSubjects.length > 0 || statusFilter !== '全部');
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
      roomName: record.roomName,
      classType: record.classType,
      durationMinutes: record.durationMinutes,
      dayOfWeek: record.dayOfWeek,
      startTime: record.startTime,
      endTime: record.endTime,
      startDate: record.startDate,
      studentIds: record.students.map((student) => student.id),
      expectedStudentCount: record.expectedStudentCount,
      reservationNote: record.reservationNote
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
      studentIds: [],
      expectedStudentCount: 1,
      reservationNote: ''
    });
  }

  // 日视图翻页要顺带把周对齐过去：classesByDay 是按「选中周」筛出来再按 dayOfWeek 分组的，
  // 只挪 selectedDate 不挪 selectedWeekStart，跨周之后日视图就会读到上一周的数据。
  function goToDate(date: Date) {
    setSelectedDate(date);
    setSelectedWeekStart(startOfWeek(date));
    setCalendarMonth(startOfMonth(date));
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
              <CandidatePanel
                key={candidate.id}
                candidate={candidate}
                teacher={teacherById[candidate.teacherId]}
                selected={candidate.id === selectedCandidate?.id}
                teacherText={(name, teacher) => teacherDisplay(name, undefined, teacher)}
                studentText={studentDisplayNames}
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
          <CoordinationPanel
            candidates={shortCandidates}
            teacherById={teacherById}
            onCoordinate={openAvailabilityFor}
            teacherText={(name, teacher) => teacherDisplay(name, undefined, teacher)}
            studentText={studentDisplayNames}
            studentLabel={studentDisplayName}
          />
        </Card>
      )}

      <Card
        // 标题跟着视图走：默认已经是日视图，再顶着「周排班」会让人以为切错了。
        title={viewMode === 'day' ? '排班工作台 · 按老师看一天' : viewMode === 'week' ? '排班工作台 · 周总览' : '排班工作台'}
        extra={(
          <Segmented
            value={viewMode}
            onChange={(value) => setViewMode(value as 'day' | 'week' | 'month' | 'list')}
            options={[
              { label: '日视图', value: 'day', icon: <CalendarOutlined /> },
              { label: '周视图', value: 'week', icon: <CalendarOutlined /> },
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

          <div className={sidebarOpen ? 'schedule-outlook-shell' : 'schedule-outlook-shell is-collapsed'}>
            {!sidebarOpen && (
              <button
                type="button"
                className="schedule-sidebar-rail"
                aria-expanded={false}
                onClick={() => setSidebarOpen(true)}
              >
                <RightOutlined />
                <span>日历与筛选</span>
                {activeFilterCount > 0 && <em>{activeFilterCount}</em>}
              </button>
            )}
            <aside className="schedule-outlook-sidebar" hidden={!sidebarOpen}>
              <div className="schedule-sidebar-collapse">
                <Button type="text" size="small" icon={<LeftOutlined />} onClick={() => setSidebarOpen(false)}>收起</Button>
              </div>
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
                  selectedDate={selectedDate}
                  highlight={viewMode === 'day' ? 'day' : 'week'}
                  classCountByDate={classCountByDate}
                  onPickDate={goToDate}
                />
              </div>

              <div className="schedule-sidebar-section">
                <div className="schedule-sidebar-head">
                  <button
                    type="button"
                    className={`schedule-sidebar-toggle ${subjectLegendOpen ? 'is-open' : ''}`}
                    aria-expanded={subjectLegendOpen}
                    onClick={() => setSubjectLegendOpen((open) => !open)}
                  >
                    <DownOutlined />
                    <strong>学科日历</strong>
                    {hiddenSubjects.length > 0 && <em>已隐藏 {hiddenSubjects.length} 个</em>}
                  </button>
                  {hiddenSubjects.length > 0 && <Button type="link" size="small" onClick={() => setHiddenSubjects([])}>全部显示</Button>}
                </div>
                {subjectLegendOpen && (
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
                )}
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
                        setStatusFilter('全部');
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
                <span><i className="legend-dot legend-candidate" />待确认</span>
                <span><i className="legend-dot legend-confirmed" />已确认</span>
                <span><i className="legend-dot legend-canceled" />已取消</span>
              </div>

              {viewMode === 'day' ? (
                <ScheduleDayResourceTimeline
                  loading={classes.isFetching || candidates.isFetching || availabilityOverview.isFetching}
                  selectedDate={selectedDate}
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
                  nearestClassDate={nearestClassDate}
                  onPreviousDay={() => goToDate(addDays(selectedDate, -1))}
                  onNextDay={() => goToDate(addDays(selectedDate, 1))}
                  onToday={() => goToDate(new Date())}
                  onJumpToDate={goToDate}
                  onPickCandidate={(record) => {
                    setSelectedCandidate(record);
                    setSelectedStudentIds(record.availableStudents.slice(0, record.capacity).map((student) => student.id));
                    setSelectedCampusId(user.campusId || 'campus-main');
                  }}
                  onEditClass={openEdit}
                  onMoveClass={confirmMoveClass}
                  onCreateClass={openCreateClassForDay}
                />
              ) : viewMode === 'week' ? (
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
                  onPreviousWeek={() => goToDate(addDays(selectedDate, -7))}
                  onNextWeek={() => goToDate(addDays(selectedDate, 7))}
                  onToday={() => goToDate(new Date())}
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
                    description={hasClassFilters ? '没有符合筛选条件的课程。' : '还没有课程。'}
                  >
                    {hasClassFilters && (
                      <Button onClick={() => {
                        setClassTeacherFilter(undefined);
                        setClassCourseFilter(undefined);
                        setClassGradeFilter(undefined);
                        setClassSubjectFilter(undefined);
                        setClassTypeFilter(undefined);
                        setHiddenSubjects([]);
                        setStatusFilter('全部');
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
        {availabilityGaps.length > 0 && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message="该时段没有可上课时间，直接提交会被拒绝"
            description={(
              <Space direction="vertical" size={4} style={{ width: '100%' }}>
                <Typography.Text type="secondary">
                  下列老师/学生在 {weekLabel(editingDayOfWeek)} {editingStartTime}-{editingEndTime} 没有登记可上课时间，先补一条再建课。
                </Typography.Text>
                {availabilityGaps.map((gap) => (
                  <Space key={`${gap.ownerType}-${gap.ownerId}`} size={8}>
                    <Typography.Text>{gap.ownerType === 'teacher' ? '老师' : '学生'}：{gap.name}</Typography.Text>
                    <Button size="small" type="link" onClick={() => openAvailabilityFor(gap.ownerType, gap.ownerId)}>去维护时间</Button>
                  </Space>
                ))}
              </Space>
            )}
          />
        )}
        <Form form={editForm} layout="vertical" onFinish={(values) => {
          // 课外辅导不跨天：这里只让家长/老师选一个上课日期，结束日期在提交时直接等于开始日期，不再单独收集。
          const payload = { ...values, endDate: values.startDate };
          if (creatingClass) {
            createManualClass.mutate(payload);
            return;
          }
          updateClass.mutate(payload);
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
          <Form.Item name="startDate">
            <Input placeholder="上课日期" />
          </Form.Item>
          <Form.Item
            name="studentIds"
            label={`学生（已选 ${editingStudentIDs.length}/${classCapacity(editingClassType)}）`}
          >
            <Select
              mode="multiple"
              showSearch
              optionFilterProp="label"
              maxCount={classCapacity(editingClassType)}
              options={studentOptions}
            />
          </Form.Item>
          <Space.Compact block>
            <Form.Item name="expectedStudentCount" label="预计人数" rules={[{ required: true, message: '请输入预计人数' }]} style={{ width: '35%' }}>
              <InputNumber min={1} max={classCapacity(editingClassType)} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="reservationNote" label="预留说明" style={{ width: '65%' }}>
              <Input maxLength={255} placeholder="例如：待家长确认学生名单" />
            </Form.Item>
          </Space.Compact>
          <Typography.Text type="secondary">可先不绑定学生来锁定时间段，课程将标记为“待确认”；补足最低人数后自动转为“已确认”。</Typography.Text>
        </Form>
      </Drawer>
    </div>
  );
}
