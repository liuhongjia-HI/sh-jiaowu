import { test, expect } from '@playwright/test';

async function fixture(page: any, blocked = false) {
 const user = { userId: 'super', name: '管理员', roles: ['super_admin'] };
 let teacher: any = { id: 't1', name: '测试老师', phone: '13911223344', learningSpaceIds: [], learningSpaces: [], grades: [], subjects: [], accountStatus: '正常', bindStatus: '待绑定', activeClassCount: 3, canUploadHandout: false, canUploadQuestion: false, canReview: false, remark: '' };
 const writes: any[] = [];
 await page.addInitScript((user: any) => { localStorage.setItem('starline_admin_token', 'teacher-lifecycle-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
 await page.route('**/api/**', async (route: any) => {
  const request = route.request(); const path = new URL(request.url()).pathname.replace('/api', ''); let data: any = [];
  if (path === '/auth/me') data = user;
  if (path === '/teachers') data = teacher ? [teacher] : [];
  if (path === '/teachers/t1/status' && request.method() === 'PUT') { const body = request.postDataJSON(); writes.push({ method: 'PUT', body }); teacher = { ...teacher, ...body }; data = teacher; }
  if (path === '/teachers/t1' && request.method() === 'DELETE') {
   writes.push({ method: 'DELETE' });
   if (blocked) { await route.fulfill({ status: 400, json: { code: 400, message: '该教师关联排课记录，不能删除，请停用账号以保留记录' } }); return; }
   teacher = null;
  }
  await route.fulfill({ json: { code: 0, data } });
 });
 await page.goto('/teachers');
 return writes;
}

test('table exposes fixed actions and disable/enable keeps status payload minimal', async ({ page }, testInfo) => {
 await page.setViewportSize({ width: 1280, height: 800 });
 await page.addInitScript(() => localStorage.setItem('starline:list-view:teachers', 'table'));
 const writes = await fixture(page);
 const disable = page.getByRole('button', { name: '停用', exact: true });
 await disable.scrollIntoViewIfNeeded();
 await expect(disable).toBeInViewport();
 await page.screenshot({ path: testInfo.outputPath('teacher-table.png'), fullPage: true });
 await disable.click();
 let dialog = page.getByRole('dialog', { name: '停用教师账号', exact: true });
 await expect(dialog).toContainText('3 节未结束');
 await dialog.getByRole('button', { name: /取\s*消/ }).click();
 expect(writes).toHaveLength(0);
 await disable.click();
 await page.getByRole('dialog', { name: '停用教师账号', exact: true }).getByRole('button', { name: '确认停用' }).click();
 await expect(page.getByRole('button', { name: '启用', exact: true })).toBeVisible();
 expect(writes[0]).toEqual({ method: 'PUT', body: { accountStatus: '停用' } });
 await page.getByRole('button', { name: '启用', exact: true }).click();
 await page.getByRole('dialog', { name: '启用教师账号', exact: true }).getByRole('button', { name: '确认启用' }).click();
 await expect(disable).toBeVisible();
 expect(writes[1].body).toEqual({ accountStatus: '正常' });
});

test('card deletion requires confirmation and removes only selected teacher', async ({ page }) => {
 const writes = await fixture(page);
 await page.getByRole('button', { name: '删除', exact: true }).click();
 const dialog = page.getByRole('dialog', { name: '删除教师账号', exact: true });
 await expect(dialog).toContainText('测试老师'); await expect(dialog).toContainText('13911223344');
 await dialog.getByRole('button', { name: /取\s*消/ }).click(); expect(writes).toHaveLength(0);
 await page.getByRole('button', { name: '删除', exact: true }).click();
 await page.getByRole('dialog', { name: '删除教师账号', exact: true }).getByRole('button', { name: '确认删除' }).click();
 await expect(page.getByText('教师账号已删除', { exact: true })).toBeVisible();
 await expect(page.getByText('测试老师', { exact: true })).toHaveCount(0);
 expect(writes).toEqual([{ method: 'DELETE' }]);
});

test('association rejection retains account and displays actionable reason', async ({ page }) => {
 await fixture(page, true);
 await page.getByRole('button', { name: '删除', exact: true }).click();
 const dialog = page.getByRole('dialog', { name: '删除教师账号', exact: true });
 await dialog.getByRole('button', { name: '确认删除' }).click();
 await expect(page.getByText('该教师关联排课记录，不能删除，请停用账号以保留记录', { exact: true })).toBeVisible();
 await expect(dialog).toBeVisible();
 await dialog.getByRole('button', { name: /取\s*消/ }).click();
 await expect(page.getByText('测试老师', { exact: true })).toBeVisible();
});
