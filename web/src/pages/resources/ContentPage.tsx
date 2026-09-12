import { Alert, Button, Card, Form, Input, Pagination, Popconfirm, Select, Skeleton, Table, Tabs, Tag, Typography, message } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type React from 'react';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { deleteData, getData, postData, putData } from '../../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../../components/ListViews';
import { CourseDialog, type CourseFormValues } from './ResourceDialogs';
import { DEFAULT_PHASES, DEFAULT_SEMESTERS, gradeOptions, phaseLabel, semesterLabel, subjectLabel, subjectOptions, subjectsForGrade, useSubjectCatalog } from '../../utils/curriculum';
import type { Course, CourseUpsertRequest, CurrentUser, LearningSpace } from '../../types/starline';
import { useSearchParams } from 'react-router-dom';
import MaterialsPage from './MaterialsPage';
import HomeworkPage from './HomeworkPage';
import ReviewsPage from './ReviewsPage';

export default function ContentPage({ user }: { user?: CurrentUser }) {
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') || 'courses';
  const content = tab === 'materials'
    ? <MaterialsPage user={user} courseId={params.get('courseId') || undefined} packageId={params.get('packageId') || undefined} onClearFilter={() => setParams({ tab: 'materials' })} />
    : tab === 'homework'
      ? <HomeworkPage user={user} />
      : tab === 'review'
        ? <ReviewsPage user={user} />
        : <CourseCatalog user={user} onViewMaterials={(courseId) => setParams({ tab: 'materials', courseId })} />;
  return <div className="page-stack"><Card><Tabs activeKey={tab} onChange={(value) => setParams(value === 'courses' ? {} : { tab: value })} items={[{ key: 'courses', label: '课程' }, { key: 'materials', label: '课程讲义' }, { key: 'homework', label: '课后练习' }, { key: 'review', label: '批改反馈' }]} /></Card>{content}</div>;
}

