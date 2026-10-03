import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Empty, Form, Input, Modal, Select, Space, Tag, Typography, message } from 'antd';
import { CopyOutlined, DownloadOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { getData, http, postData } from '../services/http';
import type { Course, LearningSpace } from '../types/starline';
import { phaseLabel, semesterLabel, subjectLabel, subjectsMatch } from '../utils/curriculum';

type Scope = { subject: string; grade?: string; semester?: string; phase?: string; courseIds?: string[]; lessonIds?: string[] };
type Selection = { count: number; size: number; courses: string[] };
type Job = { id: string; scope: Scope; status: string; count: number; size: number; error?: string; createdAt: string; expiresAt?: string };
const sizeText = (size: number) => size >= 1024 ** 3 ? `${(size / 1024 ** 3).toFixed(1)} GB` : size >= 1024 ** 2 ? `${(size / 1024 ** 2).toFixed(1)} MB` : `${Math.ceil(size / 1024)} KB`;

export function MaterialDownloads({ userId, courses, spaces, canDownload, defaultSubject }: { userId: string; courses: Course[]; spaces: LearningSpace[]; canDownload: boolean; defaultSubject?: string }) {
  const [params, setParams] = useSearchParams();
  const pickup = params.get('downloadJob') || '';
  const [open, setOpen] = useState(Boolean(pickup));
  const [scope, setScope] = useState<Scope>({ subject: defaultSubject || '' });
  const [busy, setBusy] = useState('');
  const [link, setLink] = useState('');
  const [error, setError] = useState('');
  const client = useQueryClient();
  const key = ['material-downloads', userId];
  useEffect(() => { if (pickup) setOpen(true); }, [pickup]);
  const jobs = useQuery({ queryKey: key, queryFn: () => getData<Job[]>('/material-downloads'), enabled: open,
    refetchInterval: q => q.state.data?.some(j => ['准备中', '打包中'].includes(j.status)) ? 2000 : false });
  const quote = useQuery({ queryKey: [...key, 'selection', scope], queryFn: () => postData<Selection>('/material-downloads/selection', scope), enabled: open && canDownload && Boolean(scope.subject), retry: false });
  const spaceMap = useMemo(() => new Map(spaces.map(s => [s.id, s])), [spaces]);
  const available = courses.filter(c => c.status === '启用' && (!scope.subject || subjectsMatch(c.subject, scope.subject)) && (!scope.grade || c.grade === scope.grade) && (!scope.semester || spaceMap.get(c.learningSpaceId || '')?.semester === scope.semester) && (!scope.phase || spaceMap.get(c.learningSpaceId || '')?.phase === scope.phase));
  const options = (field: 'subject' | 'grade' | 'semester' | 'phase') => Array.from(new Set(courses.filter(c => c.status === '启用').map(c => field === 'subject' || field === 'grade' ? c[field] : spaceMap.get(c.learningSpaceId || '')?.[field]).filter((v): v is string => Boolean(v)))).map(value => ({ value, label: field === 'subject' ? subjectLabel(value) : field === 'semester' ? semesterLabel(value) : field === 'phase' ? phaseLabel(value) : value }));
  const selectedCourses = scope.courseIds?.length ? available.filter(c => scope.courseIds!.includes(c.id)) : available;
  const chapters = selectedCourses.flatMap(c => (c.curriculum || []).filter(n => !(c.curriculum || []).some(child => child.parentId === n.id)).map(n => ({ value: n.id, label: `${c.name} / ${n.name}` })));
  const uniqueChapters = Array.from(new Map(chapters.map(c => [c.value, c])).values());
  function close() { setOpen(false); setLink(''); const next = new URLSearchParams(params); next.delete('downloadJob'); setParams(next, { replace: true }); }
  function change(values: Partial<Scope>) { setError(''); setScope(current => ({ ...current, ...values, courseIds: undefined, lessonIds: undefined })); }
  async function create(id?: string) {
    setBusy(id || 'create'); setError('');
    try { await postData<Job>(id ? `/material-downloads/${id}/retry` : '/material-downloads', id ? {} : scope); await client.invalidateQueries({ queryKey: key }); message.success('下载任务已创建'); }
    catch (e) { setError(e instanceof Error ? e.message : '创建失败，请重试'); }
    finally { setBusy(''); }
  }
  async function download(job: Job) {
    setBusy(job.id); setError('');
    try {
      const response = await http.get<Blob>(`/material-downloads/${job.id}/archive`, { responseType: 'blob', timeout: 300000 });
      const url = URL.createObjectURL(response.data); const a = document.createElement('a'); a.href = url; a.download = `${subjectLabel(job.scope.subject)}讲义.zip`; a.click(); window.setTimeout(() => URL.revokeObjectURL(url), 60000);
    } catch (e) { setError(e instanceof Error ? e.message : '下载失败，请重试'); await jobs.refetch(); }
    finally { setBusy(''); }
  }
  async function copy(job: Job) {
    const url = new URL('/teacher-library', window.location.origin); url.searchParams.set('downloadJob', job.id); setLink(url.toString());
    try { await navigator.clipboard.writeText(url.toString()); message.success('领取链接已复制'); }
    catch { message.info('请复制下方领取链接'); }
  }
  return <>
    <Button aria-label="批量下载讲义" icon={<DownloadOutlined />} disabled={!canDownload} onClick={() => { if (!scope.subject) setScope({ subject: defaultSubject || options('subject')[0]?.value || '' }); setOpen(true); }}>批量下载讲义</Button>
    <Modal title="批量下载讲义" open={open} width="min(960px, calc(100vw - 24px))" onCancel={close} footer={<Button onClick={close}>关闭</Button>}>
      {!canDownload && <Alert type="warning" message="当前账号没有下载权限" />}
      {error && <Alert type="error" message={error} showIcon closable onClose={() => setError('')} />}
      {canDownload && !pickup && <Form layout="vertical" className="material-download-filters">
        {(['subject', 'grade', 'semester', 'phase'] as const).map((field, i) => <Form.Item key={field} label={['学科', '年级', '学期', '阶段'][i]}><Select aria-label={`下载${['学科', '年级', '学期', '阶段'][i]}`} value={scope[field] || undefined} allowClear={field !== 'subject'} options={options(field)} onChange={value => change({ [field]: value || '' })} /></Form.Item>)}
        <Form.Item label="课程"><Select aria-label="下载课程" mode="multiple" showSearch optionFilterProp="label" placeholder="全部可访问课程" value={scope.courseIds || []} options={available.map(c => ({ value: c.id, label: c.name }))} onChange={courseIds => setScope(current => ({ ...current, courseIds, lessonIds: undefined }))} /></Form.Item>
        <Form.Item label="章节"><Select aria-label="下载章节" mode="multiple" showSearch optionFilterProp="label" placeholder="全部章节" value={scope.lessonIds || []} options={uniqueChapters} onChange={lessonIds => setScope(current => ({ ...current, lessonIds }))} /></Form.Item>
      </Form>}
      {canDownload && !pickup && <Card size="small" className="material-download-summary">
        {quote.error ? <Alert type="warning" message={(quote.error as Error).message} action={<Button onClick={() => quote.refetch()}>重试</Button>} /> : <Space direction="vertical" style={{ width: '100%' }}>
          <Typography.Text>{quote.isFetching ? '正在核对材料…' : quote.data ? `${quote.data.count} 份已发布讲义 · ${sizeText(quote.data.size)}` : '请选择学科'}</Typography.Text>
          {quote.data && <Typography.Text type="secondary">{quote.data.courses.join('、')}</Typography.Text>}
          <Button type="primary" disabled={!quote.data || quote.isFetching || Boolean(busy)} loading={busy === 'create'} onClick={() => create()}>生成下载包</Button>
        </Space>}
      </Card>}
      {pickup && <Button style={{ marginBottom: 12 }} onClick={() => { const next = new URLSearchParams(params); next.delete('downloadJob'); setParams(next, { replace: true }); setScope({ subject: defaultSubject || options('subject')[0]?.value || '' }); }}>新建下载任务</Button>}
      <Space className="material-download-heading"><Typography.Text strong>我的下载任务</Typography.Text><Button icon={<ReloadOutlined />} loading={jobs.isFetching} onClick={() => jobs.refetch()}>刷新状态</Button></Space>
      {jobs.error ? <Alert type="error" message="下载任务加载失败" action={<Button onClick={() => jobs.refetch()}>重试</Button>} /> : jobs.isLoading ? <Typography.Text>加载中…</Typography.Text> : pickup && !jobs.data?.some(j => j.id === pickup) ? <Alert type="warning" message="该领取任务不存在或属于其他账号，请使用创建任务的账号登录" /> : null}
      {!jobs.isLoading && !jobs.error && jobs.data?.length === 0 && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无下载任务" />}
      {[...(jobs.data || [])].sort((a, b) => Number(b.id === pickup) - Number(a.id === pickup)).map(job => <Card key={job.id} size="small" className={`material-download-job${pickup === job.id ? ' material-download-highlight' : ''}`}>
        <Space wrap><Typography.Text strong>{subjectLabel(job.scope.subject)}讲义</Typography.Text><Tag color={job.status === '可下载' ? 'green' : job.status === '失败' ? 'red' : 'default'}>{job.status}</Tag><Typography.Text>{job.count} 份 · {sizeText(job.size)}</Typography.Text></Space>
        <div><Typography.Text type="secondary">创建于 {new Date(job.createdAt).toLocaleString()}{job.expiresAt ? ` · 有效期至 ${new Date(job.expiresAt).toLocaleString()}` : ''}</Typography.Text></div>
        {job.error && <Typography.Text type="danger">{job.error}</Typography.Text>}
        <Space wrap style={{ marginTop: 8 }}>{job.status === '可下载' && <Button aria-label="下载 ZIP" icon={<DownloadOutlined />} disabled={!canDownload || Boolean(busy)} loading={busy === job.id} onClick={() => download(job)}>下载 ZIP</Button>}{['失败', '已过期'].includes(job.status) && <Button disabled={!canDownload || Boolean(busy)} loading={busy === job.id} onClick={() => create(job.id)}>重新生成</Button>}<Button aria-label="复制电脑领取链接" icon={<CopyOutlined />} onClick={() => copy(job)}>复制电脑领取链接</Button></Space>
      </Card>)}
      {link && <Form layout="vertical" style={{ marginTop: 12 }}><Form.Item label="电脑领取链接（使用同一账号登录）"><Input aria-label="电脑领取链接" readOnly value={link} onFocus={e => e.target.select()} /></Form.Item></Form>}
    </Modal>
  </>;
}
