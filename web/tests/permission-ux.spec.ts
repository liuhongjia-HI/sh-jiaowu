import { test, expect } from '@playwright/test';

const space = { id: 'e1', name: '五年级英语提升', grade: '五年级', subject: 'English', semester: 'S1', phase: 'Q1', level: 'S', status: '启用' };

test('课程开通关闭保护覆盖无变更、继续编辑、Esc、遮罩、日期与保存', async ({ page }) => {
  const student = { id: 'ux-student', name: '林同学', grade: '五年级', phone: '', accountStatus: '正常', learningStatus: '未开始', activeOpenings: [], openedPackages: [], openedPackageRefs: [], streakDays: 0, badgeCount: 0 };
  let savedCodes = ['handout'];
  let writes = 0;
  let releaseSave!: () => void;
  const saveGate = new Promise<void>(resolve => { releaseSave = resolve; });
  await page.addInitScript(() => localStorage.setItem('starline_admin_token', 'test-token'));
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = [];
    if (path.endsWith('/auth/me')) data = { userId: 'test', name: '管理员', roles: ['super_admin'] };
    else if (path.endsWith('/grants/direct') && route.request().method() === 'PUT') {
      writes++;
      savedCodes = route.request().postDataJSON().selections[0].contentTypeCodes;
      await saveGate;
      data = { studentName: student.name };
    } else if (path.endsWith('/grants/direct/default')) data = { startsAt: '2026-11-17', endsAt: '2027-01-15' };
    else if (path.endsWith('/students/ux-student')) data = {
      student, grants: [{ isDirect: true, permissionState: '有效', learningSpaceIds: ['e1'], contentTypes: savedCodes, packageId: 'direct', packageName: '单独开通' }],
      openingMatrix: [{ learningSpaceId: 'e1', name: space.name, subject: 'English', subjectId: 'english', subjectSortOrder: 1, content: ['course', 'handout', 'question', 'download'].map(contentTypeCode => ({ contentTypeCode, opened: savedCodes.includes(contentTypeCode), directOpened: savedCodes.includes(contentTypeCode), packageOpened: false, packageNames: [], packageGrants: [], items: [] })) }], permissions: {}, learningRecords: [], notices: [], logs: []
    };
    else if (path.endsWith('/students')) data = [student];
    else if (path.endsWith('/learning-spaces')) data = [space];
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/students');
  const open = () => page.getByRole('button', { name: '课程开通', exact: true }).click();
  const drawer = page.locator('.student-detail-drawer');
  const close = () => drawer.locator('.ant-drawer-close').click();
  const confirm = page.getByRole('dialog', { name: '课程开通变更尚未保存' });
  await open();
  await expect(drawer.getByRole('checkbox', { name: '讲义查看' })).toBeChecked();
  await close();
  await expect(drawer).toBeHidden();
  await expect(confirm).toHaveCount(0);
  await open();
  await drawer.getByRole('checkbox', { name: '习题练习' }).check();
  await close();
  await expect(confirm).toBeVisible();
  await expect(confirm.getByRole('button', { name: '继续编辑' })).toBeVisible();
  await expect(confirm).toHaveCSS('animation-name', 'none');
  await expect(confirm).toHaveCSS('transform', 'none');
  await confirm.screenshot({ path: '/tmp/starline-unsaved-close-dialog.png' });
  await page.screenshot({ path: '/tmp/starline-unsaved-close-protection.png' });
  await confirm.getByRole('button', { name: '继续编辑' }).click();
  await expect(drawer.getByRole('checkbox', { name: '习题练习' })).toBeChecked();
  await drawer.locator('.ant-drawer-close').focus();
  await page.keyboard.press('Escape');
  await expect(confirm).toBeVisible();
  await confirm.getByRole('button', { name: '继续编辑' }).click();
  await page.locator('.ant-drawer-mask').click({ position: { x: 10, y: 100 } });
  await expect(confirm).toBeVisible();
  await confirm.getByRole('button', { name: '放弃变更' }).click();
  await expect(drawer).toBeHidden();
  expect(writes).toBe(0);
  await open();
  await expect(drawer.getByRole('checkbox', { name: '习题练习' })).not.toBeChecked();
  await drawer.getByRole('textbox', { name: '结束日期' }).fill('2027-02-01');
  await close();
  await expect(confirm).toBeVisible();
  await confirm.getByRole('button', { name: '继续编辑' }).click();
  await drawer.getByRole('textbox', { name: '结束日期' }).fill('2027-01-15');
  await close();
  await expect(drawer).toBeHidden();
  await open();
  await drawer.getByRole('checkbox', { name: '习题练习' }).check();
  await drawer.getByRole('button', { name: /保存变更/ }).click();
  await expect.poll(() => writes).toBe(1);
  await close();
  await expect(page.getByText('正在保存课程开通变更，请稍候。')).toBeVisible();
  await expect(drawer).toBeVisible();
  await expect(confirm).toBeHidden();
  releaseSave();
  await page.mouse.move(0, 0);
  await expect(page.getByText('正在保存课程开通变更，请稍候。')).toBeHidden();
  await expect(drawer.getByText('暂无变更', { exact: true })).toBeVisible();
  await close();
  await expect(drawer).toBeHidden();
  await expect(confirm).toBeHidden();
});