function CourseCatalog({ user, onViewMaterials }: { user?: CurrentUser; onViewMaterials: (courseId: string) => void }) {
  const [form] = Form.useForm<CourseFormValues>();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Course | null>(null);
  const [keyword, setKeyword] = useState('');
  const [gradeFilter, setGradeFilter] = useState<string>();
  const [subjectFilter, setSubjectFilter] = useState<string>();
  const [termFilter, setTermFilter] = useState<string>();
  const [statusFilter, setStatusFilter] = useState<string>();
  const [page, setPage] = useState(1);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:courses', 'table');
  const client = useQueryClient();
  const courses = useQuery({ queryKey: ['content'], queryFn: () => getData<Course[]>('/courses') });
  const spaces = useQuery({ queryKey: ['learning-spaces-for-content'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') });
  const subjectCatalog = useSubjectCatalog();
  const canManage = Boolean(user?.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role)));
  const save = useMutation({
    mutationFn: (values: CourseFormValues) => {
      const { grade: _grade, subject: _subject, curriculum = [], ...courseValues } = values;
      const body: CourseUpsertRequest = {
        ...courseValues,
        curriculum: curriculum.map((node, index) => ({
          ...node,
          id: node.id || `node-${Date.now()}-${index}`,
          sortOrder: Math.max(1, node.sortOrder || index + 1)
        })),
        status: values.status || '启用'
      };
      return editing ? putData<Course>(`/courses/${editing.id}`, body) : postData<Course>('/courses', body);
    },
    onSuccess: () => {
      message.success(editing ? '课程已保存。' : '课程已创建，可继续上传课程讲义和题目。');
      setEditing(null);
      setOpen(false);
      form.resetFields();
      client.invalidateQueries({ queryKey: ['content'] });
      client.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (error: Error) => message.error(error.message || '保存课程失败，请检查课程范围。')
  });
  const remove = useMutation({
    mutationFn: async (ids: string[]) => {
      for (const id of ids) await deleteData(`/courses/${id}`);
    },
    onSuccess: (_data, ids) => {
      message.success(`已删除 ${ids.length} 门课程。`);
      setSelectedRowKeys([]);
      client.invalidateQueries({ queryKey: ['content'] });
      client.invalidateQueries({ queryKey: ['courses'] });
    },
    onError: (error: Error) => message.error(error.message || '删除课程失败，请稍后重试。')
  });
  // 课程本身没有学期/阶段字段——那两个维度挂在课程绑定的学习空间上，
  // 列表要显示完整归属就得按 learningSpaceId 查一次空间。
  const spaceById = useMemo(() => new Map((spaces.data ?? []).map((space) => [space.id, space])), [spaces.data]);
  const termSelectOptions = useMemo(() => termOptions(spaces.data ?? []), [spaces.data]);
  const rows = useMemo(() => {
    const term = keyword.toLowerCase();
    return (courses.data ?? []).filter((item) => {
      if (gradeFilter && item.grade !== gradeFilter) return false;
      if (subjectFilter && item.subject !== subjectFilter) return false;
      if (termFilter && courseTermKey(item, spaceById) !== termFilter) return false;
      if (statusFilter && (item.status || '启用') !== statusFilter) return false;
      if (!term) return true;
      const haystack = [...Object.values(item), courseSpaceLabel(item, spaceById)].join(' ').toLowerCase();
      return haystack.includes(term);
    });
  }, [courses.data, gradeFilter, keyword, spaceById, statusFilter, subjectFilter, termFilter]);
  if (courses.isLoading || spaces.isLoading) return <Skeleton active />;
  if (courses.error || spaces.error) return <Alert type="error" message="课程内容加载失败，请稍后重试。" />;
  const paged = rows.slice((page - 1) * 10, page * 10);
  const unrestricted = Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
  const hasFilters = Boolean(keyword || gradeFilter || subjectFilter || termFilter || statusFilter);
  const spaceLabel = (course: Course) => courseSpaceLabel(course, spaceById);
  const editAction = (course: Course) => canManage ? <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => { setEditing(course); form.setFieldsValue({ ...course, grade: course.grade, subject: course.subject, curriculum: course.curriculum ?? [] }); setOpen(true); }} /> : null;
  const rowSelection = canManage ? { selectedRowKeys, onChange: (keys: React.Key[]) => setSelectedRowKeys(keys.map(String)) } : undefined;

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>课程内容</Typography.Title>
          <Typography.Text type="secondary">维护 Unit、Chapter 和 Lesson，Unit 必填，其余层级可按教学需求灵活设置。点击课程名称可查看该课程的全部课程讲义。</Typography.Text>
        </div>
        <div className="page-heading-actions">
          <ListViewToggle storageKey="starline:list-view:courses" value={viewMode} onChange={setViewMode} />
          {canManage && <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.setFieldsValue({ name: '', grade: undefined, subject: undefined, learningSpaceId: undefined, curriculum: [], status: '启用' }); setOpen(true); }}>新增课程</Button>}
        </div>
      </div>
      <Card>
        <div className="list-toolbar" style={{ marginBottom: 16 }}>
          <div className="course-filter-bar">
            <Input.Search allowClear placeholder="搜索课程" value={keyword} onChange={(event) => { setKeyword(event.target.value); setPage(1); }} style={{ width: 220 }} />
            <ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => courses.refetch()} />
            <Select
              allowClear
              aria-label="年级"
              placeholder="年级"
              value={gradeFilter}
              options={gradeOptions()}
              onChange={(value) => {
                setGradeFilter(value);
                setSubjectFilter((current) => (value && current && !subjectsForGrade(value, subjectCatalog).includes(current) ? undefined : current));
                setPage(1);
              }}
            />
            <Select
              allowClear
              aria-label="学科"
              placeholder="学科"
              value={subjectFilter}
              options={subjectOptions(gradeFilter, subjectCatalog)}
              onChange={(value) => {
                setSubjectFilter(value);
                setPage(1);
              }}
            />
            <Select
              allowClear
              aria-label="学期阶段"
              placeholder="学期 · 阶段"
              value={termFilter}
              options={termSelectOptions}
              onChange={(value) => {
                setTermFilter(value);
                setPage(1);
              }}
            />
            <Select
              allowClear
              aria-label="状态"
              placeholder="状态"
              value={statusFilter}
              options={[{ label: '启用', value: '启用' }, { label: '停用', value: '停用' }]}
              onChange={(value) => {
                setStatusFilter(value);
                setPage(1);
              }}
            />
            {hasFilters && (
              <Button onClick={() => {
                setKeyword('');
                setGradeFilter(undefined);
                setSubjectFilter(undefined);
                setTermFilter(undefined);
                setStatusFilter(undefined);
                setPage(1);
              }}>重置</Button>
            )}
            {canManage && selectedRowKeys.length > 0 && (
              <Popconfirm
                title={`确定删除选中的 ${selectedRowKeys.length} 门课程吗？`}
                description="课程下的讲义和练习也会被移除。"
                okText="删除"
                cancelText="取消"
                okButtonProps={{ danger: true, loading: remove.isPending }}
                onConfirm={() => remove.mutate(selectedRowKeys)}
              >
                <Button danger icon={<DeleteOutlined />}>批量删除（{selectedRowKeys.length}）</Button>
              </Popconfirm>
            )}
          </div>
        </div>
        {viewMode === 'table' ? (
          <Table
            rowKey="id"
            rowSelection={rowSelection}
            dataSource={paged}
            pagination={false}
            locale={{ emptyText: hasFilters ? '没有符合条件的课程。' : '还没有课程，先新增课程。' }}
            columns={[
              { title: '课程', dataIndex: 'name', render: (name: string, row: Course) => <Typography.Link onClick={() => onViewMaterials(row.id)}>{name}</Typography.Link> },
              { title: '年级', dataIndex: 'grade' },
              { title: '学科', dataIndex: 'subject', render: (value: string) => subjectLabel(value) },
              { title: '学期 · 阶段', render: (_: unknown, row: Course) => spaceLabel(row) },
              { title: '课节数', dataIndex: 'lessonCount' },
              { title: '状态', dataIndex: 'status' },
              ...(canManage ? [{ title: '操作', render: (_: unknown, row: Course) => editAction(row) }] : [])
            ]}
          />
        ) : (
          <CardList
            rows={paged}
            rowKey={(course) => course.id}
            emptyText={hasFilters ? '没有符合条件的课程。' : '还没有课程，先新增课程。'}
            renderCard={(course) => (
              <InfoCard
                title={<Typography.Link onClick={() => onViewMaterials(course.id)}>{course.name}</Typography.Link>}
                subtitle={`${course.grade} · ${subjectLabel(course.subject)}`}
                status={<Tag color={course.status === '启用' ? 'green' : 'default'}>{course.status || '启用'}</Tag>}
                fields={[{ label: '学期 · 阶段', value: spaceLabel(course) }, { label: '课节数', value: `${course.lessonCount || 0} 节` }]}
                actions={editAction(course)}
              />
            )}
          />
        )}
        {rows.length > 10 && <Pagination current={page} pageSize={10} total={rows.length} showSizeChanger={false} onChange={(nextPage) => setPage(nextPage)} style={{ marginTop: 16 }} />}
      </Card>
      <CourseDialog form={form} open={open} editing={Boolean(editing)} loading={save.isPending} learningSpaces={spaces.data ?? []} allowedLearningSpaceIds={user?.learningSpaceIds ?? []} unrestricted={unrestricted} onCancel={() => setOpen(false)} onSubmit={(values) => save.mutate(values)} />
    </div>
  );
}

