import { Alert, Button, Card, Form, Skeleton, Space, Table, Typography, message } from 'antd';
import { DownloadOutlined, EditOutlined, EyeOutlined, PlusOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, http, postData, postForm, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { ContentEditDialog, HomeworkSubmissionDialog, UploadDialog } from './ResourceDialogs';
import { canUpload } from './resource-shared';
import type { Course, CurrentUser, Homework, HomeworkSubmissionSummary, LearningSpace, Material, QuestionBankItem } from '../../types/starline';
import type { UploadFile } from 'antd';

type ResourceKind = 'materials' | 'homework';
type UploadValues = { title: string; courseId: string; chapter?: string; deadline?: string; questionIds?: string[]; fileList?: UploadFile[] };
type ContentValues = Omit<UploadValues, 'fileList'> & { status: string };

function materialTitleFromFile(fileName: string) {
  return fileName.replace(/\.[^.]+$/, '').trim() || fileName;
}

export function ContentResourcesPage({ kind, user, courseId, onClearCourse }: { kind: ResourceKind; user?: CurrentUser; courseId?: string; onClearCourse?: () => void }) {
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Material | Homework | null>(null);
  const [submissionHomework, setSubmissionHomework] = useState<Homework | null>(null);
  const [contentForm] = Form.useForm<ContentValues>();
  const client = useQueryClient();
  const title = kind === 'materials' ? '学习资料' : '课后练习';
  const path = kind === 'materials' ? '/materials' : '/homework';
  const resources = useQuery({ queryKey: [kind], queryFn: () => getData<(Material | Homework)[]>(path) });
  const courses = useQuery({ queryKey: ['courses-for-content-resources'], queryFn: () => getData<Course[]>('/courses') });
  const questions = useQuery({ queryKey: ['question-bank-for-homework'], enabled: kind === 'homework', queryFn: () => getData<QuestionBankItem[]>('/questions') });
  const learningSpaces = useQuery({ queryKey: ['learning-spaces-for-content-resources'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') });
  const submissionSummary = useQuery({
    queryKey: ['homework-submissions', submissionHomework?.id],
    enabled: Boolean(submissionHomework?.id),
    queryFn: () => getData<HomeworkSubmissionSummary>(`/homework/${submissionHomework?.id}/submissions`)
  });
  const canManage = canUpload(kind, user);
  const create = useMutation({
    mutationFn: async (values: UploadValues) => {
      const course = (courses.data ?? []).find((item) => item.id === values.courseId);
      if (!course) throw new Error('请选择课程');
      if (kind === 'homework') return postData<Homework>('/homework', { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId || '', deadline: values.deadline || '', status: '启用', questionIds: values.questionIds ?? [] });
      const files = (values.fileList ?? []).map((item) => item.originFileObj).filter(Boolean) as File[];
      if (files.length === 0) throw new Error('请选择文件');
      const uploaded: Material[] = [];
      for (const file of files) {
        const data = new FormData();
        data.append('title', files.length === 1 && values.title?.trim() ? values.title.trim() : materialTitleFromFile(file.name));
        data.append('courseId', course.id);
        data.append('learningSpaceId', course.learningSpaceId || '');
        data.append('chapter', values.chapter || '');
        data.append('file', file);
        uploaded.push(await postForm<Material>('/materials', data));
      }
      return uploaded;
    },
    onSuccess: (result) => {
      const uploadCount = Array.isArray(result) ? result.length : 0;
      message.success(kind === 'materials' ? `已上传 ${uploadCount || 1} 份学习资料。` : '小挑战已组卷发布。');
      setOpen(false);
      client.invalidateQueries({ queryKey: [kind] });
    },
    onError: (error: Error) => message.error(error.message || '保存失败，请稍后重试。')
  });
  const save = useMutation({
    mutationFn: async (values: ContentValues) => {
      if (!editing) throw new Error('请选择要维护的内容');
      const course = (courses.data ?? []).find((item) => item.id === values.courseId);
      if (!course) throw new Error('请选择课程范围');
      if (kind === 'materials') return putData<Material>(`/materials/${editing.id}`, { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId, chapter: values.chapter || '', status: values.status || '已发布' });
      return putData<Homework>(`/homework/${editing.id}`, { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId, deadline: values.deadline || '', status: values.status || '启用', questionIds: values.questionIds ?? [] });
    },
    onSuccess: () => {
      message.success(kind === 'materials' ? '学习资料已保存。' : '题目已保存。');
      setEditing(null);
      contentForm.resetFields();
      client.invalidateQueries({ queryKey: [kind] });
      client.invalidateQueries({ queryKey: ['permissions'] });
    },
    onError: (error: Error) => message.error(error.message || '保存失败，请检查课程范围和发布状态。')
  });
  const retryPreview = useMutation({
    mutationFn: (fileId: string) => postData(`/files/${fileId}/preview/retry`, {}),
    onSuccess: () => {
      message.success('已重新提交课件预览生成。');
      client.invalidateQueries({ queryKey: [kind] });
    },
    onError: (error: Error) => message.error(error.message || '重新生成失败，请稍后重试。')
  });
  const openFile = async (url: unknown, download = false, name?: unknown) => {
    if (!url) return message.warning('这个文件还不能查看');
    try {
      const response = await http.get<Blob>(String(url).replace(/^\/api/, ''), { responseType: 'blob' });
      const blob = URL.createObjectURL(response.data);
      if (download) {
        const link = document.createElement('a');
        link.href = blob;
        link.download = String(name || '文件');
        link.click();
        window.setTimeout(() => URL.revokeObjectURL(blob), 60_000);
      } else {
        window.open(blob, '_blank', 'noopener,noreferrer');
        window.setTimeout(() => URL.revokeObjectURL(blob), 5 * 60_000);
      }
    } catch {
      message.error(download ? '下载失败，请稍后重试。' : '预览打不开，请下载原文件查看。');
    }
  };
  const openEdit = (item: Material | Homework) => {
    setEditing(item);
    contentForm.setFieldsValue({
      title: item.title,
      courseId: item.courseId || '',
      chapter: 'chapter' in item ? item.chapter : '',
      deadline: 'deadline' in item ? item.deadline : '',
      status: kind === 'materials' ? (item.publishStatus === '停用' || item.status === '停用' ? '停用' : '已发布') : item.status === '已发布' ? '启用' : item.status || '启用',
      questionIds: 'questionIds' in item ? item.questionIds ?? [] : []
    });
  };
  const selectedCourse = (courses.data ?? []).find((item) => item.id === courseId);
  const tableRows = ((resources.data ?? []) as Array<Material | Homework>).filter((item) => !courseId || item.courseId === courseId || (selectedCourse && item.course === selectedCourse.name));

  return <div className="page-stack"><div className="page-heading"><div><Typography.Title level={3}>{title}</Typography.Title><Typography.Text type="secondary">{kind === 'materials' ? (selectedCourse ? `正在查看“${selectedCourse.name}”的全部学习资料。` : '维护学习资料、图片和课件。') : '从题库选题组卷并发布到学习空间。'}</Typography.Text></div>{canManage && <Button type="primary" icon={kind === 'materials' ? <UploadOutlined /> : <PlusOutlined />} onClick={() => setOpen(true)}>{kind === 'materials' ? '上传资料' : '手动组卷'}</Button>}</div>{!canManage && <Alert type="info" showIcon message="当前账号没有上传权限，请联系管理员开通。" />}{resources.isLoading || courses.isLoading ? <Skeleton active /> : resources.error || courses.error ? <Alert type="error" message={`${title}加载失败，请稍后重试。`} /> : <Card extra={<Space><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => resources.refetch()} />{courseId && onClearCourse && <Button type="link" onClick={onClearCourse}>查看全部资料</Button>}</Space>}><Table<Material | Homework> rowKey="id" dataSource={tableRows} pagination={{ pageSize: 10 }} locale={{ emptyText: selectedCourse ? '这门课程暂时还没有上传学习资料。' : undefined }} columns={[{ title: '标题', dataIndex: 'title' }, { title: '课程', dataIndex: 'course' }, { title: '状态', render: (_: unknown, row) => <span title={row.previewError || ''}>{kind === 'materials' ? ((row as Material).previewStatus || (row as Material).publishStatus) : row.status}</span> }, { title: '操作', render: (_: unknown, row) => kind === 'materials' ? <Space><ActionButton tooltip="预览" icon={<EyeOutlined />} disabled={(row as Material).previewStatus !== '可预览'} onClick={() => openFile((row as Material).previewUrl, false, (row as Material).fileName)} />{canManage && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(row)} />}{canManage && row.previewStatus === '转换失败' && row.fileId && <ActionButton tooltip="重新生成预览" icon={<ReloadOutlined />} loading={retryPreview.isPending} onClick={() => retryPreview.mutate(row.fileId as string)} />}<ActionButton tooltip="下载" icon={<DownloadOutlined />} onClick={() => openFile((row as Material).downloadUrl, true, (row as Material).fileName)} /></Space> : <Space><ActionButton tooltip="查看提交详情" icon={<EyeOutlined />} onClick={() => setSubmissionHomework(row as Homework)} />{canManage && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(row)} />}</Space> }]} /></Card>}<HomeworkSubmissionDialog homework={submissionHomework} summary={submissionSummary.data} loading={submissionSummary.isLoading} error={Boolean(submissionSummary.error)} onCancel={() => setSubmissionHomework(null)} /><UploadDialog kind={kind} open={open} loading={create.isPending} courses={courses.data ?? []} questions={questions.data ?? []} learningSpaces={learningSpaces.data ?? []} onCancel={() => setOpen(false)} onSubmit={(values) => create.mutate(values)} /><ContentEditDialog kind={kind} form={contentForm} item={editing} loading={save.isPending} courses={courses.data ?? []} questions={questions.data ?? []} learningSpaces={learningSpaces.data ?? []} onCancel={() => setEditing(null)} onSubmit={(values) => save.mutate(values)} /></div>;
}
