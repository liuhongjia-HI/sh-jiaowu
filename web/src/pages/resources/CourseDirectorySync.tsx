import { useEffect, useState } from 'react';
import { Alert, Button, Checkbox, Modal, Select, Space, Tag, Typography } from 'antd';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { postData } from '../../services/http';
import { curriculumLessonOptions, subjectsMatch } from '../../utils/curriculum';
import type { Course, LearningSpace } from '../../types/starline';

type Target = { courseId: string; courseName: string; added: string[]; updated: string[]; preserved: number; snapshot: string; error?: string; status?: string };
type Result = { targets: Target[] };

export function CourseDirectorySync({ source, courses, spaces, onClose, onMaterials, unitIds = [], initialTargetIds = [] }: { unitIds?: string[]; initialTargetIds?: string[]; source: Course; courses: Course[]; spaces: LearningSpace[]; onClose: () => void; onMaterials: (lessonId: string) => void }) {
  const client = useQueryClient();
  const [ids, setIDs] = useState<string[]>(initialTargetIds);
  const [preview, setPreview] = useState<Result>();
  const [result, setResult] = useState<Result>();
  const [operationError, setOperationError] = useState('');
  const lessonOptions = curriculumLessonOptions(source.curriculum);
  const [lessonId, setLessonId] = useState<string>(lessonOptions.length === 1 ? lessonOptions[0].value : '');
  const sourceSpace = spaces.find(space => space.id === source.learningSpaceId);
  const candidates = courses.filter(course => {
    const space = spaces.find(item => item.id === course.learningSpaceId);
    return course.id !== source.id && course.grade === source.grade && subjectsMatch(course.subject, source.subject) && space && sourceSpace && space.semester === sourceSpace.semester && space.phase === sourceSpace.phase;
  });
  const request = { sourceCourseId: source.id, targetCourseIds: ids, ...(unitIds.length ? { unitIds } : {}) };
  const refresh = () => Promise.all(['content', 'courses', 'course-families', 'teaching-plans'].map(key => client.invalidateQueries({ queryKey: [key] })));
  const check = useMutation({ mutationFn: () => postData<Result>('/courses/directory-sync-preview', request), onMutate: () => { setOperationError(''); setPreview(undefined); }, onSuccess: data => { setPreview(data); setResult(undefined); }, onError: (err: Error) => setOperationError(err.message) });
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
  return <Modal open title={`跨班型同步 · ${source.name}`} width={720} onCancel={() => !busy && onClose()} footer={<Space wrap><Select aria-label="同步讲义源课节" placeholder="选择讲义课节" style={{ width: 200 }} value={lessonId || undefined} options={lessonOptions} onChange={setLessonId} disabled={busy} /><Button onClick={() => onMaterials(lessonId)} disabled={busy || !lessonId}>同步该课节讲义</Button><Button onClick={onClose} disabled={busy}>关闭</Button>{result?.targets.some(target => target.error) ? <Button onClick={() => { setIDs(result.targets.filter(target => target.error).map(target => target.courseId)); setPreview(undefined); setResult(undefined); }}>仅重试失败目标</Button> : null}{!result && <Button type="primary" loading={busy} disabled={busy || !ids.length || !!preview && preview.targets.every(target => target.error)} onClick={() => preview ? execute.mutate() : check.mutate()}>{preview ? '确认同步目录' : '预览目录变更'}</Button>}</Space>}>
    <Alert type="info" message={unitIds.length ? "同步所选 Unit 及下级目录" : "同步整套课程目录"} description="新增缺少的章节，更新此前同步过的章节；保留目标原有额外章节及资料。讲义通过单独入口同步。" style={{ marginBottom: 16 }} />
    {operationError && <Alert type="error" showIcon message="操作结果未确认" description={`${operationError}。请重新预览后再确认同步。`} style={{ marginBottom: 16 }} />}
    <Checkbox.Group value={ids} disabled={busy || !!result} onChange={values => { setIDs(values.map(String)); setPreview(undefined); setOperationError(''); }} style={{ display: 'grid', gap: 8 }}>{candidates.map(course => <Checkbox key={course.id} value={course.id}>{course.name}</Checkbox>)}</Checkbox.Group>
    {!candidates.length && <Typography.Text type="secondary">没有同年级、学科、学期和阶段的其他课程。</Typography.Text>}
    {(result || preview)?.targets.map(target => <div key={target.courseId} style={{ marginTop: 16, padding: 12, border: '1px solid #e1e8ec', borderRadius: 8 }}><Typography.Text strong>{target.courseName || target.courseId}</Typography.Text>{target.error ? <Alert type="error" message={target.error} style={{ marginTop: 8 }} /> : <><div><Tag color={target.status ? 'green' : 'blue'}>{target.status || '待同步'}</Tag><span>新增 {target.added.length} · 更新 {target.updated.length} · 保留 {target.preserved}</span></div>{target.added.length > 0 && <div>新增：{target.added.join('、')}</div>}{target.updated.length > 0 && <div>更新：{target.updated.join('、')}</div>}</>}</div>)}
  </Modal>;
}
