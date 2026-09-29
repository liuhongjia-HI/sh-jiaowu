import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Drawer, Empty, Input, Modal, Select, Space, Spin, Table, Tag, Typography, message } from 'antd';
import { DownloadOutlined, EyeOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, http, postData, postForm } from '../services/http';

type Scope = { grade: string; subject: string };
type Plan = { id: string; title: string; grade: string; subject: string; fileName: string; fileSize: number; fileType: string; previewStatus: string; previewUrl: string; downloadUrl?: string; uploaderName: string; createdAt: string };
type PlanList = { plans: Plan[]; uploadScopes: Scope[]; canUpload: boolean };
type PendingFile = { file: File; title: string; error?: string; done?: boolean };

const nameWithoutExtension = (name: string) => name.replace(/\.[^.]+$/, '');
const scopeKey = (scope: Scope) => `${scope.grade}\u0000${scope.subject}`;
const subjectNames: Record<string, string> = { 英文: '英语', english: '英语', math: '数学', chinese: '语文' };
const displaySubject = (subject: string) => subjectNames[subject] || subject;

export default function TeachingPlans() {
  const client = useQueryClient();
  const query = useQuery({ queryKey: ['teaching-plans'], queryFn: () => getData<PlanList>('/teaching-plans'), refetchOnWindowFocus: true,
    refetchInterval: current => current.state.data?.plans.some(plan => ['待转换', '转换中'].includes(plan.previewStatus)) ? 3000 : false });
  const data = query.data;
  const [keyword, setKeyword] = useState('');
  const [gradeFilter, setGradeFilter] = useState<string>();
  const [subjectFilter, setSubjectFilter] = useState<string>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [uploadScope, setUploadScope] = useState<string>();
  const [pending, setPending] = useState<PendingFile[]>([]);
  const [uploading, setUploading] = useState(false);
  const [selected, setSelected] = useState<Plan>();
  const [previewURL, setPreviewURL] = useState('');
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState('');
  const [previewAttempt, setPreviewAttempt] = useState(0);
  const [downloading, setDownloading] = useState(false);
  const [retrying, setRetrying] = useState(false);
  const active = data?.plans.find(plan => plan.id === selected?.id) || selected;

  const uploadScopes = data?.uploadScopes || [];
  const effectiveUploadScope = uploadScope || (uploadScopes.length === 1 ? scopeKey(uploadScopes[0]) : undefined);
  const canRetry = !!active && uploadScopes.some(scope => scope.grade === active.grade && scope.subject === active.subject);
  const gradeOptions = useMemo(() => Array.from(new Set((data?.plans || []).map(plan => plan.grade))).map(value => ({ value, label: value })), [data]);
  const subjectOptions = useMemo(() => Array.from(new Set((data?.plans || []).filter(plan => !gradeFilter || plan.grade === gradeFilter).map(plan => plan.subject))).map(value => ({ value, label: displaySubject(value) })), [data, gradeFilter]);
  const visible = (data?.plans || []).filter(plan => (!gradeFilter || plan.grade === gradeFilter) && (!subjectFilter || plan.subject === subjectFilter) && (!keyword.trim() || `${plan.title} ${plan.fileName} ${plan.uploaderName}`.toLocaleLowerCase().includes(keyword.trim().toLocaleLowerCase())));

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
  }, [active?.id, active?.previewStatus, previewAttempt]);

  const openPreview = (plan: Plan) => { setPreviewURL(''); setPreviewError(''); setSelected(plan); };
  const closePreview = () => { setSelected(undefined); setPreviewURL(''); };
  const openUpload = () => { setPending([]); setUploadScope(undefined); setDrawerOpen(true); };
  const addFiles = (files: FileList | null) => {
    if (!files?.length) return;
    const additions = Array.from(files).map(file => ({ file, title: nameWithoutExtension(file.name) }));
    setPending(current => [...current, ...additions]);
  };
  const upload = async () => {
    const scope = uploadScopes.find(item => scopeKey(item) === effectiveUploadScope);
    if (!scope || !pending.length) { message.warning('请选择年级学科和教案文件'); return; }
    setUploading(true);
    let success = 0;
    const next = [...pending];
    for (let i = 0; i < next.length; i++) {
      if (next[i].done) continue;
      const body = new FormData();
      body.append('grade', scope.grade);
      body.append('subject', scope.subject);
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
    setUploading(false);
    await client.invalidateQueries({ queryKey: ['teaching-plans'] });
    if (success) message.success(`已上传 ${success} 份教案`);
    if (next.every(item => item.done)) setDrawerOpen(false);
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

  return <div className="page-stack">
    <div className="page-heading"><div><Typography.Title level={3}>教案</Typography.Title><Typography.Text type="secondary">按年级和学科查找内部教案。管理员与对应年级学科教师可见。</Typography.Text></div><Space><Button icon={<ReloadOutlined />} loading={query.isFetching} onClick={() => query.refetch()}>刷新</Button>{data?.canUpload && <Button type="primary" icon={<PlusOutlined />} onClick={openUpload}>上传教案</Button>}</Space></div>
    {query.isLoading ? <Card><Spin /></Card> : query.error ? <Alert type="error" message="教案加载失败" description="请检查网络后重试" action={<Button onClick={() => query.refetch()}>重试</Button>} /> : <Card title="教案列表" extra={<Typography.Text type="secondary">{visible.length} 份</Typography.Text>}>
      <Space wrap style={{ marginBottom: 16 }}><Input.Search aria-label="搜索教案" placeholder="搜索教案、文件名或上传人" allowClear value={keyword} onChange={event => setKeyword(event.target.value)} style={{ width: 270 }} /><Select aria-label="筛选年级" placeholder="全部年级" allowClear value={gradeFilter} options={gradeOptions} onChange={value => { setGradeFilter(value); setSubjectFilter(undefined); }} style={{ minWidth: 130 }} /><Select aria-label="筛选学科" placeholder="全部学科" allowClear value={subjectFilter} options={subjectOptions} onChange={setSubjectFilter} style={{ minWidth: 130 }} />{(keyword || gradeFilter || subjectFilter) && <Button onClick={() => { setKeyword(''); setGradeFilter(undefined); setSubjectFilter(undefined); }}>重置</Button>}</Space>
      <Table<Plan> rowKey="id" dataSource={visible} pagination={{ pageSize: 12, showSizeChanger: false }} locale={{ emptyText: <Empty description={keyword || gradeFilter || subjectFilter ? '当前条件下没有教案，请调整筛选条件' : '当前范围暂无教案'} /> }} columns={[
        { title: '教案', key: 'title', render: (_, plan) => <div><Button type="link" style={{ padding: 0, height: 'auto', whiteSpace: 'normal', textAlign: 'left' }} onClick={() => openPreview(plan)}>{plan.title}</Button><div><Typography.Text type="secondary">{plan.fileName}</Typography.Text></div></div> },
        { title: '年级 · 学科', key: 'scope', render: (_, plan) => <Tag>{plan.grade} · {displaySubject(plan.subject)}</Tag> },
        { title: '上传人', dataIndex: 'uploaderName' },
        { title: '上传时间', dataIndex: 'createdAt' },
        { title: '预览', dataIndex: 'previewStatus', render: (status: string) => <Tag color={status === '可预览' ? 'green' : status === '转换失败' ? 'red' : 'blue'}>{status || '待转换'}</Tag> },
        { title: '操作', key: 'actions', render: (_, plan) => <Space><Button icon={<EyeOutlined />} onClick={() => openPreview(plan)}>查看</Button>{plan.downloadUrl && <Button aria-label={`下载 ${plan.title}`} icon={<DownloadOutlined />} loading={downloading} onClick={() => download(plan)} />}</Space> }
      ]} />
    </Card>}

    <Drawer title="上传教案" width={520} open={drawerOpen} onClose={() => !uploading && setDrawerOpen(false)} maskClosable={!uploading} extra={<Button type="primary" loading={uploading} disabled={!pending.length || !effectiveUploadScope} onClick={upload}>上传 {pending.filter(item => !item.done).length} 份</Button>}>
      <Alert type="info" showIcon message="教案只向管理员和对应年级学科教师展示，学生不可见。" style={{ marginBottom: 20 }} />
      <Typography.Text strong>所属年级 · 学科</Typography.Text><Select aria-label="选择教案所属年级学科" placeholder="请选择年级和学科" value={effectiveUploadScope} options={uploadScopes.map(scope => ({ value: scopeKey(scope), label: `${scope.grade} · ${displaySubject(scope.subject)}` }))} onChange={setUploadScope} style={{ width: '100%', marginTop: 8, marginBottom: 20 }} />
      <Typography.Text strong>教案文件</Typography.Text><div style={{ marginTop: 8, marginBottom: 16 }}><input type="file" accept=".pdf,.doc,.docx,.ppt,.pptx" multiple aria-label="选择教案文件" onChange={event => { addFiles(event.target.files); event.target.value = ''; }} /><div><Typography.Text type="secondary">可一次选择多份 PDF、Word 或 PPT；单文件不超过 50MB。</Typography.Text></div></div>
      {pending.map((item, index) => <Card size="small" key={`${item.file.name}-${index}`} style={{ marginBottom: 10 }}><Space direction="vertical" style={{ width: '100%' }}><Typography.Text type="secondary">{item.file.name} · {(item.file.size / 1024 / 1024).toFixed(1)} MB</Typography.Text><Input aria-label={`第 ${index + 1} 份教案标题`} value={item.title} disabled={item.done || uploading} maxLength={128} onChange={event => setPending(current => current.map((row, i) => i === index ? { ...row, title: event.target.value } : row))} />{item.done ? <Typography.Text type="success">已上传</Typography.Text> : item.error ? <Typography.Text type="danger">{item.error}</Typography.Text> : <Button type="link" danger style={{ padding: 0 }} onClick={() => setPending(current => current.filter((_, i) => i !== index))}>移除</Button>}</Space></Card>)}
    </Drawer>

    <Modal open={!!active} title={active ? <div>{active.title}<div style={{ fontSize: 13, fontWeight: 400 }}>{active.grade} · {displaySubject(active.subject)} · {active.fileName}</div></div> : '查看教案'} onCancel={closePreview} width="92vw" style={{ top: 24 }} footer={<Space>{active?.downloadUrl && <Button icon={<DownloadOutlined />} loading={downloading} onClick={() => download(active)}>下载原文件</Button>}<Button onClick={closePreview}>返回列表</Button></Space>}>
      {previewLoading ? <div className="teacher-preview-placeholder"><Spin size="large" /></div> : previewError ? <Alert type="error" message={previewError} action={<Button onClick={() => setPreviewAttempt(value => value + 1)}>重试打开</Button>} /> : previewURL && active ? <iframe title={`教案预览：${active.title}`} src={previewURL} className="teacher-preview-frame" /> : <Alert type={active?.previewStatus === '转换失败' ? 'error' : 'info'} message={active?.previewStatus === '转换失败' ? '预览生成失败' : '正在生成教案预览'} description={active?.previewStatus === '转换失败' ? (canRetry ? '可重新生成预览，原文件仍可下载。' : '请联系可上传该范围教案的老师或管理员重新生成预览。') : '完成后会自动更新，您也可以先下载原文件。'} action={active?.previewStatus === '转换失败' && canRetry ? <Button loading={retrying} onClick={() => retryPreview(active)}>重新生成预览</Button> : <Button onClick={() => query.refetch()}>刷新状态</Button>} />}
    </Modal>
  </div>;
}
