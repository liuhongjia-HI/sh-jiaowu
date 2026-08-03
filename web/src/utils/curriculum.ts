// 年级与学科的开设关系，与后端保持一致。这里的学科名称是业务字典值，
// “综合科学”和“科学”是两个不同学科，不能合并为一个选项。
// 规则对应 learning-api/internal/infrastructure/store/memory.go 的 subjectAppliesToGrade。

export const GRADES = [
  '一年级', '二年级', '三年级', '四年级', '五年级',
  '六年级', '七年级', '八年级', '九年级'
];

export const ALL_SUBJECTS = ['数学', '英文', '语文', '综合科学', '科学', '地理', '历史', '物理', '化学'];

export const SUBJECTS_BY_GRADE: Record<string, string[]> = {
  一年级: ['数学', '英文', '语文', '综合科学'],
  二年级: ['数学', '英文', '语文', '综合科学'],
  三年级: ['数学', '英文', '语文', '综合科学'],
  四年级: ['数学', '英文', '语文', '科学', '地理'],
  五年级: ['数学', '英文', '语文', '科学', '地理'],
  六年级: ['数学', '英文', '语文', '科学', '历史'],
  七年级: ['数学', '英文', '语文', '科学', '历史'],
  八年级: ['数学', '英文', '语文', '科学', '地理', '物理'],
  九年级: ['数学', '英文', '语文', '科学', '历史', '物理', '化学']
};

export const DEFAULT_ACADEMIC_YEAR = '2025.2026学年';

// 学年以每年 7 月 1 日为分界：7 月至次年 6 月属于同一学年。
export function academicYearForDate(date: Date = new Date()): string {
  const year = date.getFullYear();
  const startYear = date.getMonth() >= 6 ? year : year - 1;
  return `${startYear}.${startYear + 1}学年`;
}

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

// 该年级实际开设的学科；年级未知时返回全部业务学科，兼容历史数据和后台自由筛选。
export function subjectsForGrade(grade?: string): string[] {
  return SUBJECTS_BY_GRADE[grade || ''] || ALL_SUBJECTS;
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
