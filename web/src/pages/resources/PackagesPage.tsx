import { Alert, Button, Card, Form, Input, Pagination, Select, Skeleton, Space, Table, Typography, message } from 'antd';
import { EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { PackageDialog } from './ResourceDialogs';
import { academicYearForDate, ALL_SUBJECTS, DEFAULT_ACADEMIC_YEAR, gradeIndex, semesterOptions } from '../../utils/curriculum';
import type { CurrentUser, LearningSpace, PackageUpsertRequest, StudyPackage } from '../../types/starline';

export default function PackagesPage({ user }: { user?: CurrentUser }) {
  const [form] = Form.useForm<PackageUpsertRequest>(); const [open, setOpen] = useState(false); const [editing, setEditing] = useState<StudyPackage | null>(null); const [keyword, setKeyword] = useState(''); const [page, setPage] = useState(1); const client = useQueryClient();
  const packages = useQuery({ queryKey: ['packages'], queryFn: () => getData<StudyPackage[]>('/packages') }); const spaces = useQuery({ queryKey: ['learning-spaces-for-packages'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') }); const settings = useQuery({ queryKey: ['settings-for-packages'], queryFn: () => getData<Record<string, string>>('/settings') });
  const canManage = Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
  const save = useMutation({ mutationFn: (values: PackageUpsertRequest) => { const body = { ...values, summary: values.summary || '', phaseScope: values.phaseScope || '全学期', packageType: values.packageType || '', status: values.status || '启用' }; return editing ? putData<StudyPackage>(`/packages/${editing.id}`, body) : postData<StudyPackage>('/packages', body); }, onSuccess: () => { message.success(editing ? '学习套餐已保存。' : '学习套餐已创建，可给学生开通。'); setOpen(false); setEditing(null); form.resetFields(); client.invalidateQueries({ queryKey: ['packages'] }); client.invalidateQueries({ queryKey: ['dashboard'] }); }, onError: (error: Error) => message.error(error.message || '保存套餐失败，请检查学习空间和内容类型。') });
  const rows = useMemo(() => { const term = keyword.toLowerCase(); return [...(packages.data ?? [])].filter((item) => !term || Object.values(item).join(' ').toLowerCase().includes(term)).sort(comparePackages); }, [packages.data, keyword]);
  if (packages.isLoading || spaces.isLoading || settings.isLoading) return <Skeleton active />; if (packages.error || spaces.error || settings.error) return <Alert type="error" message="学习套餐加载失败，请稍后重试。" />;
  const years = Array.from(new Set([academicYearForDate(), settings.data?.academicYear, ...(packages.data ?? []).map((item) => item.academicYear), ...(spaces.data ?? []).map((item) => item.academicYear), DEFAULT_ACADEMIC_YEAR].filter(Boolean))).map((value) => ({ label: String(value), value: String(value) })); const semesters = semesterOptions(settings.data?.semesters); const paged = rows.slice((page - 1) * 10, page * 10); const defaults: PackageUpsertRequest = { name: '', academicYear: academicYearForDate(), grade: '' as never, subject: '' as never, semester: semesters[0]?.value || 'S1', phaseScope: '全学期', packageType: '', summary: '', learningSpaceIds: [], contentTypeCodes: ['question', 'handout'], status: '启用' };
  const initialValues: PackageUpsertRequest = editing ? { name: editing.name, academicYear: editing.academicYear, grade: editing.grade, subject: editing.subject, semester: editing.semester, phaseScope: editing.phaseScope, packageType: editing.packageType, summary: editing.summary, learningSpaceIds: editing.learningSpaceIds ?? [], contentTypeCodes: editing.contentTypeCodes ?? [], status: editing.status } : defaults;
  return <div className="page-stack"><div className="page-heading"><div><Typography.Title level={3}>学习套餐</Typography.Title><Typography.Text type="secondary">维护年级、学科和开放内容。</Typography.Text></div>{canManage && <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); setOpen(true); }}>新增套餐</Button>}</div><Card><Space style={{ marginBottom: 16 }}><Input.Search allowClear placeholder="搜索学习套餐" value={keyword} onChange={(event) => { setKeyword(event.target.value); setPage(1); }} /><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => packages.refetch()} /></Space><Table rowKey="id" dataSource={paged} pagination={false} columns={[{ title: '年级', dataIndex: 'grade' }, { title: '学科', dataIndex: 'subject' }, { title: '期次', render: (_: unknown, row: StudyPackage) => [row.semester, row.phaseScope].filter(Boolean).join(' · ') }, { title: '名称', dataIndex: 'name' }, { title: '状态', dataIndex: 'status' }, ...(canManage ? [{ title: '操作', render: (_: unknown, row: StudyPackage) => <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => { setEditing(row); setOpen(true); }} /> }] : [])]} />{rows.length > 10 && <Pagination current={page} pageSize={10} total={rows.length} showSizeChanger={false} onChange={setPage} style={{ marginTop: 16 }} />}</Card><PackageDialog form={form} open={open} editing={Boolean(editing)} loading={save.isPending} learningSpaces={spaces.data ?? []} academicYearOptions={years} semesterOptions={semesters} initialValues={initialValues} onCancel={() => setOpen(false)} onSubmit={(values) => save.mutate(values)} /></div>;
}

function comparePackages(left: StudyPackage, right: StudyPackage) {
  const gradeDiff = sortableIndex(gradeIndex(left.grade)) - sortableIndex(gradeIndex(right.grade));
  if (gradeDiff !== 0) return gradeDiff;
  const subjectDiff = sortableIndex(ALL_SUBJECTS.indexOf(left.subject)) - sortableIndex(ALL_SUBJECTS.indexOf(right.subject));
  if (subjectDiff !== 0) return subjectDiff;
  const phaseOrder = ['期中', 'Q1', '期末', 'Q2', '全学期'];
  const phaseDiff = sortableIndex(phaseOrder.indexOf(left.phaseScope)) - sortableIndex(phaseOrder.indexOf(right.phaseScope));
  if (phaseDiff !== 0) return phaseDiff;
  return left.name.localeCompare(right.name, 'zh-Hans-CN');
}

function sortableIndex(index: number) {
  return index < 0 ? Number.MAX_SAFE_INTEGER : index;
}
