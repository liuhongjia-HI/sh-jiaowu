import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Empty, Input, Modal, Select, Skeleton, Space, Spin, Table, Tag, Tree, Typography, message } from 'antd';
import type { DataNode } from 'antd/es/tree';
import { BookOutlined, DownloadOutlined, EyeOutlined, HistoryOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { getData, http, postData } from '../services/http';
import type { CurrentUser, Material, TeacherLibraryData } from '../types/starline';
import { subjectLabel, semesterLabel, phaseLabel } from '../utils/curriculum';

const pathText = (m: Material) => [m.curriculum?.unit, m.curriculum?.chapter, m.curriculum?.lesson].filter(Boolean).join(' / ');
const typeNames: Record<string, string> = { HD: '讲义', Blank: '空白讲义', HW: '作业', Exam: '试卷', Special: '专题' };

export default function TeacherLibrary({ user }: { user: CurrentUser }) {
  const [params, setParams] = useSearchParams();
  const client = useQueryClient();
  const query = useQuery({ queryKey: ['teacher-library', user.userId], queryFn: () => getData<TeacherLibraryData>('/teacher/library'), refetchOnWindowFocus: true,
    refetchInterval: current => {
      const selected = current.state.data?.materials.find(m => m.id === params.get('material'));
      return selected && ['待转换', '转换中'].includes(selected.previewStatus || '') ? 2500 : false;
    } });
  const data = query.data;
  const [url, setUrl] = useState('');
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState('');
  const [retry, setRetry] = useState(0);
  const [downloading, setDownloading] = useState(false);
  const [historyError, setHistoryError] = useState('');
  const courseId = params.get('course') || '';
  const lessonId = params.get('lesson') || '';
  const materialId = params.get('material') || '';
  const keyword = params.get('q') || '';
  const recent = params.get('recent') === '1';
  const active = data?.materials.find(m => m.id === materialId);
  const patch = (values: Record<string, string>, replace = false) => {
    const next = new URLSearchParams(params);
    Object.entries(values).forEach(([key, value]) => value ? next.set(key, value) : next.delete(key));
    setParams(next, { replace });
  };
  const spaceById = useMemo(() => new Map((data?.spaces ?? []).map(s => [s.id, s])), [data?.spaces]);
  const courses = (data?.courses ?? []).filter(c => {
    const s = spaceById.get(c.learningSpaceId || '');
    return (!params.get('subject') || c.subject === params.get('subject')) && (!params.get('grade') || c.grade === params.get('grade')) && ['semester', 'phase', 'level'].every(k => !params.get(k) || s?.[k as 'semester' | 'phase' | 'level'] === params.get(k));
  });
  const allowedCourses = new Set(courses.map(c => c.id));
  const rows = (data?.materials ?? []).filter(m => allowedCourses.has(m.courseId || '') && (!courseId || m.courseId === courseId) && (!lessonId || m.lessonId === lessonId) && (!params.get('tag') || m.tagCode === params.get('tag')) && (!recent || data?.recentMaterialIds.includes(m.id)) && (!keyword.trim() || [m.title, m.fileName, m.course, pathText(m)].join(' ').toLocaleLowerCase().includes(keyword.trim().toLocaleLowerCase())));
  if (recent) rows.sort((a, b) => (data?.recentMaterialIds.indexOf(a.id) ?? 0) - (data?.recentMaterialIds.indexOf(b.id) ?? 0));
  const tree: DataNode[] = courses.map(c => {
    const nodes = c.curriculum || [];
    const children = (parentId: string): DataNode[] => nodes.filter(n => (n.parentId || '') === parentId).map(n => ({ key: `${c.id}:${n.id}`, title: n.name, children: children(n.id) }));
    return { key: c.id, title: c.name, children: children('') };
  });
  // A parent chapter includes all descendant lesson files.
  const displayRows = lessonId && courseId ? (() => {
    const course = courses.find(c => c.id === courseId);
    const descendants = new Set([lessonId]);
    let changed = true;
    while (changed) { changed = false; for (const n of course?.curriculum ?? []) if (descendants.has(n.parentId || '') && !descendants.has(n.id)) { descendants.add(n.id); changed = true; } }
    return (data?.materials ?? []).filter(m => m.courseId === courseId && descendants.has(m.lessonId) && allowedCourses.has(m.courseId || '') && (!params.get('tag') || m.tagCode === params.get('tag')) && (!recent || data?.recentMaterialIds.includes(m.id)) && (!keyword.trim() || [m.title, m.fileName, m.course, pathText(m)].join(' ').toLowerCase().includes(keyword.trim().toLowerCase())));
  })() : rows;

  useEffect(() => {
    let cancelled = false;
    let objectUrl = '';
    const controller = new AbortController();
    setUrl(''); setPreviewError(''); setHistoryError(''); setPreviewLoading(false);
    if (!active) return;
    if (active.previewStatus !== '可预览' || !active.previewUrl) return;
    setPreviewLoading(true);
    http.get<Blob>(active.previewUrl.replace(/^\/api/, ''), { responseType: 'blob', signal: controller.signal, timeout: 120000 }).then(async response => {
      if (cancelled) return;
      objectUrl = URL.createObjectURL(response.data); setUrl(objectUrl); setPreviewLoading(false);
      try {
        await postData(`/teacher/materials/${active.id}/view`, {});
        if (!cancelled) client.invalidateQueries({ queryKey: ['teacher-library', user.userId] });
      } catch { if (!cancelled) setHistoryError('讲义已打开，但最近查看记录保存失败。'); }
    }).catch(error => { if (!cancelled) { setPreviewError(error.message || '讲义打开失败，请重试。'); setPreviewLoading(false); } });
    return () => { cancelled = true; controller.abort(); if (objectUrl) URL.revokeObjectURL(objectUrl); };
  }, [active?.id, active?.previewUrl, active?.previewStatus, retry, client, user.userId]);

  async function download(m: Material) {
    if (!m.downloadUrl) return;
    setDownloading(true);
    try {
      const response = await http.get<Blob>(m.downloadUrl.replace(/^\/api/, ''), { responseType: 'blob', timeout: 120000 });
      const href = URL.createObjectURL(response.data); const link = document.createElement('a'); link.href = href; link.download = m.fileName || m.title; link.click(); window.setTimeout(() => URL.revokeObjectURL(href), 60000);
    } catch (error) { message.error(error instanceof Error ? error.message : '下载失败，请重试'); }
    finally { setDownloading(false); }
  }
  const options = (key: 'subject' | 'grade' | 'semester' | 'phase' | 'level') => Array.from(new Set((data?.spaces ?? []).map(s => s[key]).filter(Boolean))).map(value => ({ value, label: key === 'subject' ? subjectLabel(value) : key === 'semester' ? semesterLabel(value) : key === 'phase' ? phaseLabel(value) : value }));
  const currentCourse = data?.courses.find(c => c.id === courseId);
  const scopeDescription = data?.policy ? [ ...(data.policy.scopes || []).map(s => `${subjectLabel(s.subject)} · ${s.grade || '全部年级'}`), ...(data.policy.spaceIds || []).map(id => spaceById.get(id)?.name || '指定学习空间') ].join('；') : (data?.spaces ?? []).map(s => s.name).join('；');
  const siblings = active ? (data?.materials ?? []).filter(m => m.courseId === active.courseId && m.lessonId === active.lessonId) : [];
  const page = Math.max(1, Number(params.get('page')) || 1);
  const pageSize = 12;
  const effectivePage = Math.min(page, Math.max(1, Math.ceil(displayRows.length / pageSize)));
  return <div className="page-stack teacher-library">
    <div className="page-heading"><div><Typography.Title level={3}>我的讲义</Typography.Title><Typography.Text type="secondary">按课程查找讲义，打开后可在同一课节的文件间切换。</Typography.Text></div><Button icon={<ReloadOutlined />} loading={query.isFetching} onClick={() => query.refetch()}>刷新资料</Button></div>
    {query.isLoading ? <Skeleton active /> : query.error ? <Alert type="error" message="资料加载失败" description="请检查网络后重试。" action={<Button onClick={() => query.refetch()}>重试</Button>} /> : <>
      <Card size="small"><Typography.Text strong>查阅范围：</Typography.Text><Typography.Text>{scopeDescription || '尚未分配资料范围，请联系管理员。'}</Typography.Text></Card>
      <Card size="small"><Space wrap>
        <Input.Search aria-label="搜索讲义" placeholder="搜索课程、课节、讲义或文件名" value={keyword} onChange={e => patch({ q: e.target.value, page: '' }, true)} style={{ width: 320 }} allowClear />
        {(['subject', 'grade', 'semester', 'phase', 'level'] as const).map((key, i) => <Select key={key} aria-label={['学科', '年级', '学期', '阶段', '班型'][i]} placeholder={['学科', '年级', '学期', '阶段', '班型'][i]} allowClear value={params.get(key) || (options(key).length === 1 ? options(key)[0].value : undefined)} options={options(key)} onChange={value => patch({ [key]: value || '', course: '', lesson: '', page: '' })} style={{ minWidth: 110 }} />)}
        <Select aria-label="资料类型" placeholder="资料类型" value={params.get('tag') || undefined} allowClear options={Object.entries(typeNames).map(([value, label]) => ({ value, label }))} onChange={v => patch({ tag: v || '', page: '' })} style={{ width: 120 }} />
        <Button onClick={() => setParams({})}>重置</Button>
      </Space></Card>
      <div className="teacher-library-layout">
        <Card className="teacher-library-directory" title={<Space><BookOutlined />课程目录</Space>} size="small">
          <Space direction="vertical" style={{ width: '100%' }}><Button block type={!courseId && !recent ? 'primary' : 'default'} onClick={() => patch({ course: '', lesson: '', recent: '', page: '' })}>全部讲义</Button><Button block icon={<HistoryOutlined />} type={recent ? 'primary' : 'default'} onClick={() => patch({ course: '', lesson: '', recent: '1', page: '' })}>最近查看</Button></Space>
          {tree.length ? <Tree key={[params.get('subject'), params.get('grade'), params.get('semester'), params.get('phase'), params.get('level'), courseId].join('|')} blockNode treeData={tree} defaultExpandParent defaultExpandedKeys={courseId ? [courseId, ...(lessonId ? [`${courseId}:${lessonId}`] : [])] : []} selectedKeys={courseId ? [lessonId ? `${courseId}:${lessonId}` : courseId] : []} onSelect={(keys) => { const key = String(keys[0] || ''); const c = courses.find(item => key === item.id || key.startsWith(`${item.id}:`)); patch({ course: c?.id || '', lesson: c && key !== c.id ? key.slice(c.id.length + 1) : '', recent: '', page: '' }); }} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前范围暂无课程" />}
        </Card>
        <Card title={recent ? '最近查看' : currentCourse?.name || '全部讲义'} extra={<Typography.Text type="secondary">{displayRows.length} 份资料</Typography.Text>}>
          <Table<Material> rowKey="id" dataSource={displayRows} pagination={{ current: effectivePage, pageSize, showSizeChanger: false, onChange: p => patch({ page: String(p) }) }} locale={{ emptyText: <Empty description={keyword || courseId || params.get('tag') ? '当前条件下没有讲义，可调整搜索或筛选条件。' : recent ? '打开讲义后，会在这里留下最近查看记录。' : '负责范围内暂无可查阅的讲义，请联系资料负责人。'} /> }} columns={[
            { title: '讲义', key: 'name', render: (_, m) => <div><Button type="link" style={{ padding: 0, height: 'auto', whiteSpace: 'normal', textAlign: 'left' }} onClick={() => patch({ material: m.id })}>{m.title}</Button><div><Typography.Text type="secondary">{m.fileName}</Typography.Text></div><Tag>{typeNames[m.tagCode || ''] || m.tagCode || m.fileType}</Tag>{m.status !== '启用' && <Tag color="orange">{m.status}</Tag>}</div> },
            { title: '所属课程 / 章节', render: (_, m) => <div>{m.course}<div><Typography.Text type="secondary">{pathText(m) || '课程资料'}</Typography.Text></div></div> },
            { title: '更新时间', dataIndex: 'updatedAt', width: 160, render: (value: string, m: Material) => value || m.createdAt || '—' },
            { title: '操作', width: 155, render: (_, m) => <Space><Button icon={<EyeOutlined />} onClick={() => patch({ material: m.id })}>查看</Button>{data?.canDownload && m.downloadUrl && <Button aria-label={`下载 ${m.title}`} icon={<DownloadOutlined />} loading={downloading} onClick={() => download(m)} />}</Space> }
          ]} />
        </Card>
      </div>
    </>}
    <Modal open={Boolean(materialId)} width="92vw" style={{ top: 24 }} title={active ? <div>{active.title}<div style={{ fontSize: 13, fontWeight: 400 }}>{active.course} / {pathText(active)}</div></div> : '查看讲义'} onCancel={() => patch({ material: '' })} footer={<Space>{active && data?.canDownload && active.downloadUrl && <Button icon={<DownloadOutlined />} loading={downloading} onClick={() => download(active)}>下载原文件</Button>}<Button onClick={() => patch({ material: '' })}>返回讲义列表</Button></Space>}>
      {query.isLoading ? <Skeleton active /> : !active ? <Alert type="warning" message="资料不存在或已不在负责范围内" description="请返回列表，刷新资料后重试。" /> : <>
        {siblings.length > 1 && <Space wrap style={{ marginBottom: 12 }}><Typography.Text>同课节资料</Typography.Text><Select aria-label="切换讲义" showSearch optionFilterProp="label" style={{ width: 500 }} value={materialId} options={siblings.map(m => ({ value: m.id, label: `${typeNames[m.tagCode || ''] || '资料'} · ${m.fileName || m.title}` }))} onChange={id => patch({ material: id }, true)} /><Button disabled={siblings.findIndex(m => m.id === materialId) <= 0} onClick={() => patch({ material: siblings[siblings.findIndex(m => m.id === materialId) - 1].id }, true)}>上一份</Button><Button disabled={siblings.findIndex(m => m.id === materialId) >= siblings.length - 1} onClick={() => patch({ material: siblings[siblings.findIndex(m => m.id === materialId) + 1].id }, true)}>下一份</Button></Space>}
        {historyError && <Alert type="warning" message={historyError} />}
        {previewLoading ? <div className="teacher-preview-placeholder"><Spin tip="讲义加载中，大文件可能需要一些时间…" size="large"><div style={{ minWidth: 400, minHeight: 100 }} /></Spin></div> : previewError ? <Alert type="error" message={previewError} action={<Button onClick={() => setRetry(n => n + 1)}>重试</Button>} /> : url ? <iframe title={`讲义预览：${active.title}`} src={url} className="teacher-preview-frame" /> : <Alert type="info" message={active.previewStatus === '转换失败' ? '讲义预览生成失败' : '讲义预览尚未准备好'} description={active.previewStatus === '转换失败' ? '请联系资料负责人重新生成预览。' : (['待转换', '转换中'].includes(active.previewStatus || '') ? '正在生成预览，完成后会自动打开。' : '请联系资料负责人检查文件，或刷新状态后重试。')} action={<Button onClick={() => query.refetch()}>刷新状态</Button>} />}
      </>}
    </Modal>
  </div>;
}