test('教师范围分组选择保持历史权限和资料权限独立，保存原 ID', async ({ page }) => {
  const spaces = [space,
    { ...space, id: 'e2', name: '五年级英语进阶', semester: 'S2', phase: 'Q2', level: 'H' },
    { ...space, id: 'm1', name: '五年级数学基础', subject: 'Mathematics' },
    { ...space, id: 'old', name: '四年级英语旧范围', grade: '四年级', status: '停用' },
    { ...space, id: 'disabled', name: '停用未授权', status: '停用' }
  ];
  const teacher = { id: 'ux-teacher', name: '陈老师', phone: '13800000000', learningSpaceIds: ['e1', 'old', 'missing'], teacherLibrary: { spaceIds: ['m1'], scopes: [], canDownload: false, canViewDrafts: false, canManageCourses: false }, canUploadHandout: false, canUploadQuestion: false, canReview: false, remark: '', accountStatus: '正常', bindStatus: '待绑定', activeClassCount: 0 };
  let saved: any;
  await page.addInitScript(() => localStorage.setItem('starline_admin_token', 'test-token'));
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = [];
    if (path.endsWith('/auth/me')) data = { userId: 'test', name: '管理员', roles: ['super_admin'] };
    else if (path.endsWith('/teachers/ux-teacher') && route.request().method() === 'PUT') { saved = route.request().postDataJSON(); data = teacher; }
    else if (path.endsWith('/teachers')) data = [teacher];
    else if (path.endsWith('/learning-spaces')) data = spaces;
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.setViewportSize({ width: 1440, height: 1100 });
  await page.goto('/teachers');
  await page.getByRole('button', { name: '编辑', exact: true }).click();
  const drawer = page.getByRole('dialog', { name: '编辑教师' });
  await drawer.getByText(/指定授课班型与阶段/).click();
  const exact = drawer.getByRole('combobox', { name: '精确授课范围' });
  await exact.click();
  await expect(page.getByRole('option', { name: /S2.*第二学期.*Q2.*期末.*H/ })).toBeVisible();
  await expect(page.getByRole('option', { name: /停用未授权/ })).toHaveClass(/ant-select-item-option-disabled/);
  const secondTerm = page.getByRole('option', { name: /S2.*第二学期.*Q2.*期末.*H/ });
  await secondTerm.click();
  await secondTerm.click();
  await page.locator('.ant-select-dropdown:visible .rc-virtual-list-holder').evaluate(el => { el.scrollTop = 0; });
  await page.screenshot({ animations: 'disabled', path: '/tmp/starline-teacher-scope-options.png' });
  await drawer.screenshot({ animations: 'disabled', path: '/tmp/starline-teacher-scope-grouped.png' });
  await page.keyboard.press('Escape');
  await drawer.getByText(/额外资料查阅范围/).click();
  await drawer.locator('summary').filter({ hasText: '已选 3 项' }).click();
  await drawer.getByRole('heading', { name: '授课范围', exact: true }).click();
  await page.locator('.ant-select-dropdown:visible').waitFor({ state: 'hidden' });
  await page.screenshot({ animations: 'disabled', path: '/tmp/starline-teacher-scope-summary.png' });
  await page.setViewportSize({ width: 390, height: 844 });
  await exact.scrollIntoViewIfNeeded();
  expect(await drawer.locator('.ant-drawer-body').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
  await page.screenshot({ animations: 'disabled', path: '/tmp/starline-teacher-scope-narrow.png' });
  await drawer.getByRole('button', { name: /^保\s*存$/ }).click();
  await expect.poll(() => saved?.learningSpaceIds).toEqual(['e1', 'old', 'missing']);
  expect(saved.teacherLibrary).toEqual(teacher.teacherLibrary);
});
