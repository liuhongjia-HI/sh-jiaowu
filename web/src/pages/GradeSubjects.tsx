import { ArrowLeftOutlined, EditOutlined, PictureOutlined, PlusOutlined, RightOutlined, UploadOutlined } from '@ant-design/icons';
import { Button, Card, Empty, Form, Image, Input, InputNumber, Select, Skeleton, Space, Table, Tag, Upload, message } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { FormDrawer } from '../components/FormDrawer';
import { ActionButton } from '../components/ListViews';
import { getData, postForm, putData, resolveAssetUrl } from '../services/http';
import type { GradeSubjectCatalogUpdateRequest, GradeSubjectMetadata, SubjectMetadata } from '../types/starline';
import { gradeOptions } from '../utils/curriculum';

const MAX_IMAGE_SIZE = 5 * 1024 * 1024;
const GRADE_GROUPS = [
  { label: '小学', grades: ['一年级', '二年级', '三年级', '四年级', '五年级', '六年级'] },
  { label: '初中', grades: ['七年级', '八年级', '九年级'] },
  { label: '高中', grades: ['十年级', '十一年级', '十二年级'] }
];

type GradeSummary = { grade: string; gradeCode: string; subjectCount: number; enabledCount: number; missingCoverCount: number };

function gradeCode(grade: string) {
  const index = gradeOptions().findIndex((option) => option.value === grade);
  return index >= 0 ? `G${index + 1}` : grade;
}

function summarizeGrade(items: GradeSubjectMetadata[], grade: string): GradeSummary {
  const rows = items.filter((item) => item.grade === grade);
  return { grade, gradeCode: gradeCode(grade), subjectCount: rows.length, enabledCount: rows.filter((item) => item.status === '启用').length, missingCoverCount: rows.filter((item) => !item.imageUrl).length };
}