function courseSpaceLabel(course: Course, spaceById: Map<string, LearningSpace>) {
  const space = course.learningSpaceId ? spaceById.get(course.learningSpaceId) : undefined;
  return space ? `${semesterLabel(space.semester)} · ${phaseLabel(space.phase)}` : '—';
}

function courseTermKey(course: Course, spaceById: Map<string, LearningSpace>) {
  const space = course.learningSpaceId ? spaceById.get(course.learningSpaceId) : undefined;
  return space ? termKey(space.semester, space.phase) : '';
}

function termKey(semester: string, phase: string) {
  return `${semester}::${phase}`;
}

function termOptions(spaces: LearningSpace[]) {
  const labels = new Map<string, string>();
  for (const space of spaces) {
    const value = termKey(space.semester, space.phase);
    if (!value || labels.has(value)) continue;
    labels.set(value, `${semesterLabel(space.semester)} · ${phaseLabel(space.phase)}`);
  }
  if (labels.size === 0) {
    for (const semester of DEFAULT_SEMESTERS) {
      for (const phase of DEFAULT_PHASES) {
        labels.set(termKey(semester, phase), `${semesterLabel(semester)} · ${phaseLabel(phase)}`);
      }
    }
  }
  return Array.from(labels.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([value, label]) => ({ value, label }));
}
