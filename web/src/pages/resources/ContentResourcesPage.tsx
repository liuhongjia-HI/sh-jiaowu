import { Alert, Button, Card, Form, Input, Popconfirm, Select, Skeleton, Space, Table, Typography, message } from 'antd';
import { DeleteOutlined, DownloadOutlined, EditOutlined, EyeOutlined, HolderOutlined, PlusOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { deleteData, getData, http, postData, postForm, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { ContentEditDialog, CourseDialog, type CourseFormValues, HomeworkSubmissionDialog, UploadDialog } from './ResourceDialogs';
import { canUpload } from './resource-shared';
import type { Course, CourseUpsertRequest, CurrentUser, Homework, HomeworkSubmissionSummary, LearningSpace, Material, MaterialReorderRequest, QuestionBankItem, StudyPackage } from '../../types/starline';
import type { UploadFile } from 'antd';

type ResourceKind = 'materials' | 'homework';
type UploadValues = { title: string; courseId: string; lessonId: string; tagCode?: string; deadline?: string; deadlineAt?: string; assessmentType?: 'practice' | 'mock_exam'; questionIds?: string[]; fileList?: UploadFile[] };
type ContentValues = Omit<UploadValues, 'fileList'> & { status: string };

function materialTitleFromFile(fileName: string) {
  return fileName.replace(/\.[^.]+$/, '').trim() || fileName;
}

function suggestTagCode(fileName: string) {
  const normalized = fileName.toLowerCase();
  if (normalized.startsWith('hd_')) return 'HD';
  if (normalized.startsWith('blank_')) return 'Blank';
  if (normalized.startsWith('hw_')) return 'HW';
  if (normalized.startsWith('exam_')) return 'Exam';
  if (normalized.startsWith('special_')) return 'Special';
  return '';
}

function deadlineAtValue(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toISOString();
}

function moveMaterial(items: Material[], sourceID: string, targetID: string) {
  const sourceIndex = items.findIndex((item) => item.id === sourceID);
  const targetIndex = items.findIndex((item) => item.id === targetID);
  if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) return items;
  const next = [...items];
  const [source] = next.splice(sourceIndex, 1);
  next.splice(targetIndex, 0, source);
  return next;
}

export function ContentResourcesPage({ kind, user, courseId, packageId, onClearFilter }: { kind: ResourceKind; user?: CurrentUser; courseId?: string; packageId?: string; onClearFilter?: () => void }) {
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Material | Homework | null>(null);
  const [submissionHomework, setSubmissionHomework] = useState<Homework | null>(null);
  const [contentForm] = Form.useForm<ContentValues>();
  const [courseForm] = Form.useForm<CourseFormValues>();
  const [courseEditor, setCourseEditor] = useState<Course | null>(null);
  const client = useQueryClient();
  const title = kind === 'materials' ? '课程讲义' : '课后练习';
  const path = kind === 'materials' ? '/materials' : '/homework';
  const [keyword, setKeyword] = useState('');
  const [subject, setSubject] = useState<string>();
  const [tagCode, setTagCode] = useState<string>();
  const [uploaderId, setUploaderId] = useState<string>();
  const [uploadedFrom, setUploadedFrom] = useState('');
  const [uploadedTo, setUploadedTo] = useState('');
  const [draggingMaterialId, setDraggingMaterialId] = useState('');
  const materialParams = Object.fromEntries(Object.entries({ keyword, subject, tagCode, uploaderId, uploadedFrom, uploadedTo }).filter(([, value]) => Boolean(value))) as Record<string, string>;
  const resources = useQuery({ queryKey: [kind, materialParams], queryFn: () => getData<(Material | Homework)[]>(path, kind === 'materials' ? materialParams : undefined) });
  const allMaterials = useQuery({ queryKey: ['materials', 'all-for-reorder'], enabled: kind === 'materials', queryFn: () => getData<Material[]>('/materials') });
  const courses = useQuery({ queryKey: ['courses-for-content-resources'], queryFn: () => getData<Course[]>('/courses') });
  const packages = useQuery({ queryKey: ['packages-for-content-resources'], enabled: Boolean(packageId), queryFn: () => getData<StudyPackage[]>('/packages') });
  const questions = useQuery({ queryKey: ['question-bank-for-homework'], enabled: kind === 'homework', queryFn: () => getData<QuestionBankItem[]>('/questions') });
  const learningSpaces = useQuery({ queryKey: ['learning-spaces-for-content-resources'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') });
  const submissionSummary = useQuery({
    queryKey: ['homework-submissions', submissionHomework?.id],
    enabled: Boolean(submissionHomework?.id),
    queryFn: () => getData<HomeworkSubmissionSummary>(`/homework/${submissionHomework?.id}/submissions`)
  });
  const canManage = canUpload(kind, user);
  const canManageCourse = Boolean(user?.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role)));
  const unrestrictedCourseScope = Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
  const create = useMutation({
    mutationFn: async (values: UploadValues) => {
      const course = (courses.data ?? []).find((item) => item.id === values.courseId);
      if (!course) throw new Error('请选择课程');
	  if (kind === 'homework') return postData<Homework>('/homework', { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId || '', lessonId: values.lessonId, tagCode: values.tagCode || '', deadlineAt: deadlineAtValue(values.deadlineAt), assessmentType: values.assessmentType || 'practice', status: '启用', questionIds: values.questionIds ?? [] });
      const files = (values.fileList ?? []).map((item) => item.originFileObj).filter(Boolean) as File[];
      if (files.length === 0) throw new Error('请选择文件');
      const uploaded: Material[] = [];
      for (const file of files) {
        const data = new FormData();
        data.append('title', files.length === 1 && values.title?.trim() ? values.title.trim() : materialTitleFromFile(file.name));
        data.append('courseId', course.id);
        data.append('learningSpaceId', course.learningSpaceId || '');
		data.append('lessonId', values.lessonId);
        data.append('tagCode', values.tagCode || suggestTagCode(file.name));
        data.append('file', file);
        uploaded.push(await postForm<Material>('/materials', data));
      }
      return uploaded;
    },
    onSuccess: (result) => {
      const uploadCount = Array.isArray(result) ? result.length : 0;
      message.success(kind === 'materials' ? `已上传 ${uploadCount || 1} 份课程讲义。` : '小挑战已组卷发布。');
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
	  if (kind === 'materials') return putData<Material>(`/materials/${editing.id}`, { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId, lessonId: values.lessonId, tagCode: values.tagCode || '', status: values.status || '已发布' });
	  return putData<Homework>(`/homework/${editing.id}`, { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId, lessonId: values.lessonId, tagCode: values.tagCode || '', deadlineAt: deadlineAtValue(values.deadlineAt), assessmentType: values.assessmentType || 'practice', status: values.status || '启用', questionIds: values.questionIds ?? [] });
    },
    onSuccess: () => {
      message.success(kind === 'materials' ? '课程讲义已保存。' : '题目已保存。');
      setEditing(null);
      contentForm.resetFields();
      client.invalidateQueries({ queryKey: [kind] });
      client.invalidateQueries({ queryKey: ['permissions'] });
    },
    onError: (error: Error) => message.error(error.message || '保存失败，请检查课程范围和发布状态。')
  });
  const saveCourse = useMutation({
    mutationFn: (values: CourseFormValues) => {
      if (!courseEditor) throw new Error('请选择要维护的课程');
      const { grade: _grade, subject: _subject, curriculum = [], ...courseValues } = values;
      const body: CourseUpsertRequest = {
        ...courseValues,
        curriculum: curriculum.map((node, index) => ({ ...node, id: node.id || `node-${Date.now()}-${index}`, sortOrder: index + 1 })),
        status: values.status || '启用'
      };
      return putData<Course>(`/courses/${courseEditor.id}`, body);
    },
    onSuccess: () => {
      message.success('课程目录已同步，当前上传可直接选择新课节。');
      setCourseEditor(null);
      courseForm.resetFields();
      client.invalidateQueries({ queryKey: ['courses-for-content-resources'] });
      client.invalidateQueries({ queryKey: ['courses'] });
      client.invalidateQueries({ queryKey: ['content'] });
    },
    onError: (error: Error) => message.error(error.message || '保存课程目录失败，请检查层级关系。')
  });
  const removeContent = useMutation({
    mutationFn: (id: string) => deleteData(`${path}/${id}`),
    onSuccess: () => {
      message.success(kind === 'materials' ? '课程讲义已删除。' : '课后练习已删除。');
      client.invalidateQueries({ queryKey: [kind] });
		if (kind === 'materials') client.invalidateQueries({ queryKey: ['materials', 'all-for-reorder'] });
    },
    onError: (error: Error) => message.error(error.message || '删除失败，请稍后重试。')
  });
  const retryPreview = useMutation({
    mutationFn: (fileId: string) => postData(`/files/${fileId}/preview/retry`, {}),
    onSuccess: () => {
      message.success('已重新提交课件预览生成。');
      client.invalidateQueries({ queryKey: [kind] });
    },
    onError: (error: Error) => message.error(error.message || '重新生成失败，请稍后重试。')
  });
  const reorderMaterials = useMutation({
    mutationFn: (values: MaterialReorderRequest) => postData('/materials/reorder', values),
    onSuccess: () => {
      message.success('讲义展示顺序已保存，小程序会同步更新。');
      client.invalidateQueries({ queryKey: ['materials'] });
    },
    onError: (error: Error) => message.error(error.message || '排序保存失败，请刷新后重试。')
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
    } catch (error) {
      const reason = error instanceof Error ? error.message : '';
      message.error(reason || (download ? '下载失败，请稍后重试。' : '预览打不开，请下载原文件查看。'));
    }
  };
  const openEdit = (item: Material | Homework) => {
    setEditing(item);
    contentForm.setFieldsValue({
      title: item.title,
      courseId: item.courseId || '',
	  lessonId: item.lessonId || '',
      tagCode: item.tagCode || '',
      deadline: 'deadline' in item ? item.deadline : '',
		deadlineAt: 'deadlineAt' in item ? item.deadlineAt?.slice(0, 16) : '',
		assessmentType: 'assessmentType' in item ? (item.assessmentType || 'practice') : 'practice',
      status: kind === 'materials' ? (item.publishStatus === '停用' || item.status === '停用' ? '停用' : '已发布') : item.status === '已发布' ? '启用' : item.status || '启用',
      questionIds: 'questionIds' in item ? item.questionIds ?? [] : []
    });
  };
  const selectedCourse = (courses.data ?? []).find((item) => item.id === courseId);
  const selectedPackage = (packages.data ?? []).find((item) => item.id === packageId);
  const packageIncludesMaterials = Boolean(selectedPackage?.contentTypeCodes?.includes('handout'));
  const packageSpaceIds = new Set(selectedPackage?.learningSpaceIds ?? []);
  const packageCourseIds = new Set((courses.data ?? []).filter((item) => packageSpaceIds.has(item.learningSpaceId || '')).map((item) => item.id));
  const tableRows = ((resources.data ?? []) as Array<Material | Homework>).filter((item) => {
    if (courseId) return item.courseId === courseId || (selectedCourse && item.course === selectedCourse.name);
    if (packageId) return Boolean(packageIncludesMaterials && (packageSpaceIds.has(item.learningSpaceId || '') || packageCourseIds.has(item.courseId || '')));
    return true;
  });
  const filterDescription = selectedCourse
    ? `正在查看“${selectedCourse.name}”的全部课程讲义。`
    : selectedPackage
      ? `正在查看“${selectedPackage.name}”套餐包含的全部课程讲义。`
      : packageId && !packages.isLoading
        ? '未找到对应套餐，请返回查看全部课程讲义。'
        : '维护课程讲义、图片和课件。';
  const loading = resources.isLoading || courses.isLoading || (Boolean(packageId) && packages.isLoading);
  const loadError = resources.error || courses.error || (Boolean(packageId) && packages.error);

  const handleMaterialDrop = (target: Material) => {
    if (!allMaterials.data) {
      setDraggingMaterialId('');
      message.info('资料正在加载，请稍后再试。');
      return;
    }
    const source = (allMaterials.data ?? []).find((item) => item.id === draggingMaterialId);
    setDraggingMaterialId('');
    if (!source || source.id === target.id) return;
    if (source.courseId !== target.courseId) {
      message.warning('讲义只能在同一课程内调整顺序。');
      return;
    }
    const courseMaterials = (allMaterials.data ?? []).filter((item) => item.courseId === target.courseId);
    const reordered = moveMaterial(courseMaterials, source.id, target.id);
    if (reordered === courseMaterials) return;
    reorderMaterials.mutate({ courseId: target.courseId || '', materialIds: reordered.map((item) => item.id) });
  };

  const moveMaterialByOffset = (source: Material, offset: number) => {
    const courseMaterials = (allMaterials.data ?? []).filter((item) => item.courseId === source.courseId);
    const sourceIndex = courseMaterials.findIndex((item) => item.id === source.id);
    const target = courseMaterials[sourceIndex + offset];
    if (target) handleMaterialDropFromSource(source, target);
  };

  const handleMaterialDropFromSource = (source: Material, target: Material) => {
    if (source.courseId !== target.courseId) return;
    const courseMaterials = (allMaterials.data ?? []).filter((item) => item.courseId === target.courseId);
    const reordered = moveMaterial(courseMaterials, source.id, target.id);
    if (reordered === courseMaterials) return;
    reorderMaterials.mutate({ courseId: target.courseId || '', materialIds: reordered.map((item) => item.id) });
  };

  const subjectOptions = Array.from(new Set((resources.data ?? []).map((row) => row.subject).filter(Boolean))).map((value) => ({ label: value, value }));
  const tagOptions = [{ label: 'HD · 课程讲义', value: 'HD' }, { label: 'Blank · 空白练习', value: 'Blank' }, { label: 'HW · 课后作业', value: 'HW' }, { label: 'Exam · 测试卷', value: 'Exam' }, { label: 'Special · 专题资料', value: 'Special' }];
  const uploaderOptions = Array.from(new Map((resources.data ?? []).filter((row): row is Material => 'ownerTeacherId' in row && Boolean(row.ownerTeacherId)).map((row) => [row.ownerTeacherId as string, { label: row.ownerTeacherName || row.ownerTeacherId as string, value: row.ownerTeacherId as string }])).values());
  return <div className="page-stack">
    <div className="page-heading">
      <div>
        <Typography.Title level={3}>{title}</Typography.Title>
        <Typography.Text type="secondary">{kind === 'materials' ? filterDescription : '从题库选题组卷并发布到学习空间。'}</Typography.Text>
      </div>
      {canManage && <Button type="primary" icon={kind === 'materials' ? <UploadOutlined /> : <PlusOutlined />} onClick={() => setOpen(true)}>{kind === 'materials' ? '上传讲义' : '手动组卷'}</Button>}
    </div>
    {kind === 'materials' && <Card><Space wrap><Input.Search allowClear placeholder="搜索讲义标题" value={keyword} onChange={(event) => setKeyword(event.target.value)} style={{ width: 220 }} /><Select allowClear placeholder="学科" value={subject} onChange={setSubject} options={subjectOptions} style={{ width: 130 }} /><Select allowClear placeholder="主标签" value={tagCode} onChange={setTagCode} options={tagOptions} style={{ width: 150 }} /><Select allowClear showSearch placeholder="上传人" value={uploaderId} onChange={setUploaderId} options={uploaderOptions} style={{ width: 150 }} /><Input type="date" value={uploadedFrom} onChange={(event) => setUploadedFrom(event.target.value)} /><Input type="date" value={uploadedTo} onChange={(event) => setUploadedTo(event.target.value)} /><Button onClick={() => { setKeyword(''); setSubject(undefined); setTagCode(undefined); setUploaderId(undefined); setUploadedFrom(''); setUploadedTo(''); }}>重置</Button></Space></Card>}
    {!canManage && <Alert type="info" showIcon message="当前账号没有上传权限，请联系管理员开通。" />}
    {loading ? <Skeleton active /> : loadError ? <Alert type="error" message={`${title}加载失败，请稍后重试。`} /> : <Card extra={<Space><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => resources.refetch()} />{kind === 'materials' && canManage && <Typography.Text type="secondary">拖动左侧图标即可调整同一课程内的讲义顺序</Typography.Text>}{(courseId || packageId) && onClearFilter && <Button type="link" onClick={onClearFilter}>查看全部讲义</Button>}</Space>}>
      <Table<Material | Homework>
        rowKey="id"
        dataSource={tableRows}
        pagination={{ pageSize: 10 }}
        onRow={(row) => {
          if (kind !== 'materials' || !canManage) return {};
          const target = row as Material;
          return {
            onDragOver: (event) => event.preventDefault(),
            onDrop: (event) => { event.preventDefault(); handleMaterialDrop(target); },
            onDragEnd: () => setDraggingMaterialId('')
          };
        }}
        columns={[
          ...(kind === 'materials' && canManage ? [{ title: '排序', width: 64, render: (_: unknown, row: Material | Homework) => <button type="button" className="material-sort-handle" title="拖动调整顺序；键盘可用上下方向键" aria-label={`拖动 ${String((row as Material).title)} 调整顺序`} draggable={!allMaterials.isLoading} onDragStart={(event) => { setDraggingMaterialId((row as Material).id); event.dataTransfer.effectAllowed = 'move'; }} onKeyDown={(event) => { const offset = event.key === 'ArrowUp' ? -1 : event.key === 'ArrowDown' ? 1 : 0; if (offset) { event.preventDefault(); moveMaterialByOffset(row as Material, offset); } }}><HolderOutlined /></button> }] : []),
          { title: '标题', dataIndex: 'title' },
		  { title: '目录', render: (_: unknown, row: Material | Homework) => { const path = row.curriculum; return path ? `${path.unit} · ${path.chapter} · ${path.lesson}` : '—'; } },
		  ...(kind === 'materials' ? [{ title: '学科', dataIndex: 'subject' }, { title: '上传人', dataIndex: 'ownerTeacherName' }, { title: '上传时间', dataIndex: 'createdAt' }] : [{ title: '类型', render: (_: unknown, row: Material | Homework) => (row as Homework).assessmentType === 'mock_exam' ? '模拟考试' : '常规练习' }, { title: '截止时间', render: (_: unknown, row: Material | Homework) => (row as Homework).deadlineAt ? new Date((row as Homework).deadlineAt as string).toLocaleString() : '不设截止' }]),
          { title: '课程', dataIndex: 'course' },
          { title: '状态', render: (_: unknown, row: Material | Homework) => { const status = kind === 'materials' ? ((row as Material).previewStatus || (row as Material).publishStatus) : row.status; return <div><div>{status}</div>{row.previewError && <Typography.Text type={status === '转换失败' ? 'danger' : 'secondary'} style={{ fontSize: 12 }}>{row.previewError}</Typography.Text>}</div>; } },
          { title: '操作', render: (_: unknown, row: Material | Homework) => {
            const deleteAction = canManage ? <Popconfirm
              title={`确定删除“${row.title}”吗？`}
              description={kind === 'materials' ? '删除后，学生将无法再查看这份讲义。' : '删除后，学生将无法再查看这份练习；已有学生提交记录的练习不能删除，请改为停用。'}
              okText="删除"
              okButtonProps={{ danger: true, loading: removeContent.isPending }}
              cancelText="取消"
              onConfirm={() => removeContent.mutate(row.id)}
            ><ActionButton tooltip="删除" danger icon={<DeleteOutlined />} /></Popconfirm> : null;
            return kind === 'materials'
              ? <Space><ActionButton tooltip="预览" icon={<EyeOutlined />} disabled={(row as Material).previewStatus !== '可预览'} onClick={() => openFile((row as Material).previewUrl, false, (row as Material).fileName)} />{canManage && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(row)} />}{deleteAction}<ActionButton tooltip="下载" icon={<DownloadOutlined />} onClick={() => openFile((row as Material).downloadUrl, true, (row as Material).fileName)} /></Space>
              : <Space><ActionButton tooltip="查看提交详情" icon={<EyeOutlined />} onClick={() => setSubmissionHomework(row as Homework)} />{canManage && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(row)} />}{deleteAction}</Space>;
          } }
        ]}
      />
    </Card>}
    <HomeworkSubmissionDialog homework={submissionHomework} summary={submissionSummary.data} loading={submissionSummary.isLoading} error={Boolean(submissionSummary.error)} onCancel={() => setSubmissionHomework(null)} />
    <UploadDialog kind={kind} open={open} loading={create.isPending} courses={courses.data ?? []} questions={questions.data ?? []} learningSpaces={learningSpaces.data ?? []} onManageCurriculum={canManageCourse ? (course) => { setCourseEditor(course); courseForm.setFieldsValue({ ...course, grade: course.grade, subject: course.subject, curriculum: course.curriculum ?? [] }); } : undefined} onCancel={() => setOpen(false)} onSubmit={(values) => create.mutate(values)} />
    <ContentEditDialog kind={kind} form={contentForm} item={editing} loading={save.isPending} courses={courses.data ?? []} questions={questions.data ?? []} learningSpaces={learningSpaces.data ?? []} onCancel={() => setEditing(null)} onSubmit={(values) => save.mutate(values)} />
    <CourseDialog form={courseForm} open={Boolean(courseEditor)} editing loading={saveCourse.isPending} learningSpaces={learningSpaces.data ?? []} allowedLearningSpaceIds={user?.learningSpaceIds ?? []} unrestricted={unrestrictedCourseScope} onCancel={() => { setCourseEditor(null); courseForm.resetFields(); }} onSubmit={(values) => saveCourse.mutate(values)} />
  </div>;
}
