import { useListState } from '../../hooks/useListState';
import { subjectPalette } from '../../utils/subject-colors';
import { DeleteOutlined, LeftOutlined, PlusOutlined, ReloadOutlined, RightOutlined, SaveOutlined } from '@ant-design/icons';
import { Alert, Button, Drawer, Form, Input, InputNumber, Modal, Segmented, Select, Skeleton, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { getData, postData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { TeacherScopeFields } from '../../components/TeacherScopeFields';
import { serializeTeacherScopes, teacherScopeFormValues } from '../../utils/teacherScopes';
import { coursesForTeacher, filterTeachingCourses, schedulingStudentOptions } from '../../utils/schedulingScopes';
import { gradeOptions, subjectLabel, subjectOptions, useSubjectCatalog } from '../../utils/curriculum';
import type { AvailabilitySlot, Course, CurrentUser, LearningSpace, ScheduleClass, Student, Teacher, TeacherUpsertRequest } from '../../types/starline';
import { addDays, addMonths, weekdayOfDateText, classCapacity, formatWeekRange, isClockText, localDateText, scheduleClassOccursOn, startOfMonth, startOfWeek, weekOptions } from './scheduling-utils';
import { CalendarTimeline, SchedulingAssistant, type CalendarMode, type CalendarPerson, type CalendarSelection } from './CalendarWorkbench';
import { MiniMonthCalendar, MonthScheduleBoard, buildMiniMonthDays, ScheduleLessonList, filterClasses, parseOwnerKey, RepeatFields, buildRepeatPayload, type RepeatFormValues, type ScheduleRepeatValues, pendingReviewColumns, scheduleClassPayload, type ScheduleMoveTarget, scheduleClassSubject, studentDisplayName, teacherOptionLabel, uniqueScheduleCampuses } from './SchedulingViews';

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

const classTypeOptions = ['1V1', '1V2', '1V3', '1V4'].map((value) => ({ label: value, value }));
export default function Scheduling({ user }: { user: CurrentUser }) {
  const [availabilityForm] = Form.useForm<AvailabilityFormValues>();
  const [editForm] = Form.useForm<ScheduleClassFormValues>();
  const [teacherScopeForm] = Form.useForm();
  const [scopeTeacher, setScopeTeacher] = useState<Teacher | null>(null);
  const [formCourseGrade, setFormCourseGrade] = useState<string>();
  const [formCourseSubject, setFormCourseSubject] = useState<string>();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [selectedLessonId, setSelectedLessonId] = useState<string>();
  const [calendarPeople, setCalendarPeople] = useListState<string[]>('scheduling:calendarPeople', []);
  const [cancelRecord, setCancelRecord] = useState<ScheduleClass | null>(null);
  const [cancelScope, setCancelScope] = useState('this');
  const [editScope, setEditScope] = useState('this');
  const [preflightBusy, setPreflightBusy] = useState(false);
  const [assistantOpen, setAssistantOpen] = useState(false);
  const [editingClass, setEditingClass] = useState<ScheduleClass | null>(null);
  // 拖动重复课次时挂起这次调整，等用户选完影响范围再提交。
  const [moveScopeRequest, setMoveScopeRequest] = useState<{ record: ScheduleClass; target: ScheduleMoveTarget } | null>(null);
  const [moveScope, setMoveScope] = useState<'this' | 'thisAndFuture' | 'all'>('this');
  const [creatingClass, setCreatingClass] = useState(false);
  const [copyingClass, setCopyingClass] = useState(false);
  // 重复排课默认关闭：绝大多数排课是单节，默认展开一堆重复选项只会碍事。
  const [repeatEnabled, setRepeatEnabled] = useState(false);
  const [availabilityOpen, setAvailabilityOpen] = useState(false);
  // 首次进入展示月历，后续沿用个人选择。
  const [viewMode, setViewMode] = useState<CalendarMode>(() => {
    try { const saved = localStorage.getItem(`starline-calendar-view:${user.userId}`); if (saved === 'workweek') return 'week'; return ['day','week','month','list'].includes(saved ?? '') ? saved as CalendarMode : 'month'; } catch { return 'month'; }
  });
  useEffect(() => { try { localStorage.setItem(`starline-calendar-view:${user.userId}`, viewMode); } catch { /* storage unavailable */ } }, [viewMode, user.userId]);
  const [classGradeFilter, setClassGradeFilter] = useListState<string>('scheduling:classGradeFilter');
  const [classSubjectFilter, setClassSubjectFilter] = useListState<string>('scheduling:classSubjectFilter');
  const [classTeacherFilter, setClassTeacherFilter] = useListState<string>('scheduling:classTeacherFilter');
  const [classStudentFilter, setClassStudentFilter] = useListState<string>('scheduling:classStudentFilter');
  const [classCampusFilter, setClassCampusFilter] = useListState<string>('scheduling:classCampusFilter');
  const [classCourseFilter, setClassCourseFilter] = useListState<string>('scheduling:classCourseFilter');
  const [classTypeFilter, setClassTypeFilter] = useListState<string>('scheduling:classTypeFilter');
  const [statusFilter, setStatusFilter] = useListState<string>('scheduling:statusFilter', '全部');
  const [selectedWeekStart, setSelectedWeekStart] = useListState<Date>('scheduling:selectedWeekStart', () => startOfWeek(new Date()));
  // 日视图选中的那一天。始终保持在 selectedWeekStart 所在周内，
  // 这样日/周视图共用同一份按 dayOfWeek 分组的数据，切换视图不用重新取数。
  const [selectedDate, setSelectedDate] = useListState<Date>('scheduling:selectedDate', () => new Date());
  const [calendarMonth, setCalendarMonth] = useListState<Date>('scheduling:calendarMonth', () => startOfMonth(new Date()));
  const [hiddenSubjects, setHiddenSubjects] = useListState<string[]>('scheduling:hiddenSubjects', []);
  const queryClient = useQueryClient();
  // 排课权限下放：老师也能建课，但落「待审核」，通过后才对学生可见。
  const canCreateClass = user.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role));
  // 审核仍然只归管理员。
  const canReviewClass = user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));

  const canAdjustLesson = (record: ScheduleClass) => canCreateClass && record.status !== '已取消' && record.status !== '已上课'
    && (canReviewClass || record.teacherId === user.userId && ['待审核', '已驳回'].includes(record.auditStatus));
  const detailReadOnly = !canCreateClass || Boolean(editingClass && !copyingClass && !canAdjustLesson(editingClass));

  const teachers = useQuery({ queryKey: ['teachers'], queryFn: () => getData<Teacher[]>('/teachers') });
  const students = useQuery({ queryKey: ['students'], queryFn: () => getData<Student[]>('/students') });
  const courses = useQuery({ queryKey: ['courses'], queryFn: () => getData<Course[]>('/courses') });
  const teachingSpaces = useQuery({ queryKey: ['learning-spaces'], queryFn: () => getData<LearningSpace[]>('/learning-spaces'), enabled: Boolean(scopeTeacher) });
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
  const editingCourseId = Form.useWatch('courseId', editForm);
  const editingStartTime = Form.useWatch('startTime', editForm);
  const editingEndTime = Form.useWatch('endTime', editForm);
  const editingStartDate = Form.useWatch('startDate', editForm);
  // 星期不再是独立表单项：课次的日期决定星期，从日期推出来即可。
  const watchedValues = Form.useWatch([], editForm) as ScheduleClassFormValues & { repeat?: RepeatFormValues } | undefined;
  const previewPayloadText = JSON.stringify(watchedValues ? { ...watchedValues, editScope, id: creatingClass ? undefined : editingClass?.id,
    repeat: creatingClass && repeatEnabled && watchedValues.repeat ? buildRepeatPayload(watchedValues.repeat) : undefined } : null);
  const [debouncedPreview, setDebouncedPreview] = useState(previewPayloadText);
  useEffect(() => { const timer = setTimeout(() => setDebouncedPreview(previewPayloadText), 300); return () => clearTimeout(timer); }, [previewPayloadText]);
  const previewPayload = JSON.parse(debouncedPreview) as (ScheduleClassFormValues & { id?: string }) | null;
  const livePreview = useQuery({ queryKey: ['schedule-preview', debouncedPreview], queryFn: () => postData<SchedulePreview>('/schedule-classes/preview', previewPayload),
    enabled: Boolean((creatingClass || editingClass) && previewPayload?.teacherId && previewPayload?.courseId && weekdayOfDateText(previewPayload?.startDate) && isClockText(previewPayload?.startTime) && isClockText(previewPayload?.endTime)), retry: false });
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

  const completeClass = useMutation({
    mutationFn: (id: string) => postData<ScheduleClass>(`/schedule-classes/${id}/completed`, {}),
    onSuccess: (record) => { message.success('已标记已上课'); setEditingClass(record); queryClient.invalidateQueries({ queryKey: ['schedule-classes'] }); },
    onError: (e: Error) => message.error(e.message)
  });

  const cancelClass = useMutation({
    mutationFn: ({ id, editScope }: { id: string; editScope: string }) => postData<ScheduleClass>(`/schedule-classes/${id}/cancel`, { editScope }),
    onSuccess: () => {
      message.success('课程已取消，该时间可重新排课');
      setCancelRecord(null);
      setEditingClass(null);
      queryClient.invalidateQueries({ queryKey: ['schedule-classes'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '取消课程失败，请稍后重试。')
  });

  const createManualClass = useMutation({
    mutationFn: (values: ScheduleClassFormValues) => postData<ScheduleClass>('/schedule-classes', values),
    onSuccess: (record) => {
      if (copyingClass) {
        message.success(record.auditStatus === '待审核' ? '复制课程已提交审核' : '复制课程已创建，课表已更新');
      } else if (record.auditStatus === '待审核') {
        message.success('课程已提交审核，管理员通过后学生端可见');
      } else {
        message.success(record.status === '待确认' ? '时间段已锁定，后续可补充学生' : '课程已创建，课表已更新');
      }
      setCreatingClass(false);
      setCopyingClass(false);
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
      let successText = '调课已保存';
      if (record.auditStatus === '待审核') successText = '修改已提交审核，管理员通过后学生端可见';
      else if (record.status === '待确认') successText = '调课已保存，当前课程待确认';
      message.success(successText);
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
      putData<ScheduleClass>(`/schedule-classes/${record.id}`, { ...scheduleClassPayload(record, target), editScope, ignoreWarnings: true }),
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

  const courseOptions = (courses.data ?? []).map((item) => ({ label: `${item.name} · ${item.grade}/${subjectLabel(item.subject)}`, value: item.id }));
  const teacherOptions = (teachers.data ?? []).filter(item => item.accountStatus !== '停用' || item.id === editingClass?.teacherId).map((item) => ({ label: teacherOptionLabel(item), value: item.id, disabled: item.accountStatus === '停用' }));
  const studentOptions = schedulingStudentOptions(students.data ?? []);
  const formTeacher = (teachers.data ?? []).find(item => item.id === editingTeacherId);
  const teachingCourses = coursesForTeacher(courses.data ?? [], formTeacher);
  const candidateCourses = filterTeachingCourses(teachingCourses, formCourseGrade, formCourseSubject);
  const formCourseOptions = candidateCourses.map(item => ({ value: item.id, label: `${item.name} · ${item.grade}/${subjectLabel(item.subject)}` }));
  const selectedFormCourse = (courses.data ?? []).find(item => item.id === editingCourseId);
  if (selectedFormCourse && !candidateCourses.some(item => item.id === selectedFormCourse.id)) formCourseOptions.push({ value: selectedFormCourse.id, label: `${selectedFormCourse.name} · ${teachingCourses.some(item => item.id === selectedFormCourse.id) ? '当前筛选之外' : '不在授课范围'}` });
  const validTeacherCourse = !editingCourseId || teachingCourses.some(item => item.id === editingCourseId) || !creatingClass && editingClass?.teacherId === editingTeacherId && editingClass?.courseId === editingCourseId;
  const saveTeacherScope = useMutation({
    mutationFn: (values: any) => {
      if (!scopeTeacher) throw new Error('请选择老师');
      const base = teacherScopeFormValues(scopeTeacher, teachingSpaces.data ?? []);
      const body: TeacherUpsertRequest = { name: scopeTeacher.name, phone: scopeTeacher.phone, campusId: scopeTeacher.campusId, accountStatus: scopeTeacher.accountStatus, remark: scopeTeacher.remark, canUploadHandout: scopeTeacher.canUploadHandout, canUploadQuestion: scopeTeacher.canUploadQuestion, canReview: scopeTeacher.canReview,
        ...serializeTeacherScopes({ ...values, teacherLibrary: { ...base.teacherLibrary, ...values.teacherLibrary } }, teachingSpaces.data ?? []) };
      return putData<Teacher>(`/teachers/${scopeTeacher.id}`, body);
    },
    onSuccess: () => { message.success('授课范围已更新'); setScopeTeacher(null); queryClient.invalidateQueries({ queryKey: ['teachers'] }); setFormCourseGrade(undefined); setFormCourseSubject(undefined); },
    onError: (error: Error) => message.error(error.message || '授课范围保存失败')
  });
  useEffect(() => { if (scopeTeacher && teachingSpaces.data) teacherScopeForm.setFieldsValue(teacherScopeFormValues(scopeTeacher, teachingSpaces.data)); }, [scopeTeacher, teachingSpaces.data, teacherScopeForm]);
  const campusOptions = uniqueScheduleCampuses(classes.data ?? []).map((value) => ({ label: value, value }));
  const subjectCatalog = useSubjectCatalog();
  const classSubjectOptions = subjectOptions(classGradeFilter, subjectCatalog);
  const courseById = useMemo(() => Object.fromEntries((courses.data ?? []).map((item) => [item.id, item])), [courses.data]);
  const teacherById = useMemo(() => Object.fromEntries((teachers.data ?? []).map((item) => [item.id, item])), [teachers.data]);
  const studentById = useMemo(() => Object.fromEntries((students.data ?? []).map((item) => [item.id, item])), [students.data]);
  const statusOptions = [
    { label: '全部状态', value: '全部' },
    { label: '待成班', value: '待确认' },
    { label: '待上课', value: '已确认' },
    { label: '待审核', value: '待审核' },
    { label: '已驳回', value: '已驳回' },
    { label: '已上课', value: '已上课' },
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
  const allPeople: CalendarPerson[] = [
    ...(teachers.data ?? []).map(item => ({ key: `teacher:${item.id}`, id: item.id, name: item.name, kind: 'teacher' as const })),
    ...(students.data ?? []).map(item => ({ key: `student:${item.id}`, id: item.id, name: studentDisplayName(item), kind: 'student' as const }))
  ];
  const selectedPeople = allPeople.filter(person => calendarPeople.includes(person.key));
  const dayPeople = calendarPeople.length ? selectedPeople : allPeople.filter(person => person.kind === 'teacher' && (!classTeacherFilter || person.id === classTeacherFilter));
  const filteredClasses = useMemo(() => filterClasses(classes.data ?? [], classFilters, courseById).filter(item => !calendarPeople.length || calendarPeople.some(key => key === `teacher:${item.teacherId}` || item.students.some(student => key === `student:${student.id}`))), [classes.data, classFilters, courseById, calendarPeople]);
  const subjectVisibleClasses = useMemo(
    () => filteredClasses.filter((item) => !hiddenSubjects.includes(scheduleClassSubject(item, courseById) || '其他')),
    [filteredClasses, hiddenSubjects, courseById]
  );
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
  const hasClassFilters = Boolean(classGradeFilter || classSubjectFilter || classTeacherFilter || classStudentFilter || classCampusFilter || classCourseFilter || classTypeFilter || hiddenSubjects.length > 0 || statusFilter !== '全部');

  useEffect(() => {
    if (ownerOptions.length > 0 && !availabilityForm.getFieldValue('ownerKey')) {
      availabilityForm.setFieldValue('ownerKey', ownerOptions[0].value);
    }
  }, [availabilityForm, ownerOptions]);

  useEffect(() => {
    if (availability.data) availabilityForm.setFieldValue('slots', availability.data);
  }, [availability.data, availabilityForm]);

  useEffect(() => {
    if (classSubjectFilter && !subjectOptions(classGradeFilter, subjectCatalog).some((item) => item.value === classSubjectFilter)) {
      setClassSubjectFilter(undefined);
    }
  }, [classGradeFilter, classSubjectFilter, subjectCatalog]);

  const editingPeople = allPeople.filter(person => person.kind === 'teacher' ? person.id === editingTeacherId : editingStudentIDs.includes(person.id));
  function requestCancel(id: string) {
    const record = (classes.data ?? []).find(item => item.id === id);
    if (record) { setCancelScope('this'); setCancelRecord(record); }
  }
  const restoreClass = useMutation({
    mutationFn: ({ id, ignoreWarnings }: { id: string; ignoreWarnings: boolean }) => postData<ScheduleClass>(`/schedule-classes/${id}/restore`, { ignoreWarnings }),
    onSuccess: () => { message.success('课次已恢复'); queryClient.invalidateQueries({ queryKey: ['schedule-classes'] }); queryClient.invalidateQueries({ queryKey: ['schedule-classes-pending'] }); },
    onError: (e: Error) => message.error(e.message || '恢复失败，请重新检查')
  });
  async function requestRestore(record: ScheduleClass) {
    try {
      const result = await postData<SchedulePreview>(`/schedule-classes/${record.id}/restore-preview`, {});
      if (!result.canSave) { Modal.error({ title: '无法恢复课程', width: 640, content: <PreviewResult result={result} /> }); return; }
      const warnings = result.lessons.some(lesson => lesson.warnings.length > 0);
      Modal.confirm({ title: '恢复这节课程', width: 640, content: <PreviewResult result={result} />, okText: warnings ? '已协调，恢复课程' : '恢复课程', cancelText: '保留取消', onOk: () => restoreClass.mutateAsync({ id: record.id, ignoreWarnings: warnings }) });
    } catch (e) { message.error(e instanceof Error ? e.message : '恢复预检失败，请重试'); }
  }

  async function preflight(payload: ScheduleClassFormValues, id: string | undefined, submit: () => void) {
    setPreflightBusy(true);
    try {
      const result = await postData<SchedulePreview>('/schedule-classes/preview', { ...payload, id });
      if (!result.canSave) { Modal.error({ title: '无法排课', width: 640, content: <PreviewResult result={result} /> }); return; }
      if (result.lessons.some(lesson => lesson.warnings.length)) {
        Modal.confirm({ title: '确认超出可上课时间', width: 640, content: <PreviewResult result={result} />, okText: '已协调，继续排课', cancelText: '返回调整', onOk: submit });
      } else submit();
    } catch (error) { message.error(error instanceof Error ? error.message : '排课预检失败，请重试'); }
    finally { setPreflightBusy(false); }
  }
  function openEdit(record: ScheduleClass) {
    setSelectedLessonId(record.id);
    setEditScope('this');
    setAssistantOpen(false);
    setCreatingClass(false);
    setCopyingClass(false);
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

  // 右键复制：把这节课的课程、老师、学生、班型、时段原样带进新建表单，
  // 用户通常只需要改日期或时间就能再排一节。重复录入一模一样的信息
  // 是排课里最费时的部分，也是客户点名要的「快捷排课」。
  //
  // 复制出来的是一节全新的单次课：不带 seriesId，也不继承审核状态——
  // 老师复制出来的课同样要走审核，否则复制就成了绕过审核的后门。
  function openCopy(record: ScheduleClass) {
    if (!canCreateClass) return;
    setEditingClass(null);
    setFormCourseGrade(undefined);
    setFormCourseSubject(undefined);
    setCreatingClass(true);
    setCopyingClass(true);
    setRepeatEnabled(false);
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
    message.info('已复制课程内容，改好时间后提交即可');
  }

  // 拖块下沿改时长。走的是同一条调课链路，所以重复课次同样要问影响范围——
  // 少了这一步，拉伸就成了绕过三选一的后门，又变回「改一节动整个学期」。
  function confirmResizeClass(record: ScheduleClass, endTime: string) {
    if (!canAdjustLesson(record) || endTime === record.endTime) return;
    const target: ScheduleMoveTarget = {
      lessonDate: record.lessonDate,
      startTime: record.startTime,
      endTime,
      label: `${record.lessonDate} ${record.startTime}-${endTime}`
    };
    const isRepeating = Boolean(record.seriesId) && !record.detached;
    if (!isRepeating) {
      void preflight({ ...scheduleClassPayload(record, target), editScope: 'this' }, record.id, () => moveClass.mutate({ record, target, editScope: 'this' }));
      return;
    }
    setMoveScopeRequest({ record, target });
  }

  // 视图里点空白格新建：格子对应的是具体某一天，直接用那天的日期开表单。
  // 星期不再由用户单独选，避免出现「星期三」和「6月4日」互相打架的状态。
  function openCreateClassForDay(lessonDate: string, selection: CalendarSelection = {}) {
    if (!canCreateClass) return;
    setEditingClass(null);
    setFormCourseGrade(undefined);
    setFormCourseSubject(undefined);
    setCreatingClass(true);
    setCopyingClass(false);
    setRepeatEnabled(false);
    setEditScope('this');
    setAssistantOpen(false);
    const chosen = parseOwnerKey(selection.ownerKey);
    const selectedTeachers = selectedPeople.filter(person => person.kind === 'teacher');
    const selectedStudents = selectedPeople.filter(person => person.kind === 'student');
    const studentIds = chosen?.ownerType === 'student' ? [chosen.ownerId] : selectedStudents.slice(0, 4).map(person => person.id);
    const startTime = selection.startTime ?? '19:00';
    const endTime = selection.endTime ?? '20:30';
    const duration = clockMinutes(endTime) - clockMinutes(startTime);
    editForm.setFieldsValue({
      courseId: undefined,
      teacherId: chosen?.ownerType === 'teacher' ? chosen.ownerId : selectedTeachers.length === 1 ? selectedTeachers[0].id : undefined,
      campusId: user.campusId || 'campus-main',
      roomName: '',
      classType: `1V${Math.max(1, studentIds.length)}`,
      durationMinutes: duration,
      startTime,
      endTime,
      startDate: lessonDate,
      studentIds,
      expectedStudentCount: Math.max(1, studentIds.length),
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

  function navigateMonth(offset: number) {
    const month = addMonths(calendarMonth, offset);
    const lastDay = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
    goToDate(new Date(month.getFullYear(), month.getMonth(), Math.min(selectedDate.getDate(), lastDay)));
  }

  function confirmMoveClass(record: ScheduleClass, target: ScheduleMoveTarget) {
    const isSameTarget = record.lessonDate === target.lessonDate &&
      (!target.startTime || (record.startTime === target.startTime && record.endTime === target.endTime));
    if (!canAdjustLesson(record) || isSameTarget) return;

    // 单次课，以及已经单独调整过、不再跟随系列的课次，都只影响它自己，不必问范围。
    const isRepeating = Boolean(record.seriesId) && !record.detached;
    if (!isRepeating) {
      Modal.confirm({
        title: '确认调课',
        content: `将「${record.name}」调整到${target.label}。`,
        okText: '确认调整',
        cancelText: '取消',
        onOk: () => preflight({ ...scheduleClassPayload(record, target), editScope: 'this' }, record.id, () => moveClass.mutate({ record, target, editScope: 'this' }))
      });
      return;
    }

    // 重复课程必须先问清改哪些课次。以前没有这一步，拖一节课会把整学期一起挪走。
    // 「此课次及后续」和「整个系列」按整体平移处理，已上过的课次不动。
    setMoveScopeRequest({ record, target });
  }

  if (teachers.isLoading || students.isLoading || courses.isLoading || classes.isLoading || availabilityOverview.isLoading) return <Skeleton active />;
  if (teachers.error || students.error || courses.error || classes.error || availabilityOverview.error) return <Alert type="error" message="排课数据加载失败" action={<Button onClick={() => { void teachers.refetch(); void students.refetch(); void courses.refetch(); void classes.refetch(); void availabilityOverview.refetch(); }}>重试</Button>} />;

  return (
    <div className="page-stack calendar-page">
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

      <div className="calendar-workbench">
        <div className="calendar-toolbar">
          <Space wrap size={6}>
            <Button onClick={() => setSidebarOpen(open => !open)}>{sidebarOpen ? '收起侧栏' : '人员日历'}</Button>
            <Button icon={<LeftOutlined />} aria-label="上一期" onClick={() => viewMode === 'month' ? navigateMonth(-1) : goToDate(addDays(selectedDate, viewMode === 'day' ? -1 : -7))} />
            <Button onClick={() => goToDate(new Date())}>今天</Button>
            <Button icon={<RightOutlined />} aria-label="下一期" onClick={() => viewMode === 'month' ? navigateMonth(1) : goToDate(addDays(selectedDate, viewMode === 'day' ? 1 : 7))} />
            <strong>{viewMode === 'day' ? localDateText(selectedDate) : viewMode === 'list' ? '全部课次' : viewMode === 'month' ? `${calendarMonth.getFullYear()} 年 ${calendarMonth.getMonth() + 1} 月` : formatWeekRange(selectedWeekStart)}</strong>
          </Space>
          <Space wrap><Segmented value={viewMode} onChange={value => setViewMode(value as CalendarMode)} options={[
            { label: '月', value: 'month' }, { label: '周', value: 'week' }, { label: '日', value: 'day' }, { label: '列表', value: 'list' }
          ]} />{canCreateClass && <Button aria-label="新建课程" type="primary" icon={<PlusOutlined />} onClick={() => openCreateClassForDay(localDateText(selectedDate))}>新建课程</Button>}</Space>
        </div>
        <div className="calendar-filterbar">
          <Space wrap size={8}>
            <Select aria-label="教师筛选" allowClear showSearch optionFilterProp="label" placeholder="全部教师" options={teacherOptions} value={classTeacherFilter} onChange={setClassTeacherFilter} />
            <Select aria-label="学生筛选" allowClear showSearch optionFilterProp="label" placeholder="全部学生" options={studentOptions} value={classStudentFilter} onChange={setClassStudentFilter} />
            <Select aria-label="年级筛选" allowClear placeholder="全部年级" options={gradeOptions()} value={classGradeFilter} onChange={setClassGradeFilter} />
            <Select aria-label="学科筛选" allowClear placeholder="全部学科" options={classSubjectOptions} value={classSubjectFilter} onChange={setClassSubjectFilter} />
            <Select aria-label="状态筛选" options={statusOptions} value={statusFilter} onChange={setStatusFilter} />
            <Button onClick={() => setFiltersOpen(open => !open)}>更多筛选</Button>
            {hasClassFilters && <Button type="text" onClick={() => { setClassGradeFilter(undefined); setClassSubjectFilter(undefined); setClassCampusFilter(undefined); setClassCourseFilter(undefined); setClassTypeFilter(undefined); setClassTeacherFilter(undefined); setClassStudentFilter(undefined); setHiddenSubjects([]); setStatusFilter('全部'); }}>清空筛选</Button>}
          </Space>
          <Space wrap>{canReviewClass && <Button className="calendar-review-button" onClick={() => setReviewOpen(true)}>待审核 {pendingClasses.data?.length ?? 0}</Button>}<Button icon={<SaveOutlined />} onClick={() => setAvailabilityOpen(true)}>教师可上课时间</Button><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => queryClient.invalidateQueries()} /></Space>
        </div>
        {filtersOpen && <div className="calendar-extra-filters"><Space wrap>
          <Select aria-label="校区筛选" allowClear placeholder="全部校区" options={campusOptions} value={classCampusFilter} onChange={setClassCampusFilter} />
          <Select aria-label="课程筛选" allowClear showSearch optionFilterProp="label" placeholder="全部课程" options={courseOptions} value={classCourseFilter} onChange={setClassCourseFilter} />
          <Select aria-label="班型筛选" allowClear placeholder="全部班型" options={classTypeOptions} value={classTypeFilter} onChange={setClassTypeFilter} />
        </Space></div>}
        <div className={sidebarOpen ? 'schedule-outlook-shell' : 'calendar-shell-collapsed'}>
          {sidebarOpen && <aside className="schedule-outlook-sidebar">
            <div className="schedule-sidebar-section">
              <div className="schedule-sidebar-head"><strong>{calendarMonth.getFullYear()} 年 {calendarMonth.getMonth() + 1} 月</strong><Space size={4}>
                <ActionButton tooltip="上个月" icon={<LeftOutlined />} onClick={() => setCalendarMonth(addMonths(calendarMonth, -1))} />
                <ActionButton tooltip="下个月" icon={<RightOutlined />} onClick={() => setCalendarMonth(addMonths(calendarMonth, 1))} />
              </Space></div>
              <MiniMonthCalendar month={calendarMonth} selectedWeekStart={selectedWeekStart} selectedDate={selectedDate} highlight={viewMode === 'day' ? 'day' : 'week'} classCountByDate={classCountByDate} onPickDate={goToDate} />
            </div>
            <div className="schedule-sidebar-section calendar-person-picker">
              <strong>人员日历</strong>
              <Select aria-label="选择人员日历" mode="multiple" allowClear showSearch optionFilterProp="label" placeholder="全部教师" value={calendarPeople} onChange={setCalendarPeople}
                options={allPeople.map(person => ({ value: person.key, label: `${person.kind === 'teacher' ? '教师' : '学生'} · ${person.name}` }))} />
            </div>
          </aside>}
          <main className="schedule-outlook-main">
            <div className="calendar-context">
              <Space wrap size={10}>{Array.from(new Set((courses.data ?? []).map(course => course.subject))).map(subject => <span className="calendar-subject-legend" key={subject}><i style={{ background: subjectPalette(subject).accent }} />{subjectLabel(subject)}</span>)}{hasClassFilters && <Tag color="blue">已筛选</Tag>}{selectedPeople.map(person => <Tag key={person.key}>{person.name}</Tag>)}</Space>
              {viewMode !== 'month' && viewMode !== 'list' && <Space wrap size={10}><span className="calendar-legend-teacher">教师可上课</span><span className="calendar-legend-student">学生可上课</span><span title="空白不表示已登记可上课时间">双击新建 · 拖选时段</span></Space>}
            </div>
            {viewMode === 'day' || viewMode === 'week' ? <CalendarTimeline
              mode={viewMode} date={selectedDate} people={viewMode === 'day' ? dayPeople : selectedPeople} lessons={subjectVisibleClasses}
              slots={availabilityOverview.data ?? []} courseById={courseById} teacherById={teacherById} studentById={studentById} canManage={canCreateClass} canAdjustClass={canAdjustLesson}
              onCreate={openCreateClassForDay} onEdit={openEdit} onCopy={openCopy} onMove={confirmMoveClass} onResize={confirmResizeClass}
            /> : viewMode === 'month' ? <MonthScheduleBoard month={calendarMonth} classes={subjectVisibleClasses} courseById={courseById} teacherById={teacherById} canManage={canCreateClass} canAdjustClass={canAdjustLesson} onEditClass={openEdit} onCopyClass={openCopy} onMoveClass={confirmMoveClass} onCreate={openCreateClassForDay} selectedDate={selectedDate} onSelectDate={goToDate} onOpenDay={date => { goToDate(date); setViewMode('day'); }} selectedLessonId={selectedLessonId} onCancel={record => requestCancel(record.id)} onComplete={record => completeClass.mutate(record.id)} />
              : <ScheduleLessonList classes={subjectVisibleClasses} courseById={courseById} teacherById={teacherById} canManage={canCreateClass} canAdjustClass={canAdjustLesson} onEditClass={openEdit} onCancel={record => requestCancel(record.id)} onRestore={requestRestore} cancelling={cancelClass.isPending} />}
          </main>
        </div>
      </div>

      <Drawer title={`待审核排课（${pendingClasses.data?.length ?? 0}）`} open={reviewOpen && canReviewClass} onClose={() => setReviewOpen(false)} width="min(1000px, 100vw)">
        {pendingClasses.error ? <Alert type="error" message="审核数据加载失败" action={<Button onClick={() => pendingClasses.refetch()}>重试</Button>} /> : <Table<ScheduleClass> rowKey="id" size="small" pagination={{ pageSize: 10 }} scroll={{ x: 760 }} dataSource={pendingClasses.data ?? []} loading={pendingClasses.isFetching} columns={pendingReviewColumns(courseById, record => reviewClass.mutate({ id: record.id, approve: true }), (record, reason) => reviewClass.mutate({ id: record.id, approve: false, reason }), reviewClass.isPending)} />}
      </Drawer>
      <Drawer title="编辑老师授课范围" width="min(560px, 100vw)" open={!!scopeTeacher} onClose={() => setScopeTeacher(null)} destroyOnHidden extra={<Button type="primary" loading={saveTeacherScope.isPending} disabled={!teachingSpaces.data} onClick={() => teacherScopeForm.submit()}>保存范围</Button>}>
        {teachingSpaces.isLoading ? <Skeleton active /> : teachingSpaces.error ? <Alert type="error" message="授课范围加载失败" action={<Button onClick={() => teachingSpaces.refetch()}>重试</Button>} /> : <Form form={teacherScopeForm} layout="vertical" onFinish={values => saveTeacherScope.mutate(values)}><Typography.Paragraph strong>{scopeTeacher?.name}</Typography.Paragraph><TeacherScopeFields form={teacherScopeForm} spaces={teachingSpaces.data ?? []} /></Form>}
      </Drawer>
      <Drawer
        title="教师可上课时间"
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
                      <Space wrap className="availability-date-fields">
                        <Form.Item name={[field.name, 'startDate']}><Input type="date" aria-label="生效日期" onChange={event => { if (availabilityForm.getFieldValue(['slots', field.name, 'unavailable']) && weekdayOfDateText(event.target.value)) availabilityForm.setFieldValue(['slots', field.name, 'dayOfWeek'], weekdayOfDateText(event.target.value)); }} /></Form.Item>
                        <Form.Item name={[field.name, 'endDate']}><Input type="date" aria-label="结束日期" /></Form.Item>
                        <Form.Item name={[field.name, 'unavailable']} valuePropName="checked"><Switch checkedChildren="不可上课" unCheckedChildren="可上课" onChange={checked => {
                          if (!checked) return;
                          const date = availabilityForm.getFieldValue(['slots', field.name, 'startDate']) || localDateText(selectedDate);
                          availabilityForm.setFieldValue(['slots', field.name, 'startDate'], date);
                          if (!availabilityForm.getFieldValue(['slots', field.name, 'endDate'])) availabilityForm.setFieldValue(['slots', field.name, 'endDate'], date);
                          availabilityForm.setFieldValue(['slots', field.name, 'dayOfWeek'], weekdayOfDateText(date));
                        }} /></Form.Item>
                      </Space>
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
        title={copyingClass ? '复制课程' : creatingClass ? '新建课程' : '课程详情'}
        open={Boolean(editingClass) || creatingClass}
        width="min(640px, 100vw)"
        onClose={() => {
          setEditingClass(null);
          setCreatingClass(false);
          setCopyingClass(false);
        }}
        extra={(editingClass || creatingClass) && (
          <Space>
            {canCreateClass && editingClass && editingClass.status !== '已上课' && editingClass.status !== '已取消' && editingClass.auditStatus === '已通过' && new Date(`${editingClass.lessonDate}T${editingClass.endTime}`).getTime() <= Date.now() && (
              <Button loading={completeClass.isPending} onClick={() => completeClass.mutate(editingClass.id)}>标记已上课</Button>
            )}
            {editingClass && canAdjustLesson(editingClass) && (
              <Button danger loading={cancelClass.isPending} onClick={() => requestCancel(editingClass.id)}>取消课程</Button>
            )}
            <Button type="primary" disabled={detailReadOnly} loading={preflightBusy || updateClass.isPending || createManualClass.isPending} onClick={() => editForm.submit()}>
              {copyingClass ? '创建复制课程' : creatingClass ? '创建课程' : '保存调课'}
            </Button>
          </Space>
        )}
      >
        {copyingClass && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="已带入原课程信息，请修改上课日期、时间或其他属性。"
          />
        )}
        {editingClass && !canReviewClass && editingClass.auditStatus === '已通过' && <Alert type="info" showIcon message="课程已通过审核，请联系教务调整" style={{ marginBottom: 12 }} />}
        <Form form={editForm} disabled={detailReadOnly} layout="vertical" onFinish={values => {
          const { repeat, ...rest } = values as ScheduleClassFormValues & { repeat?: RepeatFormValues };
          const payload = { ...rest, durationMinutes: clockMinutes(values.endTime) - clockMinutes(values.startTime), editScope, ...(creatingClass && repeatEnabled && repeat ? { repeat: buildRepeatPayload(repeat) } : {}) };
          void preflight(payload, creatingClass ? undefined : editingClass?.id, () => {
            if (creatingClass) createManualClass.mutate({ ...payload, ignoreWarnings: true });
            else updateClass.mutate({ ...payload, ignoreWarnings: true });
          });
        }}>
          <Form.Item name="teacherId" label="老师" rules={[{ required: true, message: '请选择老师' }]}>
            <Select showSearch optionFilterProp="label" options={teacherOptions} onChange={() => { setFormCourseGrade(undefined); setFormCourseSubject(undefined); }} />
          </Form.Item>
          {canReviewClass && formTeacher && <Button type="link" style={{ padding: 0, marginBottom: 12 }} onClick={() => setScopeTeacher(formTeacher)}>编辑老师授课范围</Button>}
          {teachingCourses.length > 8 && <Space.Compact block style={{ marginBottom: 12 }}>
            <Select aria-label="排课课程年级" allowClear placeholder="年级" value={formCourseGrade} options={[...new Set(teachingCourses.map(item => item.grade))].map(value => ({ value, label: value }))} onChange={value => { setFormCourseGrade(value); setFormCourseSubject(undefined); }} style={{ width: '50%' }} />
            <Select aria-label="排课课程学科" allowClear placeholder="学科" value={formCourseSubject} options={[...new Set(teachingCourses.filter(item => !formCourseGrade || item.grade === formCourseGrade).map(item => item.subject))].map(value => ({ value, label: subjectLabel(value) }))} onChange={setFormCourseSubject} style={{ width: '50%' }} />
          </Space.Compact>}
          <Form.Item name="courseId" label="课程" dependencies={['teacherId']} rules={[{ required: true, message: '请选择课程' }, { validator: async () => { if (!validTeacherCourse) throw new Error('课程不在老师授课范围，请调整课程或授课范围'); } }]}>
            <Select showSearch optionFilterProp="label" disabled={detailReadOnly || !editingTeacherId} options={formCourseOptions} notFoundContent={editingTeacherId ? '老师尚无可排课程，请调整授课范围' : '请先选择老师'} />
          </Form.Item>
          <Form.Item name="campusId" hidden><Input /></Form.Item>
          {editingClass?.seriesId && !editingClass.detached && <Form.Item label="本次修改范围"><Select value={editScope} onChange={setEditScope} options={scopeOptions} /></Form.Item>}
          <Space.Compact block>
            <Form.Item name="classType" rules={[{ required: true, message: '请选择班型' }]} style={{ width: '32%' }}>
              <Select options={classTypeOptions} />
            </Form.Item>
            <Form.Item name="durationMinutes" rules={[{ required: true, message: '请输入课长' }]} style={{ width: '68%' }}>
              <InputNumber min={30} max={1440} step={30} addonAfter="分钟" style={{ width: '100%' }} onChange={value => { if (value && isClockText(editingStartTime)) editForm.setFieldValue('endTime', timeText(Math.min(1440, clockMinutes(editingStartTime) + value))); }} />
            </Form.Item>
          </Space.Compact>
          <Form.Item
            name="studentIds"
            label={`学生（已选 ${editingStudentIDs.length}/${classCapacity(editingClassType)}）`}
            dependencies={['classType']}
            rules={[{ validator: async (_, ids?: string[]) => {
              const capacity = classCapacity(editForm.getFieldValue('classType'));
              if (capacity && (ids?.length ?? 0) > capacity) throw new Error(`当前班型最多 ${capacity} 名学生，请调整班型或学生`);
            } }]}
          >
            <Select
              mode="multiple"
              showSearch
              optionFilterProp="label"
              maxCount={classCapacity(editingClassType)}
              options={studentOptions}
            />
          </Form.Item>
          <Form.Item name="startDate" label={creatingClass && repeatEnabled ? '首节上课日期' : '上课日期'} rules={[{ required: true, message: '请选择上课日期' }]}>
            <Input type="date" />
          </Form.Item>
          <Space.Compact block>
            <Form.Item name="startTime" rules={[{ required: true, message: '请输入开始时间' }]} style={{ width: '50%' }}>
              <Input type="time" onChange={event => { if (isClockText(event.target.value)) editForm.setFieldValue('endTime', timeText(Math.min(1440, clockMinutes(event.target.value) + (editForm.getFieldValue('durationMinutes') || 90)))); }} />
            </Form.Item>
            <Form.Item name="endTime" rules={[{ required: true, message: '请输入结束时间' }]} style={{ width: '50%' }}>
              <Input type="time" onChange={event => { if (isClockText(event.target.value) && isClockText(editingStartTime)) editForm.setFieldValue('durationMinutes', clockMinutes(event.target.value) - clockMinutes(editingStartTime)); }} />
            </Form.Item>
          </Space.Compact>


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
          <Space.Compact block>
            <Form.Item name="expectedStudentCount" label="计划招收人数" dependencies={['classType']} style={{ width: '35%' }} rules={[{ validator: async (_, value?: number | null) => {
              if (value === undefined || value === null) return;
              const capacity = classCapacity(editForm.getFieldValue('classType'));
              if (!Number.isInteger(value) || value < 1 || value > capacity) throw new Error(`计划招收人数应为 1 至 ${capacity} 的整数`);
            } }]}>
              <InputNumber min={1} max={classCapacity(editingClassType)} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="reservationNote" label="预留说明" style={{ width: '65%' }}>
              <Input maxLength={255} placeholder="例如：待家长确认学生名单" />
            </Form.Item>
          </Space.Compact>
          <Typography.Text type="secondary">“已确认”表示人数达到开班要求，不代表家长已确认。</Typography.Text>
          <Button block style={{ marginTop: 12 }} onClick={() => setAssistantOpen(open => !open)}>{assistantOpen ? '收起共同时间' : '找共同时间'}</Button>
          {assistantOpen && editingStartDate && <SchedulingAssistant people={editingPeople} date={editingStartDate} duration={Math.max(30, clockMinutes(editingEndTime ?? '20:30') - clockMinutes(editingStartTime ?? '19:00'))}
            lessons={classes.data ?? []} slots={availabilityOverview.data ?? []} excludeId={editingClass?.id} onChoose={(startTime, endTime) => editForm.setFieldsValue({ startTime, endTime, durationMinutes: clockMinutes(endTime) - clockMinutes(startTime) })} />}
          <div className="calendar-preflight" aria-live="polite">
            {livePreview.isFetching ? '正在检查课次…' : livePreview.error ? <Alert type="error" message={livePreview.error instanceof Error ? livePreview.error.message : '预检失败'} /> : livePreview.data && <PreviewResult result={livePreview.data} />}
          </div>
        </Form>
      </Drawer>

      {/* 拖动重复课次时先问清影响范围。以前没有这一步，
          拖一节课会把整个学期的课一起挪走，而且没有任何提示。 */}
      <Modal
        title="调整重复课程"
        open={Boolean(moveScopeRequest)}
        okText="确认调整"
        cancelText="取消"
        confirmLoading={preflightBusy || moveClass.isPending}
        onCancel={() => setMoveScopeRequest(null)}
        onOk={() => {
          if (!moveScopeRequest) return;
          const { record, target } = moveScopeRequest;
          void preflight({ ...scheduleClassPayload(record, target), editScope: moveScope }, record.id, () => moveClass.mutate(
            { record, target, editScope: moveScope }, { onSuccess: () => setMoveScopeRequest(null) }
          ));
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
              : '所选范围内的未来课次将同步调整，已上课和已单独调整的课次保留。'}
          </Typography.Text>
        </Space>
      </Modal>
      <Modal title="取消课程" open={Boolean(cancelRecord)} okText="取消课程" okButtonProps={{ danger: true }} cancelText="保留" confirmLoading={cancelClass.isPending}
        onCancel={() => setCancelRecord(null)} onOk={() => { if (cancelRecord) cancelClass.mutate({ id: cancelRecord.id, editScope: cancelScope }); }}>
        <Space direction="vertical" style={{ width: '100%' }}><span>{cancelRecord?.name}</span>
          {cancelRecord?.seriesId && !cancelRecord.detached && <Select style={{ width: '100%' }} value={cancelScope} onChange={setCancelScope} options={scopeOptions} />}
          <span>取消后释放时间；批量取消保留历史课次和已单独调整的课次。</span>
        </Space>
      </Modal>
    </div>
  );
}

const scopeOptions = [ { label: '仅此课次', value: 'this' }, { label: '此课次及后续', value: 'thisAndFuture' }, { label: '整个系列（未来课次）', value: 'all' } ];
type SchedulePreview = { canSave: boolean; lessons: { date: string; startTime?: string; endTime?: string; errors: string[]; warnings: string[] }[] };
const clockMinutes = (value: string) => Number(value.slice(0, 2)) * 60 + Number(value.slice(3));
const timeText = (value: number) => `${String(Math.floor(value / 60)).padStart(2, '0')}:${String(value % 60).padStart(2, '0')}`;
function PreviewResult({ result }: { result: SchedulePreview }) {
  const issues = result.lessons.filter(lesson => lesson.errors.length || lesson.warnings.length);
  return <div className="preview-result"><Tag color={result.canSave ? 'green' : 'red'}>{result.lessons.length} 节课 · {result.canSave ? '无撞课' : '存在阻塞'}</Tag>
    {result.lessons.length > 1 && <details><summary>查看全部课次</summary>{result.lessons.map(lesson => <div key={lesson.date}>{lesson.date} · {lesson.startTime}–{lesson.endTime}</div>)}</details>}
    <div className="preview-issues">{issues.map(lesson => <div key={lesson.date}><strong>{lesson.date}</strong>{lesson.errors.map((text, i) => <div className="preview-error" key={`e${i}`}>{text}</div>)}{lesson.warnings.map((text, i) => <div className="preview-warning" key={`w${i}`}>{text}</div>)}</div>)}</div>
  </div>;
}
