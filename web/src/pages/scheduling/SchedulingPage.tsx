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
  weekdayOfDateText,
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
  RepeatFields,
  buildRepeatPayload,
  type RepeatFormValues,
  type ScheduleRepeatValues,
  pendingReviewColumns,
  scheduleClassPayload,
  type ScheduleMoveTarget,
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
  startTime: string;
  endTime: string;
  /** 这节课的日期；重复排课时是第一节的日期。 */
  startDate: string;
  studentIds: string[];
  expectedStudentCount: number;
  reservationNote?: string;
  repeat?: ScheduleRepeatValues;
  editScope?: string;
  ignoreWarnings?: boolean;
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
  const [editForm] = Form.useForm<ScheduleClassFormValues>();
  // 侧栏（迷你日历 / 学科日历 / 筛选）默认收起：它们是「偶尔用一次」的辅助工具，
  // 常驻展开会占掉 260px，把真正要看的排班网格挤窄一档。收起后留一条竖栏可随时展开，
  // 并在上面标出生效中的筛选条数——否则筛选被藏起来，结果变少却看不出原因。
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [selectedStudentIds, setSelectedStudentIds] = useState<string[]>([]);
  const [selectedCampusId, setSelectedCampusId] = useState(user.campusId || 'campus-main');
  const [editingClass, setEditingClass] = useState<ScheduleClass | null>(null);
  // 拖动重复课次时挂起这次调整，等用户选完影响范围再提交。
  const [moveScopeRequest, setMoveScopeRequest] = useState<{ record: ScheduleClass; target: ScheduleMoveTarget } | null>(null);
  const [moveScope, setMoveScope] = useState<'this' | 'thisAndFuture' | 'all'>('this');
  const [creatingClass, setCreatingClass] = useState(false);
  // 重复排课默认关闭：绝大多数排课是单节，默认展开一堆重复选项只会碍事。
  const [repeatEnabled, setRepeatEnabled] = useState(false);
  const [availabilityOpen, setAvailabilityOpen] = useState(false);
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
  // 排课权限下放：老师也能建课，但落「待审核」，通过后才对学生可见。
  const canCreateClass = user.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role));
  // 审核仍然只归管理员。
  const canReviewClass = user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));

  const teachers = useQuery({ queryKey: ['teachers'], queryFn: () => getData<Teacher[]>('/teachers') });
  const students = useQuery({ queryKey: ['students'], queryFn: () => getData<Student[]>('/students') });
  const courses = useQuery({ queryKey: ['courses'], queryFn: () => getData<Course[]>('/courses') });
  const classes = useQuery({ queryKey: ['schedule-classes'], queryFn: () => getData<ScheduleClass[]>('/schedule-classes') });
  const availabilityOverview = useQuery({ queryKey: ['availability-overview'], queryFn: () => getData<AvailabilitySlot[]>('/availability/overview') });
  // 待审核队列只有管理员看得到，老师侧不发这个请求。
  const pendingClasses = useQuery({
    queryKey: ['schedule-classes-pending'],
    queryFn: () => getData<ScheduleClass[]>('/schedule-classes/pending'),
    enabled: canReviewClass
  });

  const ownerKey = Form.useWatch('ownerKey', availabilityForm);
  const editingClassType = Form.useWatch('classType', editForm);
  const editingStudentIDs = Form.useWatch('studentIds', editForm) ?? [];
  const editingTeacherId = Form.useWatch('teacherId', editForm);
  const editingStartTime = Form.useWatch('startTime', editForm);
  const editingEndTime = Form.useWatch('endTime', editForm);
  const editingStartDate = Form.useWatch('startDate', editForm);
  // 星期不再是独立表单项：课次的日期决定星期，从日期推出来即可。
  const editingDayOfWeek = useMemo(() => weekdayOfDateText(editingStartDate), [editingStartDate]);
  const owner = parseOwnerKey(ownerKey);
  const availability = useQuery({
    queryKey: ['availability', owner?.ownerType, owner?.ownerId],
    enabled: Boolean(owner),
    queryFn: () => getData<AvailabilitySlot[]>('/availability', { ownerType: owner?.ownerType ?? '', ownerId: owner?.ownerId ?? '' })
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
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败，请检查星期和时间段。')
  });

  const cancelClass = useMutation({
    mutationFn: (id: string) => postData<ScheduleClass>(`/schedule-classes/${id}/cancel`, {}),
    onSuccess: () => {
      message.success('课程已取消，可重新生成候选排课');
      setEditingClass(null);
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
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
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '调课失败，请稍后重试。')
  });

  const reviewClass = useMutation({
    mutationFn: ({ id, approve, reason }: { id: string; approve: boolean; reason?: string }) =>
      postData<ScheduleClass>(`/schedule-classes/${id}/${approve ? 'approve' : 'reject'}`, { reason }),
    onSuccess: (record) => {
      message.success(record.auditStatus === '已通过' ? '已通过，学生端已可见' : '已驳回，老师会看到理由');
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
      queryClient.invalidateQueries({ queryKey: ['schedule-classes-pending'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '审核失败，请稍后重试。')
  });

  const moveClass = useMutation({
    mutationFn: ({ record, target, editScope }: { record: ScheduleClass; target: ScheduleMoveTarget; editScope?: string }) =>
      putData<ScheduleClass>(`/schedule-classes/${record.id}`, { ...scheduleClassPayload(record, target), editScope }),
    onSuccess: () => {
      message.success('调课已保存');
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
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
    setRepeatEnabled(false);
    setEditingClass(record);
    editForm.setFieldsValue({
      courseId: record.courseId,
      teacherId: record.teacherId,
      campusId: record.campusId || user.campusId || 'campus-main',
      roomName: record.roomName,
      classType: record.classType,
      durationMinutes: record.durationMinutes,
      startTime: record.startTime,
      endTime: record.endTime,
      startDate: record.lessonDate,
      studentIds: record.students.map((student) => student.id),
      expectedStudentCount: record.expectedStudentCount,
      reservationNote: record.reservationNote
    });
  }

  // 视图里点空白格新建：格子对应的是具体某一天，直接用那天的日期开表单。
  // 星期不再由用户单独选，避免出现「星期三」和「6月4日」互相打架的状态。
  function openCreateClassForDay(lessonDate: string) {
    if (!canCreateClass) return;
    setEditingClass(null);
    setCreatingClass(true);
    setRepeatEnabled(false);
    editForm.setFieldsValue({
      courseId: undefined,
      teacherId: undefined,
      campusId: user.campusId || 'campus-main',
      roomName: '',
      classType: '1V1',
      durationMinutes: 90,
      startTime: '19:00',
      endTime: '20:30',
      startDate: lessonDate,
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
    const isSameTarget = record.lessonDate === target.lessonDate &&
      (!target.startTime || (record.startTime === target.startTime && record.endTime === target.endTime));
    if (!canCreateClass || record.status === '已取消' || isSameTarget) return;

    // 单次课，以及已经单独调整过、不再跟随系列的课次，都只影响它自己，不必问范围。
    const isRepeating = Boolean(record.seriesId) && !record.detached;
    if (!isRepeating) {
      Modal.confirm({
        title: '确认调课',
        content: `将「${record.name}」调整到${target.label}。`,
        okText: '确认调整',
        cancelText: '取消',
        onOk: () => moveClass.mutate({ record, target, editScope: 'this' })
      });
      return;
    }

    // 重复课程必须先问清改哪些课次。以前没有这一步，拖一节课会把整学期一起挪走。
    // 「此课次及后续」和「整个系列」按整体平移处理，已上过的课次不动。
    setMoveScopeRequest({ record, target });
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

      {/* 老师排的课在通过审核前学生端看不到，这里给老师一个明确的预期，
          免得排完以为已经生效了。 */}
      {!canReviewClass && canCreateClass && (
        <Alert
          type="info"
          showIcon
          message="你排的课需要教务确认后，学生端才能看到"
          description="提交后课程进入待审核；被驳回时可以在列表里看到理由并直接修改。"
        />
      )}

      {canReviewClass && (pendingClasses.data ?? []).length > 0 && (
        <Card title={`待审核排课（${(pendingClasses.data ?? []).length}）`}>
          <Table<ScheduleClass>
            rowKey="id"
            size="small"
            pagination={false}
            dataSource={pendingClasses.data ?? []}
            loading={pendingClasses.isFetching}
            columns={pendingReviewColumns(
              courseById,
              (record) => reviewClass.mutate({ id: record.id, approve: true }),
              (record, reason) => reviewClass.mutate({ id: record.id, approve: false, reason }),
              reviewClass.isPending
            )}
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
                  month={calendarMonth}
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
            message="该时段超出已登记的可上课时间"
            description={(
              <Space direction="vertical" size={4} style={{ width: '100%' }}>
                <Typography.Text type="secondary">
                  下列老师/学生在 {weekLabel(editingDayOfWeek)} {editingStartTime}-{editingEndTime} 没有登记可上课时间。
                  可上课时间只是参考范围——如果已经线下约好，勾选下方确认后可以照排，系统会记录这次越界。
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
          // 一条记录就是一节课，日期只收 startDate；重复排课由 repeat 展开成多节。
          // ignoreWarnings：越出可上课时间只是软提醒，用户看到上面的提示后仍可继续，
          // 后端会把这次越界记进 overrideNote。
          const { repeat, ...rest } = values as ScheduleClassFormValues & { repeat?: RepeatFormValues };
          const payload = { ...rest, ignoreWarnings: availabilityGaps.length > 0 };
          if (creatingClass) {
            // 开关关着时不带 repeat 字段，后端据此判定只排一节。
            createManualClass.mutate(repeatEnabled && repeat ? { ...payload, repeat: buildRepeatPayload(repeat) } : payload);
            return;
          }
          // 改重复课次时要带上影响范围；这里是抽屉里的逐项修改，只作用于这一节。
          updateClass.mutate({ ...payload, editScope: 'this' });
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
          </Space.Compact>
          <Space.Compact block>
            <Form.Item name="startTime" rules={[{ required: true, message: '请输入开始时间' }]} style={{ width: '50%' }}>
              <Input placeholder="19:00" />
            </Form.Item>
            <Form.Item name="endTime" rules={[{ required: true, message: '请输入结束时间' }]} style={{ width: '50%' }}>
              <Input placeholder="20:30" />
            </Form.Item>
          </Space.Compact>
          <Form.Item name="startDate" label={creatingClass && repeatEnabled ? '首节上课日期' : '上课日期'} rules={[{ required: true, message: '请选择上课日期' }]}>
            <Input placeholder="2026-06-03" />
          </Form.Item>

          {/* 重复规则只在新建时出现：一节已经排好的课谈不上「重复几次」，
              要改重复方式就是删了重排。 */}
          {creatingClass && (
            <RepeatFields
              enabled={repeatEnabled}
              onToggle={setRepeatEnabled}
              startDate={editingStartDate}
              form={editForm}
            />
          )}
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

      {/* 拖动重复课次时先问清影响范围。以前没有这一步，
          拖一节课会把整个学期的课一起挪走，而且没有任何提示。 */}
      <Modal
        title="调整重复课程"
        open={Boolean(moveScopeRequest)}
        okText="确认调整"
        cancelText="取消"
        confirmLoading={moveClass.isPending}
        onCancel={() => setMoveScopeRequest(null)}
        onOk={() => {
          if (!moveScopeRequest) return;
          moveClass.mutate(
            { record: moveScopeRequest.record, target: moveScopeRequest.target, editScope: moveScope },
            { onSettled: () => setMoveScopeRequest(null) }
          );
        }}
        afterClose={() => setMoveScope('this')}
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Typography.Text>
            将「{moveScopeRequest?.record.name}」调整到{moveScopeRequest?.target.label}。这是一门重复课程，请选择本次调整影响哪些课次：
          </Typography.Text>
          <Select
            value={moveScope}
            onChange={setMoveScope}
            style={{ width: '100%' }}
            options={[
              { label: '仅此课次', value: 'this' },
              { label: '此课次及后续', value: 'thisAndFuture' },
              { label: '整个系列', value: 'all' }
            ]}
          />
          <Typography.Text type="secondary">
            {moveScope === 'this'
              ? '只改这一节，这节课此后不再跟随系列的批量调整。'
              : '按相同的天数整体平移，已经上过的课次不会被改动。'}
          </Typography.Text>
        </Space>
      </Modal>
    </div>
  );
}
