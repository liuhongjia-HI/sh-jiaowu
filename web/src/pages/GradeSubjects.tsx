import { EditOutlined, PictureOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons';
import { Button, Card, Form, Image, Input, InputNumber, Select, Skeleton, Space, Table, Tag, Upload, message } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { FormDrawer } from '../components/FormDrawer';
import { ActionButton } from '../components/ListViews';
import { getData, postForm, putData, resolveAssetUrl } from '../services/http';
import type { GradeSubjectCatalogUpdateRequest, GradeSubjectMetadata, SubjectMetadata } from '../types/starline';
import { gradeOptions } from '../utils/curriculum';

const MAX_IMAGE_SIZE = 5 * 1024 * 1024;

export default function GradeSubjects() {
  const client = useQueryClient();
  const [form] = Form.useForm<GradeSubjectMetadata>();
  const [editing, setEditing] = useState<GradeSubjectMetadata | null>(null);
  const items = useQuery({ queryKey: ['grade-subjects'], queryFn: () => getData<GradeSubjectMetadata[]>('/grade-subjects') });
  const subjects = useQuery({ queryKey: ['subjects'], queryFn: () => getData<SubjectMetadata[]>('/subjects') });
  const save = useMutation({
    mutationFn: (next: GradeSubjectMetadata) => {
      const exists = (items.data ?? []).some((item) => item.id === next.id);
      const all = exists ? (items.data ?? []).map((item) => item.id === next.id ? next : item) : [...(items.data ?? []), next];
      return putData<GradeSubjectMetadata[]>('/grade-subjects', { items: all } satisfies GradeSubjectCatalogUpdateRequest);
    },
    onSuccess: () => { message.success('年级课程目录已保存。'); setEditing(null); client.invalidateQueries({ queryKey: ['grade-subjects'] }); },
    onError: (error: Error) => message.error(error.message || '保存失败，请稍后重试。')
  });
  const upload = useMutation({ mutationFn: (file: File) => { const body = new FormData(); body.append('file', file); return postForm<{ imageUrl: string }>('/grade-subjects/upload', body); }, onSuccess: ({ imageUrl }) => form.setFieldValue('imageUrl', imageUrl), onError: (error: Error) => message.error(error.message || '图片上传失败') });

  if (items.isLoading || subjects.isLoading) return <Skeleton active />;
  if (items.error || subjects.error) return <Card>年级课程目录加载失败，请刷新后重试。</Card>;
  const subjectOptions = (subjects.data ?? []).filter((item) => item.status === '启用').map((item) => ({ value: item.name, label: item.name }));
  return <div className="page-stack"><div className="page-heading"><div><h2>年级课程目录</h2><p>维护 G1–G12 每个年级向学生展示的学科、排序和课程图片。学生端始终按这里的目录展示，不由套餐数量决定。</p></div><Button type="primary" icon={<PlusOutlined />} onClick={() => { const next: GradeSubjectMetadata = { id: `grade-subject-${Date.now()}`, gradeCode: 'G1', grade: '一年级', subject: '', displayName: '', sortOrder: (items.data ?? []).length + 1, status: '启用' }; setEditing(next); form.setFieldsValue(next); }}>新增学科</Button></div><Card><Table rowKey="id" pagination={false} dataSource={items.data} columns={[
    { title: '年级', dataIndex: 'grade' }, { title: '学科', dataIndex: 'displayName' },
    { title: '图片', render: (_, row: GradeSubjectMetadata) => row.imageUrl ? <Image width={84} height={48} style={{ objectFit: 'cover', borderRadius: 6 }} src={resolveAssetUrl(row.imageUrl)} /> : <Tag icon={<PictureOutlined />}>未上传</Tag> },
    { title: '体验课程', dataIndex: 'previewCourseId', render: (value) => value || <Tag>自动选首课</Tag> }, { title: '排序', dataIndex: 'sortOrder' }, { title: '状态', dataIndex: 'status', render: (value) => <Tag color={value === '启用' ? 'green' : 'default'}>{value}</Tag> },
    { title: '操作', render: (_, row: GradeSubjectMetadata) => <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => { setEditing(row); form.setFieldsValue(row); }} /> }
  ]} /></Card>
  <FormDrawer title={editing ? `编辑${editing.grade}${editing.displayName}` : '编辑年级课程'} open={Boolean(editing)} onCancel={() => setEditing(null)} onSubmit={() => form.submit()} submitting={save.isPending || upload.isPending}>
    <Form form={form} layout="vertical" onFinish={(values) => {
      if (!editing) return;
      const gradeIndex = gradeOptions().findIndex((option) => option.value === values.grade);
      save.mutate({ ...editing, ...values, gradeCode: gradeIndex >= 0 ? `G${gradeIndex + 1}` : editing.gradeCode });
    }}>
      <Form.Item name="grade" label="年级" rules={[{ required: true }]}><Select disabled={!editing?.id.startsWith('grade-subject-')} options={gradeOptions()} /></Form.Item>
      <Form.Item name="subject" label="学科" rules={[{ required: true }]}><Select disabled={!editing?.id.startsWith('grade-subject-')} options={subjectOptions} /></Form.Item>
      <Form.Item name="displayName" label="展示名称" rules={[{ required: true, message: '请输入学生看到的课程名称' }]}><Input /></Form.Item>
      <Form.Item label="课程图片" extra="建议横向图片，JPG 或 PNG，5MB 以内。未上传时学生端显示默认封面。">
        <Upload accept=".jpg,.jpeg,.png" showUploadList={false} beforeUpload={(file) => { if (file.size > MAX_IMAGE_SIZE) { message.error('图片不能超过 5MB'); return Upload.LIST_IGNORE; } upload.mutate(file); return false; }}><Button icon={<UploadOutlined />} loading={upload.isPending}>上传图片</Button></Upload>
        <Form.Item name="imageUrl" noStyle><Input type="hidden" /></Form.Item>
      </Form.Item>
      <Form.Item noStyle shouldUpdate>{() => form.getFieldValue('imageUrl') ? <Image width={160} height={90} style={{ objectFit: 'cover', borderRadius: 8, marginBottom: 16 }} src={resolveAssetUrl(form.getFieldValue('imageUrl'))} /> : null}</Form.Item>
      <Form.Item name="summary" label="课程简介"><Input.TextArea rows={2} placeholder="例如：建立扎实的数学思维与解题能力" /></Form.Item>
      <Space.Compact block><Form.Item name="sortOrder" label="排序" style={{ width: '50%' }}><InputNumber min={0} style={{ width: '100%' }} /></Form.Item><Form.Item name="status" label="状态" style={{ width: '50%' }}><Select options={[{ value: '启用', label: '启用' }, { value: '停用', label: '停用' }]} /></Form.Item></Space.Compact>
      <Form.Item name="previewCourseId" label="体验课程 ID" extra="留空时自动选择该年级、该学科第一门有完整首课内容的课程。"><Input placeholder="一般不需要填写" /></Form.Item>
    </Form>
  </FormDrawer></div>;
}
