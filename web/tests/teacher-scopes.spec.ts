import { test, expect } from '@playwright/test';
import type { LearningSpace, Teacher } from '../src/types/starline';

const spaces = [
  { id: 'e1', grade: '五年级', subject: 'English', level: 'S', status: '启用' },
  { id: 'e2', grade: '五年级', subject: 'English', level: 'H', status: '启用' },
  { id: 'm1', grade: '五年级', subject: 'Math', level: 'S', status: '启用' },
  { id: 'old', grade: '四年级', subject: 'English', level: 'S', status: '停用' }
] as LearningSpace[];

test.beforeEach(async ({ page }) => { await page.goto('/'); });

test('legacy partial and disabled grants survive without gaining other stages', async ({ page }) => {
  const teacher = { learningSpaceIds: ['e1', 'old'], teacherLibrary: { spaceIds: ['e1', 'old'], scopes: [], canDownload: false, canViewDrafts: false, canManageCourses: true } } as unknown as Teacher;
  const { form, result } = await page.evaluate(async ({ teacher, spaces }) => {
    const { teacherScopeFormValues, serializeTeacherScopes } = await import('/src/utils/teacherScopes.ts');
    const form = teacherScopeFormValues(teacher, spaces);
    return { form, result: serializeTeacherScopes(form, spaces) };
  }, { teacher, spaces });
  expect(form.teachingScopes).toEqual([]);
  expect(result.learningSpaceIds).toEqual(['e1', 'old']);
  expect(result.teacherLibrary).toEqual(teacher.teacherLibrary);
});

test('one teaching range follows into read access while extra read stays separate', async ({ page }) => {
  const values = ({ teachingScopes: [{ grade: '五年级', subject: '英文' }], followTeaching: true, teacherLibrary: { spaceIds: [], scopes: [{ grade: '五年级', subject: 'Math' }], canDownload: true, canViewDrafts: false, canManageCourses: false } });
  const result = await page.evaluate(async ({ values, spaces }) => {
    const { serializeTeacherScopes } = await import('/src/utils/teacherScopes.ts');
    return serializeTeacherScopes(values, spaces);
  }, { values, spaces });
  expect(result.learningSpaceIds).toEqual(['e1', 'e2']);
  expect(result.teacherLibrary.spaceIds).toEqual(['e1', 'e2']);
  expect(result.teacherLibrary.scopes).toEqual([{ grade: '五年级', subject: 'Math' }]);
  expect(result.learningSpaceIds).not.toContain('m1');
});

test('complete group is summarized and legacy separated reading stays separate', async ({ page }) => {
  const teacher = { learningSpaceIds: ['e1', 'e2'], teacherLibrary: { spaceIds: ['m1'], scopes: [], canDownload: true, canViewDrafts: true, canManageCourses: true } } as unknown as Teacher;
  const { split, form, result } = await page.evaluate(async ({ teacher, spaces }) => {
    const { splitTeachingScopes, teacherScopeFormValues, serializeTeacherScopes } = await import('/src/utils/teacherScopes.ts');
    const form = teacherScopeFormValues(teacher, spaces);
    return { split: splitTeachingScopes(['e1', 'e2'], spaces), form, result: serializeTeacherScopes(form, spaces) };
  }, { teacher, spaces });
  expect(split).toEqual({ teachingScopes: [{ grade: '五年级', subject: 'English' }], learningSpaceIds: [] });
  expect(form.followTeaching).toBe(false);
  expect(result.teacherLibrary.spaceIds).toEqual(['m1']);
});
