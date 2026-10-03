import type { Course, Student, Teacher } from '../types/starline';
import { subjectsMatch } from './curriculum';

export function coursesForTeacher(courses: Course[], teacher?: Teacher) {
  if (!teacher) return [];
  return courses.filter(course => course.status !== '停用' && course.status !== '草稿' && !!course.learningSpaceId && (teacher.learningSpaceIds ?? []).includes(course.learningSpaceId));
}

export function schedulingStudentOptions(students: Student[]) {
  const counts = new Map<string, number>();
  students.forEach(student => counts.set(student.name, (counts.get(student.name) ?? 0) + 1));
  return students.map(student => ({ value: student.id, label: (counts.get(student.name) ?? 0) > 1 ? `${student.name} · ${student.grade || '年级未设置'} · ${student.id.slice(-6)}` : student.name }));
}

export function filterTeachingCourses(courses: Course[], grade?: string, subject?: string) {
  return courses.filter(course => (!grade || course.grade === grade) && (!subject || subjectsMatch(course.subject, subject)));
}
