import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getData } from '../services/http';
import type { Course, CurriculumNode, CurriculumPath, SubjectMetadata } from '../types/starline';

// 年级与学科的开设关系，与后端基础矩阵保持一致。
// 系统设置里的学科元数据才是下拉的权威来源：启用的学科会出现在课程方案、年级目录等选项中。
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

export function levelsForGradeSubject(grade?: string, subject?: string, catalog?: string[]): string[] {
  const index = gradeIndex(grade);
  if (index < 0 || !subject || !subjectsForGrade(grade, catalog).some((item) => subjectsMatch(item, subject))) return [];
  if (index <= 3) return ['S'];
  if (index === 4) {
    if (['数学', '英文', '语文'].includes(subject)) return ['S', 'S+', 'H'];
    if (subject === '地理' || subject === '历史') return ['S', 'S+'];
    return ['S'];
  }
  if (index === 5) return ['数学', '英文', '语文'].includes(subject) ? ['S', 'S+', 'H'] : ['S', 'S+'];
  if (['数学', '英文'].includes(subject)) return ['S', 'S+', 'H', 'H+'];
  if (index === 7 && subject === '物理') return ['S'];
  if (index === 8 && subject === '化学') return ['S', 'S+'];
  return ['S', 'S+', 'H'];
}

export function levelOptions(grade?: string, subject?: string, catalog?: string[]) {
  return levelsForGradeSubject(grade, subject, catalog).map((level) => ({ label: level, value: level }));
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

// 该年级实际开设的学科。系统设置启用的学科是下拉权威来源；
// 未拉到元数据时退回基础矩阵，避免页面空着。
export function subjectsForGrade(grade?: string, catalog?: string[]): string[] {
  const matrix = SUBJECTS_BY_GRADE[grade || ''] || ALL_SUBJECTS;
  if (!catalog) return matrix;
  const listed = matrix.filter((subject) => catalog.some((item) => subjectsMatch(item, subject)));
  for (const subject of catalog) {
    if (ALL_SUBJECTS.some((item) => subjectsMatch(item, subject))) continue;
    if (listed.some((item) => subjectsMatch(item, subject))) continue;
    listed.push(subject);
  }
  return listed;
}

export function enabledSubjectNames(subjects?: Array<Pick<SubjectMetadata, 'name' | 'status'>>): string[] | undefined {
  if (!subjects) return undefined;
  return subjects.filter((item) => item.status === '启用').map((item) => item.name).filter(Boolean);
}

export function useSubjectCatalog() {
  const query = useQuery({
    queryKey: ['subjects'],
    queryFn: () => getData<SubjectMetadata[]>('/subjects')
  });
  return useMemo(() => enabledSubjectNames(query.data), [query.data]);
}

export function gradeOptions() {
  return GRADES.map((grade) => ({ label: grade, value: grade }));
}

const SUBJECT_ENGLISH_NAMES: Record<string, string> = {
  英文: 'English',
  英语: 'English',
  English: 'English',
  数学: 'Mathematics',
  Math: 'Mathematics',
  Mathematics: 'Mathematics',
  语文: 'Chinese',
  Chinese: 'Chinese',
  科学: 'Science',
  Science: 'Science',
  综合科学: 'Integrated Science',
  'Integrated Science': 'Integrated Science',
  地理: 'Geography',
  Geography: 'Geography',
  历史: 'History',
  History: 'History',
  物理: 'Physics',
  Physics: 'Physics',
  化学: 'Chemistry',
  Chemistry: 'Chemistry'
};

const SUBJECT_KEYS: Record<string, string> = {
  英文: 'english',
  英语: 'english',
  English: 'english',
  数学: 'math',
  Math: 'math',
  Mathematics: 'math',
  语文: 'chinese',
  Chinese: 'chinese',
  科学: 'science',
  Science: 'science',
  综合科学: 'integrated-science',
  'Integrated Science': 'integrated-science',
  地理: 'geography',
  Geography: 'geography',
  历史: 'history',
  History: 'history',
  物理: 'physics',
  Physics: 'physics',
  化学: 'chemistry',
  Chemistry: 'chemistry'
};

export function subjectLabel(subject?: string) {
  const name = (subject || '').trim();
  return SUBJECT_ENGLISH_NAMES[name] || name;
}

export function subjectsMatch(left?: string, right?: string) {
  const first = (left || '').trim();
  const second = (right || '').trim();
  if (first === second) return true;
  if (!first || !second) return false;
  return (SUBJECT_KEYS[first] || first.toLowerCase()) === (SUBJECT_KEYS[second] || second.toLowerCase());
}

export function subjectOptions(grade?: string, catalog?: string[]) {
  return subjectsForGrade(grade, catalog).map((subject) => ({ label: subjectLabel(subject), value: subject }));
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
  return `${space.grade} · ${subjectLabel(space.subject)} · ${semesterLabel(space.semester)} · ${phaseLabel(space.phase)} · ${space.level || 'S'}`;
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

const CURRICULUM_TYPE_LABELS: Record<CurriculumNode['type'], string> = {
  unit: 'Unit',
  chapter: 'Chapter',
  lesson: 'Lesson'
};

export function curriculumNodeOrder(node?: Pick<CurriculumNode, 'sortOrder'> | null) {
  const order = Number(node?.sortOrder);
  return Number.isFinite(order) && order >= 1 ? Math.floor(order) : 1;
}

function curriculumParentKey(parentId?: string) {
  return parentId || '';
}

export function curriculumSiblings<T extends Pick<CurriculumNode, 'type' | 'parentId'>>(
  nodes: T[],
  type: CurriculumNode['type'],
  parentId?: string
) {
  const parent = curriculumParentKey(parentId);
  return nodes.filter((node) => node.type === type && curriculumParentKey(node.parentId) === parent);
}

// Chapter / Lesson 序号按父节点重置：Unit2 下第一章仍是 Chapter 1，而不是接着 Unit1 往后排。
export function nextCurriculumSortOrder(
  nodes: Pick<CurriculumNode, 'type' | 'parentId' | 'sortOrder'>[],
  type: CurriculumNode['type'],
  parentId?: string
) {
  return curriculumSiblings(nodes, type, parentId).reduce((max, node) => Math.max(max, curriculumNodeOrder(node)), 0) + 1;
}

export function prepareCurriculumForSave(nodes: CurriculumNode[] | undefined): CurriculumNode[] {
  const assigned: CurriculumNode[] = [];
  return (nodes ?? []).map((node, index) => {
    const explicit = Number(node.sortOrder);
    const sortOrder = Number.isFinite(explicit) && explicit >= 1
      ? Math.floor(explicit)
      : nextCurriculumSortOrder(assigned, node.type, node.parentId);
    const next = {
      ...node,
      id: node.id || `node-${Date.now()}-${index}`,
      sortOrder
    };
    assigned.push(next);
    return next;
  });
}

// Unit / Chapter 名称经常留空或只填类型名，下拉和目录需要把序号带上才能区分课节。
export function formatCurriculumNodeLabel(
  node: Pick<CurriculumNode, 'type' | 'name' | 'sortOrder'> | undefined,
  fallbackType: CurriculumNode['type']
) {
  const typeLabel = CURRICULUM_TYPE_LABELS[node?.type || fallbackType];
  const numbered = `${typeLabel} ${curriculumNodeOrder(node)}`;
  const name = (node?.name || '').trim();
  if (!name || name.toLowerCase() === typeLabel.toLowerCase()) return numbered;
  const numberedPrefix = new RegExp(`^${typeLabel}\\s*${curriculumNodeOrder(node)}\\b`, 'i');
  if (numberedPrefix.test(name)) return name;
  return `${numbered} · ${name}`;
}

export function formatCurriculumLessonLabel(nodes: CurriculumNode[] | undefined, lessonId?: string) {
  if (!nodes?.length || !lessonId) return '';
  const byID = new Map(nodes.map((node) => [node.id, node]));
  const leaf = byID.get(lessonId);
  if (!leaf) return '';
  const ancestors: CurriculumNode[] = [];
  for (let current = byID.get(leaf.parentId || ''); current; current = byID.get(current.parentId || '')) {
    ancestors.unshift(current);
  }
  const parts = ancestors.map((node) => formatCurriculumNodeLabel(node, node.type));
  const leafName = (leaf.name || '').trim();
  parts.push(leaf.type === 'lesson' ? (leafName || formatCurriculumNodeLabel(leaf, 'lesson')) : formatCurriculumNodeLabel(leaf, leaf.type));
  return parts.filter(Boolean).join(' · ');
}

export function curriculumLessonOptions(nodes: CurriculumNode[] | undefined) {
  const list = nodes ?? [];
  const byID = new Map(list.map((node) => [node.id, node]));
  const leaves = list.filter((node) => node.type === 'lesson' || (node.type === 'chapter' && !list.some((child) => child.parentId === node.id)));
  const sortKey = (node: CurriculumNode) => {
    const orders = [curriculumNodeOrder(node)];
    for (let current = byID.get(node.parentId || ''); current; current = byID.get(current.parentId || '')) {
      orders.unshift(curriculumNodeOrder(current));
    }
    while (orders.length < 3) orders.unshift(0);
    return orders;
  };
  return [...leaves]
    .sort((left, right) => {
      const a = sortKey(left);
      const b = sortKey(right);
      for (let index = 0; index < a.length; index += 1) {
        if (a[index] !== b[index]) return a[index] - b[index];
      }
      return left.id.localeCompare(right.id);
    })
    .map((lesson) => ({ value: lesson.id, label: formatCurriculumLessonLabel(list, lesson.id) || lesson.name }));
}

export function formatResourceCurriculumLabel(
  row: { lessonId?: string; curriculum?: CurriculumPath },
  course?: Pick<Course, 'curriculum'>
) {
  const fromCourse = formatCurriculumLessonLabel(course?.curriculum, row.lessonId);
  if (fromCourse) return fromCourse;
  const parts = row.curriculum ? [row.curriculum.unit, row.curriculum.chapter, row.curriculum.lesson].filter((item) => item && item.trim()) : [];
  return parts.length ? parts.join(' · ') : '—';
}
