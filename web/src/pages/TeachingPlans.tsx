import { useListPagination, useListSearchParams, useListState } from '../hooks/useListState';
import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Drawer, Empty, Input, Modal, Select, Space, Spin, Table, Tag, Typography, message } from 'antd';
import { DownloadOutlined, EyeOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, http, postData, postForm, putData } from '../services/http';
import { curriculumLessonOptions, subjectsMatch } from '../utils/curriculum';
import { teachingPlanEntryQuery } from '../utils/teachingPlanEntry';
import { ResourceFilterTags } from '../components/ResourceFilterTags';
import type { Course } from '../types/starline';

type Scope = { grade: string; subject: string };
type Plan = { id: string; title: string; grade: string; subject: string; courseId?: string; lessonId?: string; chapter?: string; semester?: string; phase?: string; fileName: string; fileSize: number; fileType: string; previewStatus: string; previewError?: string; previewUrl: string; downloadUrl?: string; uploaderName: string; createdAt: string };
type PlanCourse = Pick<Course, 'id' | 'name' | 'grade' | 'subject' | 'familyId'> & { familyName?: string; semester?: string; phase?: string; level?: string };
type PlanList = { courses?: PlanCourse[]; plans: Plan[]; uploadScopes: Scope[]; canUpload: boolean; directories?: Course[] };
type PendingNoticeBatch = { batchId: string; resourceCount: number };
type PendingFile = { file: File; title: string; lessonId?: string; error?: string; done?: boolean };

const nameWithoutExtension = (name: string) => name.replace(/\.[^.]+$/, '');
const scopeKey = (scope: Scope) => `${scope.grade}\u0000${scope.subject}`;
const subjectNames: Record<string, string> = { 英文: '英语', english: '英语', math: '数学', chinese: '语文' };
const displaySubject = (subject: string) => subjectNames[subject] || subject;

