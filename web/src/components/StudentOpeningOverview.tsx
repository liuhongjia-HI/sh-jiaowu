import { Alert, Button, Card, Empty, Modal, Skeleton, Space, Table, Typography } from 'antd';
import { useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import type { Student } from '../types/starline';
import { subjectLabel } from '../utils/curriculum';

// undefined means all levels; an empty string means a missing historical level.
export type OpeningSelection = { grade: string; subject: string; level?: string };
export const openingLevelLabel = (level?: string) => level === undefined ? '全部班型' : level ? `${level}班` : '未设置班型';
export const openingSelectionLabel = (scope: OpeningSelection) => `${scope.grade} · ${subjectLabel(scope.subject)} · ${openingLevelLabel(scope.level)}`;
const keyOf = (grade: string, subject: string) => JSON.stringify([grade, subject]);
const gradeOrder = ['学前班', '一年级', '二年级', '三年级', '四年级', '五年级', '六年级', '七年级', '八年级', '九年级', '高一', '高二', '高三'];
const gradeRank = (grade: string) => gradeOrder.indexOf(({ 初一: '七年级', 初二: '八年级', 初三: '九年级' } as Record<string, string>)[grade] ?? grade);
const collator = new Intl.Collator('zh-CN', { numeric: true });

export function StudentOpeningOverview({ students, loading, error, onRetry, selection, level, onLevelChange, onSelect }: {
  students: Student[]; loading: boolean; error: boolean; onRetry: () => void;
  selection: OpeningSelection | null; level?: string; onLevelChange: (level?: string) => void;
  onSelect: (selection: OpeningSelection) => void;
}) {
  const [detailsOpen, setDetailsOpen] = useState(false);
  const model = useMemo(() => {
    const cells = new Map<string, { grade: string; subject: string; students: Set<string>; levels: Map<string, Set<string>> }>();
    const grades = new Set<string>(), subjects = new Set<string>(), levels = new Set<string>();
    for (const student of students) for (const scope of student.activeOpenings ?? []) {
      grades.add(scope.grade); subjects.add(scope.subject); levels.add(scope.level);
      const key = keyOf(scope.grade, scope.subject);
      const cell = cells.get(key) ?? { grade: scope.grade, subject: scope.subject, students: new Set<string>(), levels: new Map<string, Set<string>>() };
      cell.students.add(student.id);
      const ids = cell.levels.get(scope.level) ?? new Set<string>();
      ids.add(student.id); cell.levels.set(scope.level, ids); cells.set(key, cell);
    }
    const compareGrade = (a: string, b: string) => {
      const left = gradeRank(a), right = gradeRank(b);
      return (left < 0 ? 99 : left) - (right < 0 ? 99 : right) || collator.compare(a, b);
    };
    const details = [...cells.values()].flatMap(cell => [...cell.levels].map(([level, ids]) => ({ grade: cell.grade, subject: cell.subject, level, count: ids.size })))
      .sort((a, b) => compareGrade(a.grade, b.grade) || collator.compare(a.subject, b.subject) || collator.compare(a.level, b.level));
    return { cells, grades: [...grades].sort(compareGrade), subjects: [...subjects].sort(collator.compare), levels: [...levels].sort((a, b) => !a ? 1 : !b ? -1 : collator.compare(a, b)), details };
  }, [students]);
  // Keep the color scale stable while switching levels.
  const maxCount = Math.max(1, ...[...model.cells.values()].map(cell => cell.students.size));
  const visibleDetails = model.details.filter(row => level === undefined || row.level === level);
  return <Card className="opening-overview" title="当前开通分布" extra={<Button type="link" disabled={loading || error || !model.details.length} onClick={() => setDetailsOpen(true)}>查看明细</Button>}>
    {error ? <Alert type="error" message="开通分布加载失败" action={<Button onClick={onRetry}>重试</Button>} /> : loading ? <Skeleton active paragraph={{ rows: 3 }} /> : !model.cells.size ? <Empty description="暂无当前有效开通" /> : <>
      <div className="opening-overview-toolbar">
        <Space wrap size={6} role="group" aria-label="开通分布班型">
          {[undefined, ...model.levels].map(value => <Button key={value === undefined ? 'all' : `level-${value}`} size="small" type={level === value ? 'primary' : 'default'} aria-pressed={level === value} onClick={() => onLevelChange(value)}>{openingLevelLabel(value)}</Button>)}
        </Space>
        <Typography.Text type="secondary">全部可见学生 · {model.grades.length} 个年级 · {model.subjects.length} 门科目</Typography.Text>
      </div>
      <div className="opening-matrix-scroll">
        <table className="opening-matrix" style={{ minWidth: 86 + model.subjects.length * 112 }} aria-label="当前有效开通人数矩阵">
          <thead><tr><th scope="col">开通年级</th>{model.subjects.map(subject => <th scope="col" key={subject}>{subjectLabel(subject)}</th>)}</tr></thead>
          <tbody>{model.grades.map(grade => <tr key={grade}><th scope="row">{grade}</th>{model.subjects.map(subject => {
            const cell = model.cells.get(keyOf(grade, subject));
            const count = level === undefined ? cell?.students.size ?? 0 : cell?.levels.get(level)?.size ?? 0;
            const breakdown = model.levels.filter(value => level === undefined || level === value).flatMap(value => {
              const size = cell?.levels.get(value)?.size ?? 0;
              return size ? [`${value || '未设置'} ${size}`] : [];
            }).join(' · ');
            const selected = selection?.grade === grade && selection.subject === subject && selection.level === level;
            return <td key={subject}>{count ? <button type="button" className="opening-matrix-cell" aria-pressed={selected} aria-label={`${grade} ${subjectLabel(subject)} ${openingLevelLabel(level)} ${count}人`} style={{ '--opening-intensity': `${8 + count / maxCount * 30}%` } as CSSProperties} onClick={() => onSelect({ grade, subject, level })}>
              <strong>{count}</strong><span>{breakdown}</span>
            </button> : <span className="opening-matrix-empty" aria-label={`${grade} ${subjectLabel(subject)} 无有效开通`}>—</span>}</td>;
          })}</tr>)}</tbody>
        </table>
      </div>
      <div className="opening-overview-footer"><Typography.Text type="secondary">点击人数定位学生 · 各格人数不可直接相加</Typography.Text><span className="opening-matrix-legend">人数少 <i /> 多 · — 无有效开通</span></div>
    </>}
    <Modal title={`开通明细 · ${openingLevelLabel(level)}`} open={detailsOpen} onCancel={() => setDetailsOpen(false)} footer={null} width={760}>
      <Typography.Paragraph type="secondary">仅统计当前有效套餐和单独开通授权。同一学生在同一组合下去重，跨班型、跨科目人数可能重复。</Typography.Paragraph>
      <Table size="small" rowKey={row => JSON.stringify([row.grade, row.subject, row.level])} dataSource={visibleDetails} pagination={{ pageSize: 8, hideOnSinglePage: true }} scroll={{ x: 500 }} columns={[
        { title: '开通年级', dataIndex: 'grade' }, { title: '科目', dataIndex: 'subject', render: subjectLabel },
        { title: '班型', dataIndex: 'level', render: openingLevelLabel },
        { title: '有效开通人数', dataIndex: 'count', render: (count: number, row) => <Button type="link" onClick={() => { setDetailsOpen(false); onLevelChange(row.level); onSelect(row); }}>{count} 人 · 查看学生</Button> }
      ]} />
    </Modal>
  </Card>;
}
