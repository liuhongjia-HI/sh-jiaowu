import type { LearningSpace, Teacher, TeacherLibraryPolicy } from '../types/starline';
import { subjectLabel, subjectsMatch } from './curriculum';

export type TeachingScope = { grade: string; subject: string };

// A partial legacy grant stays exact. Opening and saving the new form must not
// grant the other class types or stages in the same grade and subject.
export function splitTeachingScopes(ids: string[], spaces: LearningSpace[]) {
  const selected = new Set(ids);
  const teachingScopes: TeachingScope[] = [];
  const represented = new Set<string>();
  for (const space of spaces) {
    if (space.status === '停用' || represented.has(space.id) || !selected.has(space.id)) continue;
    const group = spaces.filter(item => item.status !== '停用' && item.grade === space.grade && subjectsMatch(item.subject, space.subject));
    if (group.length && group.every(item => selected.has(item.id))) {
      teachingScopes.push({ grade: space.grade, subject: space.subject });
      group.forEach(item => represented.add(item.id));
    }
  }
  return { teachingScopes, learningSpaceIds: ids.filter(id => !represented.has(id)) };
}

export function resolveTeachingSpaces(scopes: TeachingScope[], exactIds: string[], spaces: LearningSpace[]) {
  return [...new Set([...exactIds, ...spaces.filter(item => item.status !== '停用' && scopes.some(scope => scope.grade === item.grade && subjectsMatch(scope.subject, item.subject))).map(item => item.id)])];
}

export function teacherScopeFormValues(teacher: Teacher, spaces: LearningSpace[]) {
  const ids = teacher.learningSpaceIds ?? [];
  const policy: TeacherLibraryPolicy = teacher.teacherLibrary ?? { spaceIds: ids, scopes: [], canDownload: true, canManageCourses: true, canViewDrafts: true };
  const followTeaching = ids.every(id => {
    const space = spaces.find(item => item.id === id);
    return policy.spaceIds?.includes(id) || !!space && policy.scopes?.some(scope => subjectsMatch(scope.subject, space.subject) && (!scope.grade || scope.grade === space.grade));
  });
  return {
    ...splitTeachingScopes(ids, spaces), followTeaching,
    teacherLibrary: { ...policy, scopes: [...(policy.scopes ?? [])], spaceIds: (policy.spaceIds ?? []).filter(id => !followTeaching || !ids.includes(id)) }
  };
}

export function serializeTeacherScopes(values: { teachingScopes?: TeachingScope[]; learningSpaceIds?: string[]; followTeaching?: boolean; teacherLibrary: TeacherLibraryPolicy }, spaces: LearningSpace[]) {
  const learningSpaceIds = resolveTeachingSpaces(values.teachingScopes ?? [], values.learningSpaceIds ?? [], spaces);
  return { learningSpaceIds, teacherLibrary: { ...values.teacherLibrary, spaceIds: [...new Set([...(values.teacherLibrary.spaceIds ?? []), ...(values.followTeaching ? learningSpaceIds : [])])] } };
}

export function teachingScopeLabels(ids: string[], spaces: LearningSpace[]) {
  return [...new Set(ids.map(id => {
    const space = spaces.find(item => item.id === id);
    return space ? `${space.grade} · ${subjectLabel(space.subject)}` : '历史范围';
  }))];
}
