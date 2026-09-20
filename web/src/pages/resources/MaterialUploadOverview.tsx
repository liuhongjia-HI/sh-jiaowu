import { Alert, Button, Card, Drawer, Empty, Select, Skeleton, Space, Table, Tag, Typography } from 'antd';
import { DownloadOutlined, EyeOutlined, UploadOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { getData } from '../../services/http';
import type { Course, Material, MaterialOverview, MaterialOverviewLesson } from '../../types/starline';
import { formatResourceCurriculumLabel, phaseLabel, semesterLabel, subjectLabel } from '../../utils/curriculum';
import { materialTagOptions } from './ResourceDialogs';

type Selection = { grade: string; subject: string };
const gradeOrder = ['学前班', '一年级', '二年级', '三年级', '四年级', '五年级', '六年级', '七年级', '八年级', '九年级', '十年级', '十一年级', '十二年级', '高一', '高二', '高三'];
const collator = new Intl.Collator('zh-CN', { numeric: true });
const keyOf = (grade: string, subject: string) => JSON.stringify([grade, subject]);

function sortGrades(values: string[]) {
  return [...values].sort((left, right) => {
    const a = gradeOrder.indexOf(left), b = gradeOrder.indexOf(right);
    return (a < 0 ? 99 : a) - (b < 0 ? 99 : b) || collator.compare(left, right);
  });
}

function csvValue(value: unknown) {
  return `"${String(value ?? '').replace(/"/g, '""')}"`;
}

function exportOverview(rows: MaterialOverviewLesson[], courses: Course[]) {
  const header = ['年级', '学科', '学期', '阶段', '班型', '课程', '课节', '上传状态', '文件名', '资料标签', '上传人', '上传时间', '预览状态'];
  const body = rows.flatMap((row) => {
    const lesson = formatResourceCurriculumLabel(row, courses.find((course) => course.id === row.courseId));
    const files = (row.materials ?? []).length ? row.materials : [undefined];
    return files.map((item) => [row.grade, subjectLabel(row.subject), semesterLabel(row.semester) || row.semester, phaseLabel(row.phase) || row.phase, row.level || '未设置', row.courseName, lesson, item ? '已上传' : '待上传', item?.fileName || '', item?.tagCode || '', item?.ownerTeacherName || '', item?.createdAt || '', item?.previewStatus || '']);
  });
  const csv = '\ufeff' + [header, ...body].map((row) => row.map(csvValue).join(',')).join('\n');
  const link = document.createElement('a');
  link.href = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
  link.download = `讲义上传清单-${new Date().toISOString().slice(0, 10)}.csv`;
  link.click();
  URL.revokeObjectURL(link.href);
}

export function MaterialUploadOverview({ courses, canManage, onUpload, onOpenFile }: {
  courses: Course[];
  canManage: boolean;
  onUpload: (courseId: string, lessonId: string) => void;
  onOpenFile: (material: Material, download: boolean) => void;
}) {
  const [semester, setSemester] = useState<string>();
  const [phase, setPhase] = useState<string>();
  const [level, setLevel] = useState<string>();
  const [tagCode, setTagCode] = useState<string>();
  const [selection, setSelection] = useState<Selection | null>(null);
  const [missingOnly, setMissingOnly] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const params = Object.fromEntries(Object.entries({ semester, phase, level, tagCode }).filter(([, value]) => Boolean(value))) as Record<string, string>;
  const overview = useQuery({ queryKey: ['materials-overview', params], queryFn: () => getData<MaterialOverview>('/materials/overview', params) });
  const model = useMemo(() => {
    const data = overview.data;
    return {
      cells: new Map((data?.cells ?? []).map((cell) => [keyOf(cell.grade, cell.subject), cell])),
      grades: sortGrades(data?.grades ?? []),
      subjects: [...(data?.subjects ?? [])].sort(collator.compare)
    };
  }, [overview.data]);
  const selectedRows = (overview.data?.lessons ?? []).filter((row) => (!selection || row.grade === selection.grade && row.subject === selection.subject) && (!missingOnly || (row.materials ?? []).length === 0));
  const maxFiles = Math.max(1, ...(overview.data?.cells ?? []).map((cell) => cell.fileCount));
  const openDetails = (next?: Selection) => { setSelection(next ?? null); setMissingOnly(false); setDrawerOpen(true); };

  return <Card className="material-overview" title="讲义上传分布" extra={<Space><Button type="link" disabled={!overview.data?.lessons.length} onClick={() => openDetails()}>查看全部明细</Button><Button type="link" disabled={!overview.data?.lessons.length} onClick={() => exportOverview(overview.data?.lessons ?? [], courses)}>导出清单</Button></Space>}>
    <div className="material-overview-filters">
      <Space wrap>
        <Select allowClear placeholder="学期" value={semester} onChange={setSemester} options={(overview.data?.semesters ?? []).map((value) => ({ value, label: semesterLabel(value) || value }))} style={{ width: 130 }} />
        <Select allowClear placeholder="阶段" value={phase} onChange={setPhase} options={(overview.data?.phases ?? []).map((value) => ({ value, label: phaseLabel(value) || value }))} style={{ width: 120 }} />
        <Select allowClear placeholder="资料标签" value={tagCode} onChange={setTagCode} options={materialTagOptions} style={{ width: 140 }} />
      </Space>
      <Space wrap size={6} role="group" aria-label="讲义统计班型">
        {[undefined, ...(overview.data?.levels ?? [])].map((value) => <Button key={value ?? 'all'} size="small" type={level === value ? 'primary' : 'default'} onClick={() => setLevel(value)}>{value ? `${value}班` : '全部班型'}</Button>)}
      </Space>
    </div>
    {overview.isLoading ? <Skeleton active paragraph={{ rows: 3 }} /> : overview.error ? <Alert type="error" message="讲义上传统计加载失败" action={<Button onClick={() => overview.refetch()}>重试</Button>} /> : !overview.data?.lessons.length ? <Empty description="当前筛选范围暂无课程课节" /> : <>
      <div className="material-overview-summary">
        <span><strong>{overview.data.summary.fileCount}</strong> 份讲义</span>
        <span><strong>{overview.data.summary.coveredLessonCount}</strong> 个已覆盖课节</span>
        <button type="button" onClick={() => { setSelection(null); setMissingOnly(true); setDrawerOpen(true); }}><strong>{overview.data.summary.missingLessonCount}</strong> 个待上传课节</button>
      </div>
      <div className="opening-matrix-scroll">
        <table className="opening-matrix" style={{ minWidth: 86 + model.subjects.length * 112 }} aria-label="讲义上传文件数矩阵">
          <thead><tr><th scope="col">年级</th>{model.subjects.map((subject) => <th scope="col" key={subject}>{subjectLabel(subject)}</th>)}</tr></thead>
          <tbody>{model.grades.map((grade) => <tr key={grade}><th scope="row">{grade}</th>{model.subjects.map((subject) => {
            const cell = model.cells.get(keyOf(grade, subject));
            if (!cell) return <td key={subject}><span className="opening-matrix-empty" aria-label={`${grade} ${subjectLabel(subject)} 无课程`}>—</span></td>;
            const breakdown = (cell.levelCounts ?? []).map((item) => `${item.level || '未设置'} ${item.fileCount}`).join(' · ');
            return <td key={subject}><button type="button" className={`opening-matrix-cell${cell.fileCount === 0 ? ' material-matrix-missing' : ''}`} style={{ '--opening-intensity': `${8 + cell.fileCount / maxFiles * 30}%` } as CSSProperties} onClick={() => openDetails({ grade, subject })}>
              <strong>{cell.fileCount ? `${cell.fileCount} 份` : '待上传'}</strong><span>{breakdown || `${cell.missingLessonCount} 个课节`}</span>
            </button></td>;
          })}</tr>)}</tbody>
        </table>
      </div>
      <div className="opening-overview-footer"><Typography.Text type="secondary">点击格子查看课程与课节；文件数可相加，覆盖课节按课程去重</Typography.Text><span className="opening-matrix-legend">文件少 <i /> 多 · — 无课程</span></div>
    </>}
    <Drawer title={selection ? `${selection.grade} · ${subjectLabel(selection.subject)} · 讲义明细` : missingOnly ? '待上传课节' : '全部讲义明细'} open={drawerOpen} width="min(920px, 100vw)" onClose={() => { setDrawerOpen(false); setSelection(null); setMissingOnly(false); }} extra={<Space><Button type={missingOnly ? 'primary' : 'default'} onClick={() => setMissingOnly((value) => !value)}>只看待上传</Button><Button onClick={() => exportOverview(selectedRows, courses)}>导出当前明细</Button></Space>}>
      <Table<MaterialOverviewLesson> rowKey={(row) => `${row.courseId}:${row.lessonId}`} dataSource={selectedRows} pagination={{ pageSize: 10 }} columns={[
        { title: '课节', width: 260, render: (_value, row) => <div><strong>{formatResourceCurriculumLabel(row, courses.find((course) => course.id === row.courseId))}</strong><div><Typography.Text type="secondary">{row.courseName} · {row.level || '未设置班型'}</Typography.Text></div></div> },
        { title: '讲义文件', render: (_value, row) => (row.materials ?? []).length ? <Space direction="vertical" size={6}>{(row.materials ?? []).map((item) => <Space key={item.id} wrap><Tag>{item.tagCode || '未标签'}</Tag><span>{item.fileName || item.title}</span><Typography.Text type="secondary">{item.ownerTeacherName || '—'} · {item.createdAt || '—'}</Typography.Text><Button type="text" size="small" icon={<EyeOutlined />} disabled={item.previewStatus !== '可预览'} onClick={() => onOpenFile(item, false)} aria-label={`预览 ${item.fileName || item.title}`} /><Button type="text" size="small" icon={<DownloadOutlined />} onClick={() => onOpenFile(item, true)} aria-label={`下载 ${item.fileName || item.title}`} /></Space>)}</Space> : <Tag color="warning">待上传</Tag> },
        { title: '操作', width: 105, render: (_value, row) => canManage ? <Button type="link" icon={<UploadOutlined />} onClick={() => onUpload(row.courseId, row.lessonId)}>上传讲义</Button> : null }
      ]} />
    </Drawer>
  </Card>;
}
