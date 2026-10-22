import { useEffect, useState } from 'react';
import { Alert, Button, Checkbox, Modal, Select, Space, Tag, Typography } from 'antd';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { postData } from '../../services/http';
import { curriculumLessonOptions, subjectsMatch } from '../../utils/curriculum';
import type { Course, LearningSpace } from '../../types/starline';

type Resolution = { targetNodeId: string; mergeNodeIds?: string[] };
type Conflict = { sourceNodeId: string; sourceLabel: string; candidates: { id: string; label: string }[]; resolved: boolean; allowNew?: boolean };
type Target = { conflicts?: Conflict[]; merged?: string[]; materialsAdded?: string[]; materialsUpdated?: string[]; materialsPreserved?: number; courseId: string; courseName: string; added: string[]; updated: string[]; preserved: number; snapshot: string; error?: string; status?: string };
type Result = { targets: Target[] };

export function CourseDirectorySync({ source, courses, spaces, onClose, onMaterials, unitIds = [], initialTargetIds = [] }: { unitIds?: string[]; initialTargetIds?: string[]; source: Course; courses: Course[]; spaces: LearningSpace[]; onClose: () => void; onMaterials: (lessonId: string) => void }) {
  const client = useQueryClient();
  const [ids, setIDs] = useState<string[]>(initialTargetIds);
  const [preview, setPreview] = useState<Result>();
  const [result, setResult] = useState<Result>();
  const [operationError, setOperationError] = useState('');
  const [includeMaterials, setIncludeMaterials] = useState(true);
  const [resolutions, setResolutions] = useState<Record<string, Record<string, Resolution>>>({});
  const [dirty, setDirty] = useState(false);
  const changeResolution = (targetId: string, nodeId: string, value: Resolution) => {
    setResolutions(current => ({ ...current, [targetId]: { ...current[targetId], [nodeId]: value } }));
    setDirty(true); setOperationError('');
  };
  const lessonOptions = curriculumLessonOptions(source.curriculum);
  const [lessonId, setLessonId] = useState<string>(lessonOptions.length === 1 ? lessonOptions[0].value : '');
  const sourceSpace = spaces.find(space => space.id === source.learningSpaceId);
  const candidates = courses.filter(course => {
    const space = spaces.find(item => item.id === course.learningSpaceId);
    return course.id !== source.id && course.grade === source.grade && subjectsMatch(course.subject, source.subject) && space && sourceSpace && space.semester === sourceSpace.semester && space.phase === sourceSpace.phase;
  });
  const request = { sourceCourseId: source.id, targetCourseIds: ids, ...(unitIds.length ? { unitIds } : {}), ...(!includeMaterials ? { includeMaterials: false } : {}), ...(Object.keys(resolutions).length ? { resolutions } : {}) };
  const refresh = () => Promise.all(['content', 'courses', 'courses-for-content-resources', 'course-families', 'teaching-plans'].map(key => client.invalidateQueries({ queryKey: [key] })));
  const check = useMutation({ mutationFn: () => postData<Result>('/courses/directory-sync-preview', request), onMutate: () => { setOperationError(''); setPreview(undefined); }, onSuccess: data => { setPreview(data); setDirty(false); setResult(undefined); }, onError: (err: Error) => setOperationError(err.message) });
  const execute = useMutation({ mutationFn: () => postData<Result>('/courses/directory-sync', { ...request, snapshots: Object.fromEntries((preview?.targets || []).map(target => [target.courseId, target.snapshot])) }), onMutate: () => setOperationError(''), onSuccess: async data => {
    setResult(data);
    await refresh();
  }, onError: async (err: Error) => { setOperationError(err.message); setPreview(undefined); await refresh(); } });
  useEffect(() => {
    if (!initialTargetIds.length) return;
    const timer = window.setTimeout(() => check.mutate(), 0);
    return () => window.clearTimeout(timer);
  }, []);
  const busy = check.isPending || execute.isPending;
  const unresolved = preview?.targets.some(target => target.conflicts?.some(conflict => !conflict.resolved));
  const canConfirm = !!preview && !dirty && !unresolved;
  return <Modal open title={`跨班型同步 · ${source.name}`} width={800} styles={{ body: { maxHeight: '65vh', overflowY: 'auto', overflowWrap: 'anywhere' } }} onCancel={() => !busy && onClose()} footer={<Space wrap><Select aria-label="同步讲义源课节" placeholder="选择讲义课节" style={{ width: 200 }} value={lessonId || undefined} options={lessonOptions} onChange={setLessonId} disabled={busy} /><Button onClick={() => onMaterials(lessonId)} disabled={busy || !lessonId}>同步该课节讲义</Button><Button onClick={onClose} disabled={busy}>关闭</Button>{result?.targets.some(target => target.error) ? <Button onClick={() => { setIDs(result.targets.filter(target => target.error).map(target => target.courseId)); setPreview(undefined); setResult(undefined); }}>仅重试失败目标</Button> : null}{!result && <Button type="primary" aria-label={canConfirm ? (includeMaterials ? '确认同步目录及讲义' : '确认同步目录') : '预览同步变更'} loading={busy} disabled={busy || !ids.length || !!preview && !dirty && preview.targets.every(target => target.error)} onClick={() => canConfirm ? execute.mutate() : check.mutate()}>{canConfirm ? (includeMaterials ? '确认同步目录及讲义' : '确认同步目录') : '预览同步变更'}</Button>}</Space>}>
    <Alert type="info" message={unitIds.length ? "同步所选 Unit 及下级目录" : "同步整套课程目录"} description="新增缺少的章节，更新此前同步过的章节；保留目标原有额外章节及资料。默认同步已发布讲义；更新此前同步的副本，保留目标自行上传的讲义。源端删除或撤回不会删除目标资料。教案、课后练习不随源课程复制。" style={{ marginBottom: 16 }} />
    <Checkbox checked={includeMaterials} disabled={busy || !!result} onChange={event => { setIncludeMaterials(event.target.checked); setDirty(true); }}>同时同步已发布讲义</Checkbox>
    {dirty && preview && <Alert type="warning" showIcon message="选择已变化，请重新预览后确认" style={{ marginTop: 12 }} />}
    {operationError && <Alert type="error" showIcon message="操作结果未确认" description={`${operationError}。请重新预览后再确认同步。`} style={{ marginBottom: 16 }} />}
    <Checkbox.Group value={ids} disabled={busy || !!result} onChange={values => { setIDs(values.map(String)); setPreview(undefined); setResolutions({}); setOperationError(''); }} style={{ display: 'grid', gap: 8 }}>{candidates.map(course => <Checkbox key={course.id} value={course.id}>{course.name}</Checkbox>)}</Checkbox.Group>
    {!candidates.length && <Typography.Text type="secondary">没有同年级、学科、学期和阶段的其他课程。</Typography.Text>}
    {(result || preview)?.targets.map(target => <div key={target.courseId} style={{ marginTop: 16, padding: 12, border: '1px solid #e1e8ec', borderRadius: 8 }}><Typography.Text strong>{target.courseName || target.courseId}</Typography.Text>{target.error ? <><Alert type="error" message={target.error} style={{ marginTop: 8 }} />{resolutions[target.courseId] && <Button size="small" style={{ marginTop: 8 }} onClick={() => { setResolutions(current => { const next = { ...current }; delete next[target.courseId]; return next; }); setPreview(undefined); setDirty(true); }}>重新选择目录对应关系</Button>}</> : <><div><Tag color={target.status ? 'green' : 'blue'}>{target.status || '待同步'}</Tag><span>新增 {target.added.length} · 更新 {target.updated.length} · 保留 {target.preserved}</span></div>{target.conflicts?.map(conflict => {
        const choice = resolutions[target.courseId]?.[conflict.sourceNodeId];
        return <div key={conflict.sourceNodeId} style={{ marginTop: 12 }}>
          <Typography.Text strong>目录对应：{conflict.sourceLabel}</Typography.Text>
          <Select aria-label={`对应目录 ${conflict.sourceNodeId}`} placeholder="请选择对应的目标目录" style={{ width: '100%', marginTop: 8 }} value={choice?.targetNodeId} disabled={busy || !!result} options={[...conflict.candidates.map(item => ({ value: item.id, label: item.label })), ...(conflict.allowNew ? [{ value: '__new__', label: '作为不同章节新增，保留原有目录' }] : [])]} onChange={value => changeResolution(target.courseId, conflict.sourceNodeId, { targetNodeId: value })} />
          {choice?.targetNodeId && choice.targetNodeId !== '__new__' && conflict.candidates.length > 1 && <div style={{ marginTop: 8 }}>
            <Typography.Text>需要合并的重复目录（关联资料会迁移到所选目录）：</Typography.Text>
            <Checkbox.Group value={choice.mergeNodeIds || []} disabled={busy || !!result} onChange={values => changeResolution(target.courseId, conflict.sourceNodeId, { ...choice, mergeNodeIds: values.map(String) })} style={{ display: 'grid', gap: 8, marginTop: 8 }} options={conflict.candidates.filter(item => item.id !== choice.targetNodeId).map(item => ({ value: item.id, label: item.label }))} />
          </div>}
        </div>;
      })}
      {!!target.merged?.length && <div style={{ marginTop: 8 }}>合并并迁移关联：{target.merged.join('；')}</div>}
      {includeMaterials && <div style={{ marginTop: 8 }}>讲义：新增 {target.materialsAdded?.length || 0} · 更新 {target.materialsUpdated?.length || 0} · 保留 {target.materialsPreserved || 0}</div>}
      {!!target.materialsAdded?.length && <div>新增讲义：{target.materialsAdded.join('、')}</div>}
      {!!target.materialsUpdated?.length && <div>更新讲义：{target.materialsUpdated.join('、')}</div>}
      {target.added.length > 0 && <div>新增：{target.added.join('、')}</div>}{target.updated.length > 0 && <div>更新：{target.updated.join('、')}</div>}</>}</div>)}
  </Modal>;
}
