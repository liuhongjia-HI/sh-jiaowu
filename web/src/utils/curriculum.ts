// 年级与学科的开设关系，与后端保持一致：
// 小学（一~六年级）只开设语文、数学、英语；初高中（七~十二年级）开设全部 9 个学科。
// 规则对应 learning-api/internal/infrastructure/store/memory.go 的 subjectAppliesToGrade。

export const GRADES = [
  '一年级', '二年级', '三年级', '四年级', '五年级', '六年级',
  '七年级', '八年级', '九年级', '十年级', '十一年级', '十二年级'
];

export const ALL_SUBJECTS = ['语文', '数学', '英语', '物理', '化学', '地理', '历史', '政治', '生物'];

export const ELEMENTARY_SUBJECTS = ['语文', '数学', '英语'];

export const DEFAULT_ACADEMIC_YEAR = '2025.2026学年';

export const DEFAULT_SEMESTERS = ['S1', 'S2'];

export const DEFAULT_PHASES = ['Q1', 'Q2'];

const SEMESTER_LABELS: Record<string, string> = {
  S1: '第一学期',
  S2: '第二学期'
};

const PHASE_LABELS: Record<string, string> = {
  Q1: '期中',
  Q2: '期末'
};

// 年级在 GRADES 中的下标，未知或为空返回 -1。
export function gradeIndex(grade?: string): number {
  if (!grade) return -1;
  return GRADES.indexOf(grade);
}

// 该年级实际开设的学科。下标 < 6（一~六年级）按小学处理，只返回语文/数学/英语；
// 其余年级或年级未知时返回全部学科。
export function subjectsForGrade(grade?: string): string[] {
  const index = gradeIndex(grade);
  if (index >= 0 && index < 6) return ELEMENTARY_SUBJECTS;
  return ALL_SUBJECTS;
}

export function gradeOptions() {
  return GRADES.map((grade) => ({ label: grade, value: grade }));
}

export function subjectOptions(grade?: string) {
  return subjectsForGrade(grade).map((subject) => ({ label: subject, value: subject }));
}

export function semesterLabel(value?: string) {
  if (!value) return '';
  const label = SEMESTER_LABELS[value];
  return label ? `${value} ${label}` : value;
}

export function phaseLabel(value?: string) {
  if (!value) return '';
  const label = PHASE_LABELS[value];
  return label ? `${value} ${label}` : value;
}

export function parseSemesterSetting(value?: string) {
  const parsed = (value || '')
    .split(/[\/,，、\s]+/)
    .map((item) => item.trim())
    .filter((item) => DEFAULT_SEMESTERS.includes(item));
  const unique = Array.from(new Set(parsed));
  return unique.length > 0 ? unique : DEFAULT_SEMESTERS;
}

export function semesterOptions(settingValue?: string) {
  return parseSemesterSetting(settingValue).map((value) => ({ label: semesterLabel(value), value }));
}

export function formatLearningSpace(space: { grade: string; subject: string; semester: string; phase: string; name?: string }) {
  return `${space.grade} · ${space.subject} · ${semesterLabel(space.semester)} · ${phaseLabel(space.phase)}`;
}