export default function GradeSubjects() {
  const client = useQueryClient();
  const [form] = Form.useForm<GradeSubjectMetadata>();
  const [selectedGrade, setSelectedGrade] = useState<string | null>(null);
  const [editing, setEditing] = useState<GradeSubjectMetadata | null>(null);
  const [creating, setCreating] = useState(false);
  const items = useQuery({ queryKey: ['grade-subjects'], queryFn: () => getData<GradeSubjectMetadata[]>('/grade-subjects') });
  const subjects = useQuery({ queryKey: ['subjects'], queryFn: () => getData<SubjectMetadata[]>('/subjects') });
  const gradeRows = items.data ?? [];
  const summaries = useMemo(() => Object.fromEntries(gradeOptions().map((option) => [option.value, summarizeGrade(gradeRows, option.value)])), [gradeRows]);
  const selectedRows = selectedGrade ? gradeRows.filter((item) => item.grade === selectedGrade).sort((left, right) => left.sortOrder - right.sortOrder) : [];
  const save = useMutation({
    mutationFn: (next: GradeSubjectMetadata) => {
      const exists = gradeRows.some((item) => item.id === next.id);
      const all = exists ? gradeRows.map((item) => item.id === next.id ? next : item) : [...gradeRows, next];
      return putData<GradeSubjectMetadata[]>('/grade-subjects', { items: all } satisfies GradeSubjectCatalogUpdateRequest);
    },
    onSuccess: () => { message.success('课程目录已保存。'); setEditing(null); setCreating(false); client.invalidateQueries({ queryKey: ['grade-subjects'] }); },
    onError: (error: Error) => message.error(error.message || '保存失败，请稍后重试。')
  });
  const upload = useMutation({
    mutationFn: (file: File) => { const body = new FormData(); body.append('file', file); return postForm<{ imageUrl: string }>('/grade-subjects/upload', body); },
    onSuccess: ({ imageUrl }) => form.setFieldValue('imageUrl', imageUrl),
    onError: (error: Error) => message.error(error.message || '图片上传失败')
  });

  function openCreate() {
    if (!selectedGrade) return;
    const next: GradeSubjectMetadata = { id: `grade-subject-${Date.now()}`, gradeCode: gradeCode(selectedGrade), grade: selectedGrade, subject: '', displayName: '', sortOrder: selectedRows.length + 1, status: '启用' };
    setCreating(true); setEditing(next); form.setFieldsValue(next);
  }
  function openEdit(row: GradeSubjectMetadata) { setCreating(false); setEditing(row); form.setFieldsValue(row); }

  if (items.isLoading || subjects.isLoading) return <Skeleton active />;
  if (items.error || subjects.error) return <Card>年级课程目录加载失败，请刷新后重试。</Card>;
  const subjectOptions = (subjects.data ?? []).filter((item) => item.status === '启用').map((item) => ({ value: item.name, label: item.name }));
  return <div className="page-stack grade-catalog-page">
    {selectedGrade ? <GradeDetail grade={selectedGrade} summary={summaries[selectedGrade]} rows={selectedRows} onBack={() => setSelectedGrade(null)} onChangeGrade={setSelectedGrade} onCreate={openCreate} onEdit={openEdit} /> : <GradeOverview summaries={summaries} onSelect={setSelectedGrade} />}
    <FormDrawer title={creating ? `新增${selectedGrade}学科` : editing ? `编辑${editing.grade}${editing.displayName}` : '编辑课程目录'} open={Boolean(editing)} onCancel={() => { setEditing(null); setCreating(false); }} onSubmit={() => form.submit()} submitting={save.isPending || upload.isPending}>
      <Form form={form} layout="vertical" onFinish={(values) => editing && save.mutate({ ...editing, ...values, grade: selectedGrade || editing.grade, gradeCode: gradeCode(selectedGrade || editing.grade) })}>
        <Form.Item name="subject" label="学科" rules={[{ required: true, message: '请选择学科' }]}><Select disabled={!creating} options={subjectOptions} placeholder="选择学科" /></Form.Item>
        <Form.Item name="displayName" label="学生端展示名称" rules={[{ required: true, message: '请输入课程名称' }]}><Input placeholder="例如：数学" /></Form.Item>
        <Form.Item label="课程图片" extra="建议使用横向图片，JPG 或 PNG，5MB 以内。"><Upload accept=".jpg,.jpeg,.png" showUploadList={false} beforeUpload={(file) => { if (file.size > MAX_IMAGE_SIZE) { message.error('图片不能超过 5MB'); return Upload.LIST_IGNORE; } upload.mutate(file); return false; }}><Button icon={<UploadOutlined />} loading={upload.isPending}>上传图片</Button></Upload><Form.Item name="imageUrl" noStyle><Input type="hidden" /></Form.Item></Form.Item>
        <Form.Item noStyle shouldUpdate>{() => form.getFieldValue('imageUrl') ? <Image width={180} height={100} style={{ objectFit: 'cover', borderRadius: 8, marginBottom: 16 }} src={resolveAssetUrl(form.getFieldValue('imageUrl'))} /> : null}</Form.Item>
        <Form.Item name="summary" label="课程简介"><Input.TextArea rows={2} placeholder="例如：建立扎实的数学思维与解题能力" /></Form.Item>
        <Space.Compact block><Form.Item name="sortOrder" label="排序" style={{ width: '50%' }}><InputNumber min={0} style={{ width: '100%' }} /></Form.Item><Form.Item name="status" label="状态" style={{ width: '50%' }}><Select options={[{ value: '启用', label: '启用' }, { value: '停用', label: '停用' }]} /></Form.Item></Space.Compact>
        <Form.Item name="previewCourseId" label="体验课程" extra="留空时自动使用该学科首个具备讲义和练习的课节。"><Input placeholder="一般不需要填写" /></Form.Item>
      </Form>
    </FormDrawer>
  </div>;
}

