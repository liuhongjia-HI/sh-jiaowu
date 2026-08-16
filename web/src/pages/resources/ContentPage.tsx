import { Alert, Button, Card, Form, Input, Pagination, Skeleton, Space, Table, Tabs, Typography, message } from 'antd';
import { EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { CourseDialog, type CourseFormValues } from './ResourceDialogs';
import type { Course, CourseUpsertRequest, CurrentUser, LearningSpace } from '../../types/starline';
import { useSearchParams } from 'react-router-dom';
import MaterialsPage from './MaterialsPage';
import HomeworkPage from './HomeworkPage';
import ReviewsPage from './ReviewsPage';

export default function ContentPage({ user }: { user?: CurrentUser }) {
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') || 'courses';
  const content = tab === 'materials' ? <MaterialsPage user={user} /> : tab === 'homework' ? <HomeworkPage user={user} /> : tab === 'review' ? <ReviewsPage /> : <CourseCatalog user={user} />;
  return <div className="page-stack"><Card><Tabs activeKey={tab} onChange={(value) => setParams(value === 'courses' ? {} : { tab: value })} items={[{ key: 'courses', label: '课程' }, { key: 'materials', label: '学习资料' }, { key: 'homework', label: '课后练习' }, { key: 'review', label: '批改反馈' }]} /></Card>{content}</div>;
}

function CourseCatalog({ user }: { user?: CurrentUser }) {
  const [form] = Form.useForm<CourseFormValues>(); const [open, setOpen] = useState(false); const [editing, setEditing] = useState<Course | null>(null); const [keyword, setKeyword] = useState(''); const [page, setPage] = useState(1); const client = useQueryClient();
  const courses = useQuery({ queryKey: ['content'], queryFn: () => getData<Course[]>('/courses') }); const spaces = useQuery({ queryKey: ['learning-spaces-for-content'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') }); const canManage = Boolean(user?.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role)));
  const save = useMutation({ mutationFn: (values: CourseFormValues) => { const { grade: _grade, subject: _subject, ...courseValues } = values; const body: CourseUpsertRequest = { ...courseValues, chapterCount: Number(values.chapterCount || 0), status: values.status || '启用' }; return editing ? putData<Course>(`/courses/${editing.id}`, body) : postData<Course>('/courses', body); }, onSuccess: () => { message.success(editing ? '课程已保存。' : '课程已创建，可继续上传学习资料和题目。'); setEditing(null); setOpen(false); form.resetFields(); client.invalidateQueries({ queryKey: ['content'] }); client.invalidateQueries({ queryKey: ['courses'] }); }, onError: (error: Error) => message.error(error.message || '保存课程失败，请检查课程范围。') });
  if (courses.isLoading || spaces.isLoading) return <Skeleton active />; if (courses.error || spaces.error) return <Alert type="error" message="课程内容加载失败，请稍后重试。" />; const rows = (courses.data ?? []).filter((item) => Object.values(item).join(' ').toLowerCase().includes(keyword.toLowerCase())); const paged = rows.slice((page - 1) * 10, page * 10); const unrestricted = Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
  return <div className="page-stack"><div className="page-heading"><div><Typography.Title level={3}>课程内容</Typography.Title><Typography.Text type="secondary">维护课程、章节和课节安排。</Typography.Text></div>{canManage && <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.setFieldsValue({ name: '', grade: undefined, subject: undefined, learningSpaceId: undefined, chapterCount: 8, status: '启用' }); setOpen(true); }}>新增课程</Button>}</div><Card><Space style={{ marginBottom: 16 }}><Input.Search allowClear placeholder="搜索课程" value={keyword} onChange={(event) => { setKeyword(event.target.value); setPage(1); }} /><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => courses.refetch()} /></Space><Table rowKey="id" dataSource={paged} pagination={false} columns={[{ title: '课程', dataIndex: 'name' }, { title: '章节数', dataIndex: 'chapterCount' }, { title: '状态', dataIndex: 'status' }, ...(canManage ? [{ title: '操作', render: (_: unknown, row: Course) => <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => { setEditing(row); form.setFieldsValue({ ...row, grade: row.grade, subject: row.subject }); setOpen(true); }} /> }] : [])]} />{rows.length > 10 && <Pagination current={page} pageSize={10} total={rows.length} showSizeChanger={false} onChange={setPage} style={{ marginTop: 16 }} />}</Card><CourseDialog form={form} open={open} editing={Boolean(editing)} loading={save.isPending} learningSpaces={spaces.data ?? []} allowedLearningSpaceIds={user?.learningSpaceIds ?? []} unrestricted={unrestricted} onCancel={() => setOpen(false)} onSubmit={(values) => save.mutate(values)} /></div>;
}
