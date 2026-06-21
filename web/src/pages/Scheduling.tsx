import { CalendarOutlined, CloseCircleOutlined, DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined, SaveOutlined, SettingOutlined, TableOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Drawer, Empty, Form, Input, InputNumber, Popconfirm, Segmented, Select, Skeleton, Space, Table, Tag, Typography, message } from 'antd';
import type { TableColumnsType } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
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
  teacherId?: string;
  courseId?: string;
  classType?: string;
  status?: string;
};

type CandidateLevel = 'full' | 'ready' | 'short';
type CourseLookup = Record<string, Course>;

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

export default function Scheduling({ user }: { user: CurrentUser }) {
  const [availabilityForm] = Form.useForm<AvailabilityFormValues>();
  const [candidateForm] = Form.useForm<CandidateFormValues>();
  const [editForm] = Form.useForm<ScheduleClassFormValues>();
  const [candidateRequest, setCandidateRequest] = useState<CandidateFormValues | null>(null);
  const [selectedCandidate, setSelectedCandidate] = useState<ScheduleCandidate | null>(null);
  const [selectedStudentIds, setSelectedStudentIds] = useState<string[]>([]);
  const [editingClass, setEditingClass] = useState<ScheduleClass | null>(null);
  const [availabilityOpen, setAvailabilityOpen] = useState(false);
  const [moreSettingsOpen, setMoreSettingsOpen] = useState(false);
  const [viewMode, setViewMode] = useState<'week' | 'list'>('week');
  const [classGradeFilter, setClassGradeFilter] = useState<string>();
  const [classTeacherFilter, setClassTeacherFilter] = useState<string>();
  const [classCourseFilter, setClassCourseFilter] = useState<string>();
  const [classTypeFilter, setClassTypeFilter] = useState<string>();
  const [statusFilter, setStatusFilter] = useState<string>('已确认');
  const queryClient = useQueryClient();
  const canCreateClass = user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));

  const teachers = useQuery({ queryKey: ['teachers'], queryFn: () => getData<Teacher[]>('/teachers') });
  const students = useQuery({ queryKey: ['students'], queryFn: () => getData<Student[]>('/students') });
  const courses = useQuery({ queryKey: ['courses'], queryFn: () => getData<Course[]>('/courses') });
  const classes = useQuery({ queryKey: ['schedule-classes'], queryFn: () => getData<ScheduleClass[]>('/schedule-classes') });
  const availabilityOverview = useQuery({ queryKey: ['availability-overview'], queryFn: () => getData<AvailabilitySlot[]>('/availability/overview') });

  const ownerKey = Form.useWatch('ownerKey', availabilityForm);
  const gradeWatch = Form.useWatch('grade', candidateForm);
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
    },
    onError: () => message.error('保存失败，请检查星期和时间段。')
  });

  const createClass = useMutation({
    mutationFn: () => {
      if (!selectedCandidate || !candidateRequest) throw new Error('请选择候选方案');
      return postData<ScheduleClass>('/schedule-classes', {
        courseId: selectedCandidate.courseId,
        teacherId: selectedCandidate.teacherId,
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

  const ownerOptions = useMemo(() => {
    const teacherOptions = (teachers.data ?? []).map((item) => ({ label: `老师 · ${teacherOptionLabel(item)}`, value: `teacher:${item.id}` }));
    const studentOptions = (students.data ?? []).map((item) => ({ label: `学生 · ${studentDisplayName(item)}`, value: `student:${item.id}` }));
    if (user.roles.includes('teacher')) return teacherOptions.filter((item) => item.value === `teacher:${user.userId}`);
    return [...teacherOptions, ...studentOptions];
  }, [teachers.data, students.data, user]);

  const courseOptions = (courses.data ?? []).map((item) => ({ label: `${item.name} · ${item.grade}/${item.subject}`, value: item.id }));
  const teacherOptions = (teachers.data ?? []).map((item) => ({ label: teacherOptionLabel(item), value: item.id }));
  const studentOptions = (students.data ?? []).map((item) => ({ label: studentOptionLabel(item), value: item.id }));
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
    teacherId: classTeacherFilter,
    courseId: classCourseFilter,
    classType: classTypeFilter,
    status: statusFilter
  }), [classGradeFilter, classTeacherFilter, classCourseFilter, classTypeFilter, statusFilter]);
  const filteredClasses = useMemo(() => filterClasses(classes.data ?? [], classFilters, courseById), [classes.data, classFilters, courseById]);
  const allCandidates = candidates.data ?? [];
  const readyCandidates = useMemo(() => allCandidates.filter((item) => candidateLevel(item) !== 'short'), [allCandidates]);
  const shortCandidates = useMemo(() => allCandidates.filter((item) => candidateLevel(item) === 'short'), [allCandidates]);
  const classesByDay = useMemo(() => groupScheduleItems(filteredClasses), [filteredClasses]);
  const candidatesByDay = useMemo(() => groupScheduleItems(readyCandidates), [readyCandidates]);
  const availabilityByDay = useMemo(() => groupScheduleItems(availabilityOverview.data ?? []), [availabilityOverview.data]);
  const availabilitySummary = useMemo(() => availabilityStats(availabilityOverview.data ?? []), [availabilityOverview.data]);
  const activeClassCount = filteredClasses.filter((item) => item.status !== '已取消').length;
  const totalConfirmedClassCount = (classes.data ?? []).filter((item) => item.status === '已确认').length;
  const hasClassFilters = Boolean(classGradeFilter || classTeacherFilter || classCourseFilter || classTypeFilter || statusFilter !== '已确认');
  const classResultNote = scheduleResultNote(filteredClasses, totalConfirmedClassCount, hasClassFilters, classGradeFilter);
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

  // 打开「维护可上课时间」抽屉，并预选某个师生，便于一键协调缺时间的对象。
  function openAvailabilityFor(ownerType: 'teacher' | 'student', ownerId: string) {
    availabilityForm.setFieldValue('ownerKey', `${ownerType}:${ownerId}`);
    setAvailabilityOpen(true);
    setTimeout(() => availability.refetch());
  }

  function openEdit(record: ScheduleClass) {
    setEditingClass(record);
    editForm.setFieldsValue({
      courseId: record.courseId,
      teacherId: record.teacherId,
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
            onChange={(value) => setViewMode(value as 'week' | 'list')}
            options={[
              { label: '课表视图', value: 'week', icon: <CalendarOutlined /> },
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

          <div className="schedule-filterbar">
            <Select allowClear showSearch optionFilterProp="label" placeholder="全部年级" options={gradeOptions()} value={classGradeFilter} onChange={setClassGradeFilter} />
            <Select allowClear placeholder="全部老师" options={teacherOptions} value={classTeacherFilter} onChange={setClassTeacherFilter} />
            <Select allowClear showSearch optionFilterProp="label" placeholder="全部课程" options={courseOptions} value={classCourseFilter} onChange={setClassCourseFilter} />
            <Select allowClear placeholder="全部班型" options={classTypeOptions} value={classTypeFilter} onChange={setClassTypeFilter} />
            <Select options={statusOptions} value={statusFilter} onChange={setStatusFilter} />
            {hasClassFilters && (
              <Button
                onClick={() => {
                  setClassGradeFilter(undefined);
                  setClassTeacherFilter(undefined);
                  setClassCourseFilter(undefined);
                  setClassTypeFilter(undefined);
                  setStatusFilter('已确认');
                }}
              >
                清空筛选
              </Button>
            )}
          </div>

          <div className="schedule-result-note">
            <Typography.Text type={filteredClasses.length === 0 && hasClassFilters ? 'warning' : 'secondary'}>
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
            <WeekScheduleBoard
              loading={classes.isFetching || candidates.isFetching || availabilityOverview.isFetching}
              availabilityByDay={availabilityByDay}
              candidatesByDay={candidatesByDay}
              classesByDay={classesByDay}
              courseById={courseById}
              teacherById={teacherById}
              studentById={studentById}
              candidateRequest={candidateRequest}
              selectedCandidateId={selectedCandidate?.id}
              emptyTips={emptyTips}
              showCandidateEmptyTips={false}
              canManage={canCreateClass}
              onPickCandidate={(record) => {
                setSelectedCandidate(record);
                setSelectedStudentIds(record.availableStudents.slice(0, record.capacity).map((student) => student.id));
              }}
              onEditClass={openEdit}
            />
          ) : (
            filteredClasses.length === 0 ? (
              <Empty
                description={hasClassFilters ? '没有符合筛选条件的课程。' : '还没有已确认课程。'}
              >
                {hasClassFilters && (
                  <Button onClick={() => {
                    setClassTeacherFilter(undefined);
                    setClassCourseFilter(undefined);
                    setClassGradeFilter(undefined);
                    setClassTypeFilter(undefined);
                    setStatusFilter('已确认');
                  }}
                  >
                    清空筛选
                  </Button>
                )}
              </Empty>
            ) : (
              <Table rowKey="id" dataSource={filteredClasses} pagination={false} columns={classColumns(courseById, teacherById, canCreateClass, openEdit, (id) => cancelClass.mutate(id), cancelClass.isPending)} />
            )
          )}
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
            </Form>
          </Space>
        )}
      </Drawer>

      <Drawer
        title="课程详情"
        open={Boolean(editingClass)}
        width={560}
        onClose={() => setEditingClass(null)}
        extra={editingClass && (
          <Space>
            <Popconfirm title="取消这节课？" description="取消后该时间不再占用，可重新排课。" okText="取消课程" cancelText="保留" onConfirm={() => cancelClass.mutate(editingClass.id)}>
              <Button danger loading={cancelClass.isPending}>取消课程</Button>
            </Popconfirm>
            <Button type="primary" loading={updateClass.isPending} onClick={() => editForm.submit()}>保存调课</Button>
          </Space>
        )}
      >
        <Form form={editForm} layout="vertical" onFinish={(values) => updateClass.mutate(values)}>
          <Form.Item name="courseId" label="课程" rules={[{ required: true, message: '请选择课程' }]}>
            <Select showSearch optionFilterProp="label" options={courseOptions} />
          </Form.Item>
          <Form.Item name="teacherId" label="老师" rules={[{ required: true, message: '请选择老师' }]}>
            <Select showSearch optionFilterProp="label" options={teacherOptions} />
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
          <Form.Item name="studentIds" label="学生" rules={[{ required: true, message: '请选择学生' }]}>
            <Select mode="multiple" showSearch optionFilterProp="label" options={studentOptions} />
          </Form.Item>
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

function WeekScheduleBoard({
  loading,
  availabilityByDay,
  candidatesByDay,
  classesByDay,
  courseById,
  teacherById,
  studentById,
  candidateRequest,
  selectedCandidateId,
  emptyTips,
  showCandidateEmptyTips,
  canManage,
  onPickCandidate,
  onEditClass
}: {
  loading: boolean;
  availabilityByDay: Record<number, AvailabilitySlot[]>;
  candidatesByDay: Record<number, ScheduleCandidate[]>;
  classesByDay: Record<number, ScheduleClass[]>;
  courseById: CourseLookup;
  teacherById: Record<string, Teacher>;
  studentById: Record<string, Student>;
  candidateRequest: CandidateFormValues | null;
  selectedCandidateId?: string;
  emptyTips: string[];
  showCandidateEmptyTips: boolean;
  canManage: boolean;
  onPickCandidate: (record: ScheduleCandidate) => void;
  onEditClass: (record: ScheduleClass) => void;
}) {
  const hasAnyItem = weekOptions.some((day) =>
    (availabilityByDay[day.value]?.length ?? 0) > 0 ||
    (candidatesByDay[day.value]?.length ?? 0) > 0 ||
    (classesByDay[day.value]?.length ?? 0) > 0
  );

  if (loading) return <Skeleton active paragraph={{ rows: 6 }} />;
  if (!hasAnyItem) {
    return (
      <ScheduleEmptyTips
        description={candidateRequest ? '暂时没有可展示的排课结果。' : '选择课程和老师后，查找可排时间。'}
        tips={emptyTips}
      />
    );
  }

  return (
    <div className="week-board-wrap">
      {showCandidateEmptyTips && candidateRequest && !hasGroupedItems(candidatesByDay) && (
        <ScheduleEmptyTips description="没有找到推荐时间。" tips={emptyTips} compact />
      )}
      <div className="week-board">
        {weekOptions.map((day) => {
          const dayAvailability = availabilityByDay[day.value] ?? [];
          const dayCandidates = candidatesByDay[day.value] ?? [];
          const dayClasses = classesByDay[day.value] ?? [];
          const isEmptyDay = dayAvailability.length === 0 && dayCandidates.length === 0 && dayClasses.length === 0;

          return (
            <div className="week-day" key={day.value}>
              <div className="week-day-header">
                <strong>{day.label}</strong>
                <span>{dayClasses.filter((item) => item.status !== '已取消').length} 节课</span>
              </div>
              <div className="week-day-body">
                {isEmptyDay ? (
                  <div className="week-day-empty">暂无安排</div>
                ) : (
                  <>
                    {dayAvailability.map((slot) => (
                      <div className={`schedule-block schedule-block-available availability-${slot.ownerType}`} key={slot.id}>
                        <div className="schedule-block-time">{slot.startTime}-{slot.endTime}</div>
                        <div className="schedule-block-title">{slot.ownerType === 'teacher' ? '老师可授课' : '学生可上课'}</div>
                        <div className="schedule-block-meta">{availabilityOwnerDisplayName(slot, teacherById, studentById)}</div>
                      </div>
                    ))}

                    {sortByStartTime(dayCandidates).map((candidate) => {
                      const level = candidateLevel(candidate);
                      const levelMeta = candidateLevelMeta(level);
                      const course = courseById[candidate.courseId];
                      return (
                        <div
                          className={`schedule-block schedule-block-candidate candidate-${level} ${candidate.id === selectedCandidateId ? 'is-selected' : ''}`}
                          key={candidate.id}
                        >
                          <div className="schedule-block-time">{candidate.startTime}-{candidate.endTime}</div>
                          <div className="schedule-block-title">{courseSubjectGradeText(course, candidate.courseName)}</div>
                          <div className="schedule-block-course">{candidate.courseName}</div>
                          <div className="schedule-block-meta">{teacherDisplay(candidate.teacherName, course, teacherById[candidate.teacherId])}</div>
                          <div className="schedule-block-students"><strong>学生：</strong>{studentDisplayNames(candidate.availableStudents)}</div>
                          <div className="schedule-block-tags">
                            <Tag>{candidate.classType}</Tag>
                            <Tag color={levelMeta.color}>{levelMeta.label}</Tag>
                            <Tag color={candidate.studentCount >= candidate.capacity ? 'green' : 'orange'}>{candidate.studentCount}/{candidate.capacity}</Tag>
                          </div>
                          <Button size="small" type={level === 'short' ? 'default' : 'primary'} disabled={level === 'short'} onClick={() => onPickCandidate(candidate)}>确认这个时间</Button>
                        </div>
                      );
                    })}

                    {sortByStartTime(dayClasses).map((item) => {
                      const course = courseById[item.courseId];
                      return (
                        <button
                          type="button"
                          className={`schedule-block schedule-block-class ${item.status === '已取消' ? 'is-canceled' : ''}`}
                          key={item.id}
                          onClick={() => canManage && item.status !== '已取消' ? onEditClass(item) : undefined}
                        >
                          <div className="schedule-block-time">{item.startTime}-{item.endTime}</div>
                          <div className="schedule-block-title">{courseSubjectGradeText(course, item.courseName)}</div>
                          <div className="schedule-block-course">{item.courseName}</div>
                          <div className="schedule-block-meta">{teacherDisplay(item.teacherName, course, teacherById[item.teacherId])}</div>
                          <div className="schedule-block-students"><strong>学生：</strong>{studentDisplayNames(item.students)}</div>
                          <div className="schedule-block-tags">
                            <Tag>{item.classType}</Tag>
                            <Tag>{item.students.length}/{item.capacity}</Tag>
                            <Tag color={item.status === '已取消' ? 'default' : 'green'}>{item.status}</Tag>
                          </div>
                        </button>
                      );
                    })}
                  </>
                )}
              </div>
            </div>
          );
        })}
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

function filterClasses(items: ScheduleClass[], filters: ScheduleFilters, courseById: CourseLookup) {
  return items.filter((item) =>
    (!filters.grade || scheduleClassGrade(item, courseById) === filters.grade) &&
    (!filters.teacherId || item.teacherId === filters.teacherId) &&
    (!filters.courseId || item.courseId === filters.courseId) &&
    (!filters.classType || item.classType === filters.classType) &&
    (!filters.status || filters.status === '全部' || item.status === filters.status)
  );
}

function scheduleClassGrade(item: ScheduleClass, courseById: CourseLookup) {
  return courseById[item.courseId]?.grade || item.students[0]?.grade || '';
}

function groupScheduleItems<T extends { dayOfWeek: number; startTime: string }>(items: T[]) {
  return items.reduce<Record<number, T[]>>((result, item) => {
    result[item.dayOfWeek] = [...(result[item.dayOfWeek] ?? []), item].sort((left, right) => left.startTime.localeCompare(right.startTime));
    return result;
  }, {});
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