function GradeOverview({ summaries, onSelect }: { summaries: Record<string, GradeSummary>; onSelect: (grade: string) => void }) {
  return <><div className="page-heading grade-catalog-heading grade-catalog-overview-heading"><div><h2>年级课程目录</h2><p>先选择年级，再维护该年级向学生展示的学科、课程图片和体验内容。</p></div><div className="grade-catalog-note" role="note">G1-G5 是小学，G6-G8 是初中，G9-G12 是高中</div></div>{GRADE_GROUPS.map((group) => <section className="grade-group" key={group.label} aria-labelledby={`grade-group-${group.label}`}><div className="grade-group-heading"><h3 id={`grade-group-${group.label}`}>{group.label}</h3><span>{group.grades.length} 个年级</span></div><div className="grade-overview-grid">{group.grades.map((grade) => { const summary = summaries[grade]; return <button className="grade-overview-card" type="button" key={grade} onClick={() => onSelect(grade)}><span className="grade-card-code">{summary.gradeCode}</span><strong>{summary.grade}</strong><span className="grade-card-summary">已配置 {summary.subjectCount} 门学科</span><span className="grade-card-status"><span>{summary.enabledCount} 门启用</span>{summary.missingCoverCount > 0 ? <span className="grade-card-warning">{summary.missingCoverCount} 张图片待补</span> : <span className="grade-card-ready">图片已齐全</span>}</span><RightOutlined aria-hidden="true" className="grade-card-arrow" /></button>; })}</div></section>)}</>;
}

function GradeDetail({ grade, summary, rows, onBack, onChangeGrade, onCreate, onEdit }: { grade: string; summary: GradeSummary; rows: GradeSubjectMetadata[]; onBack: () => void; onChangeGrade: (grade: string) => void; onCreate: () => void; onEdit: (row: GradeSubjectMetadata) => void }) {
  return <><div className="grade-breadcrumb"><Button type="link" icon={<ArrowLeftOutlined />} onClick={onBack}>返回年级列表</Button><span>年级课程目录 / {grade}</span></div><div className="page-heading grade-catalog-heading"><div><h2>{grade}课程目录</h2><p>{summary.enabledCount} 门学科已启用，{summary.missingCoverCount > 0 ? `${summary.missingCoverCount} 张课程图片待补充。` : '课程图片已配置完成。'}</p></div><Space><Select aria-label="切换年级" value={grade} options={gradeOptions()} onChange={onChangeGrade} style={{ width: 130 }} /><Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>新增{grade}学科</Button></Space></div><Card className="grade-subject-table-card">{rows.length === 0 ? <Empty description={`还没有配置${grade}学科`}><Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>新增第一门学科</Button></Empty> : <Table rowKey="id" pagination={false} dataSource={rows} columns={[{ title: '课程图片', width: 132, render: (_, row: GradeSubjectMetadata) => row.imageUrl ? <Image width={92} height={52} style={{ objectFit: 'cover', borderRadius: 6 }} src={resolveAssetUrl(row.imageUrl)} /> : <span className="course-cover-empty"><PictureOutlined /> 待上传</span> }, { title: '学科', dataIndex: 'displayName', render: (value: string, row: GradeSubjectMetadata) => <div><strong>{value}</strong>{row.summary && <div className="subject-summary-text">{row.summary}</div>}</div> }, { title: '体验内容', dataIndex: 'previewCourseId', render: (value: string) => value ? '已指定课程' : <Tag color="blue">自动选首课</Tag> }, { title: '排序', dataIndex: 'sortOrder', width: 90 }, { title: '状态', dataIndex: 'status', width: 100, render: (value: string) => <Tag color={value === '启用' ? 'green' : 'default'}>{value}</Tag> }, { title: '操作', width: 90, render: (_, row: GradeSubjectMetadata) => <ActionButton tooltip="编辑学科" icon={<EditOutlined />} onClick={() => onEdit(row)} /> }]} />}</Card></>;
}
