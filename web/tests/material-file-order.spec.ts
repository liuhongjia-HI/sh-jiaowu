import { test, expect, type Page } from '@playwright/test';

const course = { id: 'c1', name: '五年级英语', grade: '五年级', subject: 'English', semester: 'S1', phase: 'Q1', learningSpaceId: 'e1', status: '启用', curriculum: [{ id: 'l1', name: '第一课', type: 'lesson', sortOrder: 1 }, { id: 'l2', name: '第二课', type: 'lesson', sortOrder: 2 }] };
const initial = [
  { id: 'm1', fileName: '阅读讲义.pdf', lessonId: 'l1', tagCode: 'HD', createdAt: '2026-11-01 09:12:30' },
  { id: 'm2', fileName: '课后练习.pdf', lessonId: 'l1', tagCode: 'HW', createdAt: '2026-11-02 10:20:40' },
  { id: 'm3', fileName: '空白讲义.pdf', lessonId: 'l1', tagCode: 'Blank', createdAt: '' },
  { id: 'm4', fileName: '第二课讲义.pdf', lessonId: 'l2', tagCode: 'HD', createdAt: '2026-11-03 11:30:50' }
].map((item, index) => ({ ...item, title: item.fileName, courseId: 'c1', course: course.name, learningSpaceId: 'e1', sortOrder: index + 1, status: '启用', publishStatus: '已发布', previewStatus: '可预览' }));

async function setup(page: Page, fail = false, readonly = false) {
  let materials = initial.map(item => ({ ...item }));
  const writes: { courseId: string; materialIds: string[] }[] = [];
  const user = { userId: 'user-super', name: '管理员', roles: [readonly ? 'teacher' : 'super_admin'], canUploadHandout: !readonly };
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'local-order-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', async route => {
    const req = route.request(); const url = new URL(req.url()); const path = url.pathname.replace('/api', ''); let data: unknown = [];
    if (path === '/auth/me') data = user;
    if (path === '/courses') data = [course];
    if (path === '/learning-spaces') data = [{ id: 'e1', name: course.name, grade: course.grade, subject: course.subject, semester: 'S1', phase: 'Q1', status: '启用' }];
    if (path === '/materials') data = materials.filter(item => !url.searchParams.get('keyword') || item.fileName.includes(url.searchParams.get('keyword')!)).sort((a, b) => a.sortOrder - b.sortOrder);
    if (path === '/materials/reorder') {
      const body = req.postDataJSON(); writes.push(body);
      if (fail) return route.fulfill({ status: 400, json: { code: 400, message: '排序保存失败，请刷新后重试。' } });
      materials = materials.map(item => ({ ...item, sortOrder: body.materialIds.indexOf(item.id) + 1 }));
      data = { reordered: true };
    }
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.goto('/content?tab=materials&courseId=c1');
  await expect(page.locator('.lesson-pack-file-name')).toHaveCount(4);
  await page.getByRole('button', { name: '全部展开', exact: true }).click();
  return writes;
}
async function drag(page: Page, source: string, target: string) {
  const transfer = await page.evaluateHandle(() => new DataTransfer());
  await page.getByRole('button', { name: `调整文件顺序：${source}`, exact: true }).dispatchEvent('dragstart', { dataTransfer: transfer });
  await page.locator('.lesson-pack-file').filter({ hasText: target }).dispatchEvent('drop', { dataTransfer: transfer });
}

test('file timestamps and cross-tag order survive reload; pack drag still works', async ({ page }) => {
  const writes = await setup(page);
  for (const item of initial) await expect(page.locator('.lesson-pack-file').filter({ hasText: item.fileName })).toContainText(`上传时间：${item.createdAt || '—'}`);
  await page.getByRole('button', { name: '调整文件顺序：空白讲义.pdf', exact: true }).dragTo(page.locator('.lesson-pack-file').filter({ hasText: '阅读讲义.pdf' }));
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0]).toEqual({ courseId: 'c1', materialIds: ['m3', 'm1', 'm2', 'm4'] });
  await page.reload();
  await page.getByRole('button', { name: '全部展开', exact: true }).click();
  await expect(page.locator('.lesson-pack-file-name')).toHaveText(['空白讲义.pdf', '阅读讲义.pdf', '课后练习.pdf', '第二课讲义.pdf']);
  await page.screenshot({ path: '/tmp/starline-material-file-order.png', fullPage: true });
  const packs = page.locator('.material-pack-table .ant-table-tbody > tr');
  const transfer = await page.evaluateHandle(() => new DataTransfer());
  await packs.nth(1).getByTitle('拖动调整同一课程内课节顺序', { exact: true }).dispatchEvent('dragstart', { dataTransfer: transfer });
  await packs.nth(0).dispatchEvent('drop', { dataTransfer: transfer });
  await expect.poll(() => writes.length).toBe(2);
  expect(writes[1].materialIds).toEqual(['m4', 'm3', 'm1', 'm2']);
});

test('cross-lesson drop is rejected; filtered sorting preserves hidden files', async ({ page }) => {
  const writes = await setup(page);
  await drag(page, '阅读讲义.pdf', '第二课讲义.pdf');
  await expect(page.getByText('文件只能在同一课节内调整顺序。')).toBeVisible();
  expect(writes).toHaveLength(0);
  await page.getByPlaceholder('搜索课节或文件名', { exact: true }).fill('讲义');
  await expect(page.locator('.lesson-pack-file-name')).toHaveCount(3);
  await drag(page, '空白讲义.pdf', '阅读讲义.pdf');
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].materialIds).toEqual(['m3', 'm1', 'm2', 'm4']);
});

test('failed saves preserve display order; keyboard can move files', async ({ page }) => {
  const writes = await setup(page, true);
  await page.getByRole('button', { name: '调整文件顺序：阅读讲义.pdf', exact: true }).press('ArrowDown');
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].materialIds).toEqual(['m2', 'm1', 'm3', 'm4']);
  await expect(page.getByText('排序保存失败，请刷新后重试。')).toBeVisible();
  await expect(page.locator('.lesson-pack-file-name')).toHaveText(initial.map(item => item.fileName));
});

test('view-only teachers see timestamps without reorder controls', async ({ page }) => {
  await setup(page, false, true);
  await expect(page.getByRole('button', { name: /调整文件顺序：/ })).toHaveCount(0);
  await expect(page.locator('.lesson-pack-file-uploaded').first()).toHaveText('上传时间：2026-11-01 09:12:30');
});
