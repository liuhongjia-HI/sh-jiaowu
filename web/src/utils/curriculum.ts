// 年级与学科的开设关系，与后端保持一致。综合科学和历史仅兼容旧数据，
// 不再出现在新建课程与学习空间的选项中。
// 规则对应 learning-api/internal/infrastructure/store/memory.go 的 subjectAppliesToGrade。

export const GRADES = [
  '一年级', '二年级', '三年级', '四年级', '五年级',
  '六年级', '七年级', '八年级', '九年级', '十年级',
  '十一年级', '十二年级'
];

export const ALL_SUBJECTS = ['数学', '英文', '语文', '科学', '地理', '物理', '化学'];

export const SUBJECTS_BY_GRADE: Record<string, string[]> = {
  一年级: ['数学', '英文', '语文', '科学'],
  二年级: ['数学', '英文', '语文', '科学'],
  三年级: ['数学', '英文', '语文', '科学'],
  四年级: ['数学', '英文', '语文', '科学'],
  五年级: ['数学', '英文', '语文', '科学', '地理'],
  六年级: ['数学', '英文', '语文', '科学', '地理'],
  七年级: ['数学', '英文', '语文', '科学', '地理'],
  八年级: ['数学', '英文', '语文', '科学', '地理', '物理'],
  九年级: ['数学', '英文', '语文', '科学', '地理', '物理', '化学'],
  十年级: ['数学', '英文', '语文', '科学', '地理', '物理', '化学'],
  十一年级: ['数学', '英文', '语文', '科学', '地理', '物理', '化学'],
  十二年级: ['数学', '英文', '语文', '科学', '地理', '物理', '化学']
};

export const LEARNING_LEVELS = ['S', 'S+', 'H', 'H+'];

export function levelsForGradeSubject(grade?: string, subject?: string): string[] {
  const index = gradeIndex(grade);
  if (index < 0 || !subject || !subjectsForGrade(grade).includes(subject)) return [];
  if (index <= 3) return ['S'];
  if (index === 4) {
    if (['数学', '英文', '语文'].includes(subject)) return ['S', 'S+', 'H'];
    if (subject === '地理') return ['S', 'S+'];
    return ['S'];
  }
  if (index === 5) return ['数学', '英文', '语文'].includes(subject) ? ['S', 'S+', 'H'] : ['S', 'S+'];
  if (['数学', '英文'].includes(subject)) return ['S', 'S+', 'H', 'H+'];
  if (index === 7 && subject === '物理') return ['S'];
  if (index === 8 && subject === '化学') return ['S', 'S+'];
  return ['S', 'S+', 'H'];
}

export function levelOptions(grade?: string, subject?: string) {
  return levelsForGradeSubject(grade, subject).map((level) => ({ label: level, value: level }));
}

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

export function formatLearningSpace(space: { grade: string; subject: string; semester: string; phase: string; level?: string; name?: string }) {
  return `${space.grade} · ${space.subject} · ${semesterLabel(space.semester)} · ${phaseLabel(space.phase)} · ${space.level || 'S'}`;
}

// 校历（系统设置 academicCalendar）里的一条学期记录，是学年下拉的唯一权威来源——
// 不要再从学习空间或套餐历史数据里凑学年选项，那些字段要么是纯展示、要么只反映
// “曾经建过的套餐”，都不等于“学校实际配置的学年”。
type AcademicCalendarTerm = { academicYear: string; semester?: string; startDate?: string; endDate?: string };

// 把系统设置里 academicCalendar 的原始 JSON 解析成学年下拉选项：去重、按年份倒序，
// 解析失败或未配置时返回空数组而不是抛错，调用方自行拼默认学年兜底。
export function academicYearsFromCalendar(raw?: string): string[] {
  if (!raw) return [];
  let terms: AcademicCalendarTerm[];
  try {
    const parsed = JSON.parse(raw);
    terms = Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
  const years = Array.from(new Set(terms.map((term) => term.academicYear).filter(Boolean)));
  return years.sort((a, b) => b.localeCompare(a));
}
