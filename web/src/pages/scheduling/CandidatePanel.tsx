import { Button, Space, Tag, Typography } from 'antd';
import type { ScheduleCandidate, Teacher } from '../../types/starline';
import { candidateLevel, candidateLevelMeta, minimumStudentCount, weekLabel } from './scheduling-utils';

type CandidatePanelProps = {
  candidate: ScheduleCandidate;
  teacher?: Teacher;
  selected: boolean;
  onPick: () => void;
  teacherText: (name: string, teacher?: Teacher) => string;
  studentText: (students: ScheduleCandidate['availableStudents']) => string;
};

/** 候选排课卡片只负责呈现和触发确认，查询与成班 mutation 由页面保留。 */
export function CandidatePanel({ candidate, teacher, selected, onPick, teacherText, studentText }: CandidatePanelProps) {
  const meta = candidateLevelMeta(candidateLevel(candidate));
  return (
    <div className={`schedule-candidate-card ${selected ? 'is-selected' : ''}`}>
      <div className="schedule-candidate-head">
        <strong>{weekLabel(candidate.dayOfWeek)} {candidate.startTime}-{candidate.endTime}</strong>
        <Tag color={meta.color}>{meta.label}</Tag>
      </div>
      <div className="schedule-candidate-meta">{candidate.subject} · {candidate.grade} · {candidate.classType}</div>
      <div className="schedule-candidate-meta">{teacherText(candidate.teacherName, teacher)}</div>
      <div className="schedule-candidate-meta">学生（{candidate.studentCount}/{candidate.capacity}）：{studentText(candidate.availableStudents)}</div>
      <Button type="primary" size="small" onClick={onPick}>确认成班</Button>
    </div>
  );
}

type CoordinationPanelProps = {
  candidates: ScheduleCandidate[];
  teacherById: Record<string, Teacher>;
  onCoordinate: (ownerType: 'teacher' | 'student', ownerId: string) => void;
  teacherText: (name: string, teacher?: Teacher) => string;
  studentText: (students: ScheduleCandidate['availableStudents']) => string;
  studentLabel: (student: ScheduleCandidate['availableStudents'][number]) => string;
};

export function CoordinationPanel({ candidates, teacherById, onCoordinate, teacherText, studentText, studentLabel }: CoordinationPanelProps) {
  const rows = [...candidates].sort((left, right) => right.studentCount - left.studentCount).slice(0, 6);
  return (
    <div className="schedule-coordination-list">
      {rows.map((candidate) => (
        <div className="schedule-coordination-item" key={candidate.id}>
          <div className="schedule-coordination-head">
            <strong>{weekLabel(candidate.dayOfWeek)} {candidate.startTime}-{candidate.endTime}</strong>
            <Tag color="orange">还差 {Math.max(minimumStudentCount(candidate.classType) - candidate.studentCount, 1)} 人成班</Tag>
            <Typography.Text type="secondary">{teacherText(candidate.teacherName, teacherById[candidate.teacherId])} · {candidate.subject}/{candidate.grade} · {candidate.classType}</Typography.Text>
          </div>
          <div className="schedule-coordination-body">
            <div>已可上（{candidate.studentCount}）：{studentText(candidate.availableStudents)}</div>
            <div className="schedule-coordination-missing">
              <span>待协调：</span>
              {candidate.missingStudents.length === 0 ? <Typography.Text type="secondary">该时段暂无其他同学科同年级学生</Typography.Text> : (
                <Space size={[8, 8]} wrap>{candidate.missingStudents.map((student) => <Button key={student.id} size="small" onClick={() => onCoordinate('student', student.id)}>协调 {studentLabel(student)} 时间</Button>)}</Space>
              )}
            </div>
            <Button type="link" size="small" style={{ paddingLeft: 0 }} onClick={() => onCoordinate('teacher', candidate.teacherId)}>调整 {candidate.teacherName} 可授课时间</Button>
          </div>
        </div>
      ))}
    </div>
  );
}