export default function TeachingPlans() {
  const client = useQueryClient();
  const [searchParams, setSearchParams] = useListSearchParams('teaching-plans', []);
  const entryID = new URLSearchParams(teachingPlanEntryQuery('/teaching-plans', searchParams.toString())).get('plan');
  const handledEntry = useRef<string | null>(null);
  const [entryUnavailable, setEntryUnavailable] = useState(false);
  const query = useQuery({ queryKey: ['teaching-plans'], queryFn: () => getData<PlanList>('/teaching-plans'), refetchOnWindowFocus: true,
    refetchInterval: current => current.state.data?.plans.some(plan => ['待转换', '转换中'].includes(plan.previewStatus)) ? 3000 : false });
  const data = query.data;
  const batchID = useRef('');
  const pendingBatches = useQuery({ queryKey: ['teaching-plan-notice-batches'], enabled: data?.canUpload === true,
    queryFn: () => getData<PendingNoticeBatch[]>('/teaching-plans/notification-batches') });
  const [completingNotices, setCompletingNotices] = useState(false);
  const [noticeError, setNoticeError] = useState('');
  const completeNotices = async (ids: string[]) => {
    setCompletingNotices(true);
    setNoticeError('');
    let failure = '';
    for (const id of ids) {
      try { await postData(`/teaching-plans/notification-batches/${encodeURIComponent(id)}/complete`, {}); }
      catch (error) { failure = error instanceof Error ? error.message : '提醒汇总失败'; }
    }
    setNoticeError(failure);
    setCompletingNotices(false);
    await client.invalidateQueries({ queryKey: ['teaching-plan-notice-batches'] });
    return !failure;
  };
  const [keyword, setKeyword] = useListState('teaching-plans:keyword', '');
  const [gradeFilter, setGradeFilter] = useListState<string>('teaching-plans:gradeFilter');
  const [subjectFilter, setSubjectFilter] = useListState<string>('teaching-plans:subjectFilter');
  const [termFilter, setTermFilter] = useListState<string>('teaching-plans:termFilter');
  const [courseFilter, setCourseFilter] = useListState<string>('teaching-plans:courseFilter');
  const pagination = useListPagination('teaching-plans', 12, JSON.stringify([keyword, gradeFilter, subjectFilter, termFilter, courseFilter]));
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [uploadScope, setUploadScope] = useState<string>();
  const [directoryId, setDirectoryId] = useState<string>();
  const [lessonId, setLessonId] = useState<string>();
  const [chapterPlan, setChapterPlan] = useState<Plan>();
  const [chapterCourseId, setChapterCourseId] = useState<string>();
  const [chapterLessonId, setChapterLessonId] = useState<string>();
  const [savingChapter, setSavingChapter] = useState(false);
  const [pending, setPending] = useState<PendingFile[]>([]);
  const [uploading, setUploading] = useState(false);
  const [selected, setSelected] = useState<Plan>();
  const [previewURL, setPreviewURL] = useState('');
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState('');
  const [previewAttempt, setPreviewAttempt] = useState(0);
  const [downloading, setDownloading] = useState(false);
  const [retrying, setRetrying] = useState(false);
  const active = data?.plans.find(plan => plan.id === selected?.id);

  useEffect(() => {
    if (query.isSuccess && selected && !data?.plans.some(plan => plan.id === selected.id)) {
      setSelected(undefined);
      setPreviewURL('');
    }
  }, [query.isSuccess, data, selected?.id]);

  useEffect(() => {
    if (!entryID) { handledEntry.current = null; setEntryUnavailable(false); return; }
    if (!query.isSuccess) return;
    const plan = data?.plans.find(item => item.id === entryID);
    setEntryUnavailable(!plan);
    if (handledEntry.current === entryID) return;
    handledEntry.current = entryID;
    setSelected(plan);
    setPreviewURL('');
    setPreviewError('');
    setEntryUnavailable(!plan);
  }, [entryID, query.isSuccess, data]);

  const uploadScopes = data?.uploadScopes || [];
  const effectiveUploadScope = uploadScope || (uploadScopes.length === 1 ? scopeKey(uploadScopes[0]) : undefined);
  const matchingDirectories = (scope?: Scope) => (data?.directories || []).filter(course => scope && course.grade === scope.grade && subjectsMatch(course.subject, scope.subject));
  const directories = Array.from(new Map(matchingDirectories(uploadScopes.find(scope => scopeKey(scope) === effectiveUploadScope)).map(course => [course.familyId || course.id, course])).values());
  const effectiveDirectoryId = directoryId || (directories.length === 1 ? directories[0].id : undefined);
  const lessons = curriculumLessonOptions(directories.find(course => course.id === effectiveDirectoryId)?.curriculum);
  const effectiveLessonId = lessonId;
  const selectedLessonFor = (item: PendingFile) => item.lessonId === undefined ? effectiveLessonId : item.lessonId || undefined;
  const canRetry = !!active && uploadScopes.some(scope => scope.grade === active.grade && scope.subject === active.subject);
  const planCourses: PlanCourse[] = data?.courses || data?.directories || [];
  const courseById = new Map(planCourses.map(course => [course.id, course]));
  const groupKey = (plan: Plan) => plan.courseId ? courseById.get(plan.courseId)?.familyId || plan.courseId : 'unassigned';
  const gradeOptions = Array.from(new Set([...(data?.plans || []).map(plan => plan.grade), ...planCourses.map(course => course.grade)])).map(value => ({ value, label: value }));
  const subjectOptions = Array.from(new Set([...(data?.plans || []), ...planCourses].filter(item => !gradeFilter || item.grade === gradeFilter).map(item => item.subject))).map(value => ({ value, label: displaySubject(value) }));
  const termOptions = Array.from(new Map([...(data?.plans || []), ...planCourses].filter(plan => plan.semester && (!gradeFilter || plan.grade === gradeFilter) && (!subjectFilter || subjectsMatch(plan.subject, subjectFilter))).map(plan => [`${plan.semester}:${plan.phase || ''}`, { value: `${plan.semester}:${plan.phase || ''}`, label: `${plan.semester?.toUpperCase()} ${plan.phase?.toUpperCase() || ''}`.trim() }])).values());
  const filtered = (data?.plans || []).filter(plan => (!gradeFilter || plan.grade === gradeFilter) && (!subjectFilter || subjectsMatch(plan.subject, subjectFilter)) && (!termFilter || `${plan.semester}:${plan.phase || ''}` === termFilter) && (!keyword.trim() || `${plan.title} ${plan.fileName} ${plan.uploaderName} ${courseById.get(plan.courseId || '')?.name || ''}`.toLocaleLowerCase().includes(keyword.trim().toLocaleLowerCase())));
  const groups = Array.from(new Set(filtered.map(groupKey))).map(id => {
    const plans = filtered.filter(plan => groupKey(plan) === id);
    const related = planCourses.filter(course => (course.familyId || course.id) === id);
    const course = related[0];
    return { id, name: id === 'unassigned' ? '未关联课程' : course?.familyName || course?.name || '历史课程', grade: plans[0].grade, subject: plans[0].subject, semester: plans[0].semester, phase: plans[0].phase, levels: [...new Set(related.map(course => course.level).filter(Boolean))].join(' / '), plans, courseId: data?.directories?.find(item => (item.familyId || item.id) === id)?.id };
  });
  const selectedGroup = groups.find(group => group.id === courseFilter) || (groups.length === 1 ? groups[0] : undefined);
  const visible = selectedGroup?.plans || [];


  useEffect(() => {
    if (!active || active.previewStatus !== '可预览') return;
    let objectURL = '';
    let cancelled = false;
    const controller = new AbortController();
    setPreviewLoading(true);
    setPreviewError('');
    http.get<Blob>(active.previewUrl.replace(/^\/api/, ''), { responseType: 'blob', timeout: 120000, signal: controller.signal }).then(response => {
      if (cancelled) return;
      objectURL = URL.createObjectURL(response.data);
      setPreviewURL(objectURL);
    }).catch(error => { if (!cancelled) setPreviewError(error instanceof Error ? error.message : '教案打开失败'); }).finally(() => { if (!cancelled) setPreviewLoading(false); });
    return () => { cancelled = true; controller.abort(); if (objectURL) URL.revokeObjectURL(objectURL); };
  }, [active?.id, active?.previewStatus, previewAttempt, client]);

  const openPreview = (plan: Plan) => { setPreviewURL(''); setPreviewError(''); setSelected(plan); };
  const closePreview = () => { setSelected(undefined); setPreviewURL(''); const next = new URLSearchParams(searchParams); next.delete('plan'); setSearchParams(next, { replace: true }); };
  const openUpload = (courseId?: string) => { batchID.current = crypto.randomUUID(); setNoticeError(''); setPending([]); const course = data?.directories?.find(item => item.id === courseId); setUploadScope(course ? scopeKey(course) : undefined); setDirectoryId(course?.id); setLessonId(undefined); setDrawerOpen(true); };
  const addFiles = (files: FileList | File[] | null) => {
    if (!files?.length) return;
    const allowed = Array.from(files).filter(file => /\.(pdf|docx?|pptx?)$/i.test(file.name) && file.size <= 50 * 1024 * 1024);
    if (allowed.length !== files.length) message.error('仅支持 50MB 以内的 PDF、Word 或 PPT');
    setPending(current => {
      const key = (file: File) => JSON.stringify([file.name, file.size, file.lastModified]);
      const seen = new Set(current.map(item => key(item.file)));
      const additions = allowed.filter(file => {
        const id = key(file);
        if (seen.has(id)) return false;
        seen.add(id);
        return true;
      });
      return [...current, ...additions.map(file => ({ file, title: nameWithoutExtension(file.name) }))];
    });
  };
  const upload = async (onlyIndex?: number) => {
    const scope = uploadScopes.find(item => scopeKey(item) === effectiveUploadScope);
    if (!scope || !pending.length) { message.warning('请选择年级学科和教案文件'); return; }
    const toUpload = pending.filter((item, index) => !item.done && (onlyIndex === undefined || index === onlyIndex));
    if (directories.length && !effectiveDirectoryId) { message.warning('请选择课程目录'); return; }
    if (toUpload.some(item => selectedLessonFor(item) && !lessons.some(lesson => lesson.value === selectedLessonFor(item)))) { message.warning('所选章节已失效，请重新选择'); return; }
    setUploading(true);
    let success = 0;
    const next = [...pending];
    for (let i = 0; i < next.length; i++) {
      if (next[i].done || onlyIndex !== undefined && i !== onlyIndex) continue;
      const body = new FormData();
      body.append('batchId', batchID.current);
      body.append('grade', scope.grade);
      body.append('subject', scope.subject);
      const selectedLesson = selectedLessonFor(next[i]);
      if (effectiveDirectoryId) body.append('courseId', effectiveDirectoryId);
      if (selectedLesson) body.append('lessonId', selectedLesson);
      body.append('title', next[i].title.trim());
      body.append('file', next[i].file);
      try {
        await postForm<Plan>('/teaching-plans', body);
        next[i] = { ...next[i], done: true, error: undefined };
        success++;
      } catch (error) {
        next[i] = { ...next[i], error: error instanceof Error ? error.message : '上传失败' };
      }
      setPending([...next]);
    }
    const noticesCompleted = !success || await completeNotices([batchID.current]);
    setUploading(false);
    await client.invalidateQueries({ queryKey: ['teaching-plans'] });
    if (success) message.success(`已上传 ${success} 份教案`);
    if (next.every(item => item.done)) { if (noticesCompleted) setDrawerOpen(false); }
    else message.error('部分文件上传失败，请修改后重试');
  };
  const download = async (plan: Plan) => {
    if (!plan.downloadUrl) return;
    setDownloading(true);
    try {
      const response = await http.get<Blob>(plan.downloadUrl.replace(/^\/api/, ''), { responseType: 'blob', timeout: 120000 });
      const href = URL.createObjectURL(response.data);
      const link = document.createElement('a'); link.href = href; link.download = plan.fileName || plan.title; link.click();
      window.setTimeout(() => URL.revokeObjectURL(href), 60000);
    } catch (error) { message.error(error instanceof Error ? error.message : '下载失败'); }
    finally { setDownloading(false); }
  };
  const retryPreview = async (plan: Plan) => {
    setRetrying(true);
    try {
      await postData(`/teaching-plans/${plan.id}/preview/retry`, {});
      await client.invalidateQueries({ queryKey: ['teaching-plans'] });
      message.success('已重新排队生成预览');
    } catch (error) { message.error(error instanceof Error ? error.message : '重试失败'); }
    finally { setRetrying(false); }
  };
  const saveChapter = async () => {
    if (!chapterPlan) return;
    setSavingChapter(true);
    try {
      await putData(`/teaching-plans/${chapterPlan.id}/chapter`, { courseId: chapterCourseId || '', lessonId: chapterLessonId || '' });
      await client.invalidateQueries({ queryKey: ['teaching-plans'] });
      setChapterPlan(undefined); message.success('教案章节已更新');
    } catch (error) { message.error(error instanceof Error ? error.message : '章节更新失败'); }
    finally { setSavingChapter(false); }
  };

  return <div className="page-stack teaching-plans-page">
    <div className="page-heading"><div><Typography.Title level={3}>教案</Typography.Title><Typography.Text type="secondary">按年级和学科查找内部教案。管理员与对应年级学科教师可见。</Typography.Text></div><Space wrap><Button icon={<ReloadOutlined />} loading={query.isFetching} onClick={() => query.refetch()}>刷新</Button>{data?.canUpload && <Button type="primary" icon={<PlusOutlined />} disabled={Boolean(selectedGroup && selectedGroup.id !== 'unassigned' && !selectedGroup.courseId)} onClick={() => openUpload(selectedGroup?.courseId)}>上传教案</Button>}</Space></div>
    {data?.canUpload && (noticeError || (pendingBatches.data?.length ?? 0) > 0 || pendingBatches.error) && <Alert type="warning" showIcon message={noticeError || (pendingBatches.error ? '待汇总提醒加载失败' : `有 ${pendingBatches.data?.length} 批教案待汇总提醒`)} action={<Button loading={completingNotices || pendingBatches.isFetching} disabled={uploading} onClick={() => pendingBatches.error ? pendingBatches.refetch() : completeNotices((pendingBatches.data ?? []).map(row => row.batchId))}>{pendingBatches.error ? '重试' : '汇总提醒'}</Button>} />}
    {query.isLoading ? <Card><Spin /></Card> : query.error ? <Alert type="error" message="教案加载失败" description="请检查网络后重试" action={<Button onClick={() => query.refetch()}>重试</Button>} /> : <Card title="教案列表" extra={<Typography.Text type="secondary">{filtered.length} 份 · {groups.length} 个课程分组</Typography.Text>}>
      <Input.Search aria-label="搜索教案" placeholder="搜索课程、教案、文件名或上传人" allowClear value={keyword} onChange={event => setKeyword(event.target.value)} style={{ maxWidth: 320, marginBottom: 12 }} />
      <ResourceFilterTags label="年级" value={gradeFilter} options={gradeOptions} onChange={value => { setGradeFilter(value); setSubjectFilter(undefined); setTermFilter(undefined); setCourseFilter(undefined); }} />
      <ResourceFilterTags label="学科" value={subjectFilter} options={subjectOptions} onChange={value => { setSubjectFilter(value); setTermFilter(undefined); setCourseFilter(undefined); }} />
      {termOptions.length > 0 && <ResourceFilterTags label="学习阶段" value={termFilter} options={termOptions} onChange={value => { setTermFilter(value); setCourseFilter(undefined); }} />}
      {(keyword || gradeFilter || subjectFilter || termFilter || courseFilter) && <Button size="small" style={{ marginBottom: 12 }} onClick={() => { setKeyword(''); setGradeFilter(undefined); setSubjectFilter(undefined); setTermFilter(undefined); setCourseFilter(undefined); }}>重置</Button>}
      {!selectedGroup ? <Table rowKey="id" dataSource={groups} pagination={pagination} scroll={{ x: 660 }} locale={{ emptyText: <Empty description="当前条件下没有教案" /> }} columns={[
        { title: '课程', render: (_, group) => <Button type="link" onClick={() => setCourseFilter(group.id)}>{group.name}</Button> },
        { title: '年级 · 学科', render: (_, group) => `${group.grade} · ${displaySubject(group.subject)}` },
        { title: '学习阶段', render: (_, group) => group.semester ? `${group.semester.toUpperCase()} ${group.phase?.toUpperCase() || ''}` : '—' },
        { title: '班型', dataIndex: 'levels', render: value => value || '—' },
        { title: '教案', render: (_, group) => `${group.plans.length} 份` },
        { title: '操作', render: (_, group) => <Button onClick={() => setCourseFilter(group.id)}>查看教案</Button> }
      ]} /> : <>
      <Space wrap className="teaching-plan-course-heading"><Typography.Text strong>{selectedGroup.name}</Typography.Text><Tag>{selectedGroup.plans.length} 份教案</Tag>{groups.length > 1 && <Button onClick={() => setCourseFilter(undefined)}>返回课程列表</Button>}</Space>
      <Table<Plan> rowKey="id" dataSource={visible} pagination={pagination} scroll={{ x: 960 }} locale={{ emptyText: <Empty description={keyword || gradeFilter || subjectFilter ? '当前条件下没有教案，请调整筛选条件' : '当前范围暂无教案'} /> }} columns={[
        { title: '教案', key: 'title', render: (_, plan) => <div><Button type="link" style={{ padding: 0, height: 'auto', whiteSpace: 'normal', textAlign: 'left' }} onClick={() => openPreview(plan)}>{plan.title}</Button><div><Typography.Text type="secondary">{plan.fileName}</Typography.Text></div></div> },
        { title: '年级 · 学科', key: 'scope', render: (_, plan) => <Tag>{plan.grade} · {displaySubject(plan.subject)}</Tag> },
        { title: '章节', key: 'chapter', render: (_, plan) => <div>{plan.chapter || <Typography.Text type="secondary">{plan.courseId ? '课程通用' : '未关联课程'}</Typography.Text>}{plan.semester && <div><Typography.Text type="secondary">{plan.semester.toUpperCase()} {plan.phase?.toUpperCase()}</Typography.Text></div>}{uploadScopes.some(scope => scope.grade === plan.grade && subjectsMatch(scope.subject, plan.subject)) && <Button type="link" style={{ padding: 0, display: "block" }} onClick={() => { setChapterPlan(plan); setChapterCourseId(plan.courseId); setChapterLessonId(plan.lessonId); }}>关联章节</Button>}</div> },
        { title: '上传人', dataIndex: 'uploaderName' },
        { title: '上传时间', dataIndex: 'createdAt' },
        { title: '预览', dataIndex: 'previewStatus', render: (status: string) => <Tag color={status === '可预览' ? 'green' : status === '转换失败' ? 'red' : 'blue'}>{status || '待转换'}</Tag> },
        { title: '操作', key: 'actions', render: (_, plan) => <Space><Button icon={<EyeOutlined />} onClick={() => openPreview(plan)}>查看</Button>{plan.downloadUrl && <Button aria-label={`下载 ${plan.title}`} icon={<DownloadOutlined />} loading={downloading} onClick={() => download(plan)} />}</Space> }
      ]} /></>}
    </Card>}

    {entryUnavailable && <Alert type="warning" showIcon message="该教案不存在或当前账号无权查看" style={{ marginBottom: 16 }} />}

    <Drawer title="上传教案" width="min(520px, 100vw)" open={drawerOpen} onClose={() => !uploading && setDrawerOpen(false)} maskClosable={!uploading} extra={<Button type="primary" loading={uploading} disabled={!pending.some(item => !item.done) || !effectiveUploadScope} onClick={() => upload()}>上传 {pending.filter(item => !item.done).length} 份</Button>}>
      {noticeError && <Alert type="warning" showIcon message={noticeError} action={<Button loading={completingNotices} disabled={uploading} onClick={async () => { if (await completeNotices([batchID.current]) && pending.every(item => item.done)) setDrawerOpen(false); }}>重试汇总</Button>} style={{ marginBottom: 16 }} />}
      <Alert type="info" showIcon message="教案只向管理员和对应年级学科教师展示，学生不可见。" style={{ marginBottom: 20 }} />
      <Typography.Text strong>所属年级 · 学科</Typography.Text><Select showSearch optionFilterProp="label" aria-label="选择教案所属年级学科" placeholder="请选择年级和学科" value={effectiveUploadScope} options={uploadScopes.map(scope => ({ value: scopeKey(scope), label: `${scope.grade} · ${displaySubject(scope.subject)}` }))} disabled={uploading} onChange={value => { setUploadScope(value); setDirectoryId(undefined); setLessonId(undefined); setPending(current => current.map(row => row.done ? row : { ...row, lessonId: undefined })); }} style={{ width: '100%', marginTop: 8, marginBottom: 20 }} />
      {directories.length > 0 && <Space direction="vertical" style={{ width: '100%', marginBottom: 20 }}><Select showSearch optionFilterProp="label" aria-label="教案课程目录" placeholder="选择课程目录" value={effectiveDirectoryId} options={directories.map(course => ({ value: course.id, label: course.name }))} disabled={uploading} onChange={value => { setDirectoryId(value); setLessonId(undefined); setPending(current => current.map(row => row.done ? row : { ...row, lessonId: undefined })); }} style={{ width: '100%' }} /><Select allowClear aria-label="教案章节" placeholder="统一章节（选填）" value={effectiveLessonId} options={lessons} disabled={uploading || !effectiveDirectoryId} onChange={setLessonId} style={{ width: '100%' }} /></Space>}
      <Typography.Text strong>教案文件</Typography.Text><div onDragOver={event => event.preventDefault()} onDrop={event => { event.preventDefault(); if (!uploading) addFiles(event.dataTransfer.files); }} style={{ marginTop: 8, marginBottom: 16, padding: 16, border: "1px dashed #b9c8d1", borderRadius: 8 }}><input disabled={uploading} type="file" accept=".pdf,.doc,.docx,.ppt,.pptx" multiple aria-label="选择教案文件" onChange={event => { addFiles(event.target.files); event.target.value = ''; }} /><div><Typography.Text type="secondary">拖入或选择多份 PDF、Word 或 PPT；单文件不超过 50MB。</Typography.Text></div></div>
      {pending.map((item, index) => <Card size="small" key={`${item.file.name}-${index}`} style={{ marginBottom: 10 }}><Space direction="vertical" style={{ width: '100%' }}><Typography.Text type="secondary">{item.file.name} · {(item.file.size / 1024 / 1024).toFixed(1)} MB</Typography.Text><Input aria-label={`第 ${index + 1} 份教案标题`} value={item.title} disabled={item.done || uploading} maxLength={128} onChange={event => setPending(current => current.map((row, i) => i === index ? { ...row, title: event.target.value } : row))} />{directories.length > 0 && <Select aria-label={`第 ${index + 1} 份教案章节`} allowClear placeholder="跟随统一章节（可不选）" value={item.lessonId} options={[{ value: '', label: '不关联章节' }, ...lessons]} disabled={item.done || uploading || !effectiveDirectoryId} onChange={value => setPending(current => current.map((row, i) => i === index ? { ...row, lessonId: value } : row))} style={{ width: "100%" }} />}{item.done ? <Typography.Text type="success">已上传</Typography.Text> : item.error ? <Space wrap><Typography.Text type="danger">{item.error}</Typography.Text><Button loading={uploading} onClick={() => upload(index)}>重试此文件</Button><Button danger disabled={uploading} onClick={() => setPending(current => current.filter((_, i) => i !== index))}>移除</Button></Space> : <Button type="link" danger disabled={uploading} style={{ padding: 0 }} onClick={() => setPending(current => current.filter((_, i) => i !== index))}>移除</Button>}</Space></Card>)}
    </Drawer>

    <Modal title="关联教案章节" open={!!chapterPlan} onCancel={() => !savingChapter && setChapterPlan(undefined)} confirmLoading={savingChapter} onOk={saveChapter}>
      <Space direction="vertical" style={{ width: '100%' }}><Select aria-label="重新关联课程目录" allowClear placeholder="未归类" value={chapterCourseId} options={matchingDirectories(chapterPlan).map(course => ({ value: course.id, label: course.name }))} onChange={value => { setChapterCourseId(value); setChapterLessonId(undefined); }} style={{ width: '100%' }} /><Select allowClear aria-label="重新关联章节" placeholder="章节（选填，留空为课程通用）" value={chapterLessonId} options={curriculumLessonOptions(data?.directories?.find(course => course.id === chapterCourseId)?.curriculum)} disabled={!chapterCourseId} onChange={setChapterLessonId} style={{ width: '100%' }} /></Space>
    </Modal>

    <Modal destroyOnClose open={!!active} title={active ? <div>{active.title}<div style={{ fontSize: 13, fontWeight: 400 }}>{active.grade} · {displaySubject(active.subject)} · {active.fileName}</div></div> : '查看教案'} onCancel={closePreview} width="92vw" style={{ top: 24 }} footer={<Space>{active?.downloadUrl && <Button icon={<DownloadOutlined />} loading={downloading} onClick={() => download(active)}>下载原文件</Button>}<Button onClick={closePreview}>返回列表</Button></Space>}>
      {previewLoading ? <div className="teacher-preview-placeholder"><Spin size="large" /></div> : previewError ? <Alert type="error" message={previewError} action={<Button onClick={() => setPreviewAttempt(value => value + 1)}>重试打开</Button>} /> : previewURL && active ? <iframe title={`教案预览：${active.title}`} src={previewURL} className="teacher-preview-frame" /> : <Alert type={active?.previewStatus === '转换失败' ? 'error' : 'info'} message={active?.previewStatus === '转换失败' ? '预览生成失败' : '正在生成教案预览'} description={active?.previewStatus === '转换失败' ? [active.previewError, canRetry ? '可重新生成预览，原文件仍可下载。' : '请联系可上传该范围教案的老师或管理员处理。'].filter(Boolean).join(' ') : '完成后会自动更新，您也可以先下载原文件。'} action={active?.previewStatus === '转换失败' && canRetry ? <Button loading={retrying} onClick={() => retryPreview(active)}>重新生成预览</Button> : <Button onClick={() => query.refetch()}>刷新状态</Button>} />}
    </Modal>
  </div>;
}
