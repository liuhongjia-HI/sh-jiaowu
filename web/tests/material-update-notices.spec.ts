import { test, expect } from '@playwright/test';

test('edit material supports an optional replacement and sends it with publication fields', async ({ page }) => {
  const user = { userId: 'user-super', name: '管理员', roles: ['super_admin'], mustChangePassword: false };
  const course = { id: 'c1', name: '五年级英语 S1', grade: '五年级', subject: 'English', learningSpaceId: 'e1', status: '启用', curriculum: [{ id: 'l1', name: '第一课', type: 'lesson', sortOrder: 1 }] };
  const material = { id: 'm1', title: '第一课讲义', courseId: 'c1', course: course.name, learningSpaceId: 'e1', lessonId: 'l1', tagCode: 'HD', type: '课程讲义', status: '启用', publishStatus: '已发布', fileName: '第一课.pdf', fileId: 'old', createdAt: '2026-11-04 10:00:00', allowDownload: true };
  const writes: { contentType: string; body: string }[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'local-material-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', async route => {
    const request = route.request(); const path = new URL(request.url()).pathname.replace('/api', ''); let data: unknown = [];
    if (path === '/auth/me') data = user;
    if (path === '/courses') data = [course];
    if (path === '/learning-spaces') data = [{ id: 'e1', name: '五年级英语', grade: '五年级', subject: 'English', semester: 'S1', phase: 'Q1', status: '启用' }];
    if (path === '/subjects') data = [{ id: 'english', name: 'English', status: '启用' }];
    if (path === '/materials') data = [material];
    if (path === '/materials/m1' && request.method() === 'PUT') { writes.push({ contentType: request.headers()['content-type'], body: request.postDataBuffer()?.toString() || '' }); data = material; }
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.goto('/content?tab=materials&courseId=c1');
  await expect(page.locator('.lesson-pack-file-name')).toHaveCount(1);
  if (await page.getByRole('button', { name: '展开文件', exact: true }).count()) await page.getByRole('button', { name: '展开文件', exact: true }).first().click();
  await page.getByRole('button', { name: '编辑HD', exact: true }).first().click();
  const dialog = page.getByRole('dialog', { name: '编辑课程讲义', exact: true });
  await expect(dialog.getByText('替换课件文件', { exact: true })).toBeVisible();
  await expect(dialog.getByText(/替换并发布后会通知当前有正式访问权限的学生/)).toBeVisible();
  await dialog.locator('input[type=file]').setInputFiles({ name: '新版.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.4 replacement') });
  await expect(dialog.getByText('新版.pdf', { exact: true })).toBeVisible();
  await expect.poll(() => dialog.locator('..').evaluate(el => getComputedStyle(el).transform)).toBe('none');
  await page.screenshot({ path: '/tmp/starline-material-update-notice.png', fullPage: false, animations: 'disabled' });
  await dialog.getByRole('button', { name: /保\s*存|确\s*定/ }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].contentType).toContain('multipart/form-data; boundary=');
  expect(writes[0].body).toContain('filename="新版.pdf"');
  expect(writes[0].body).toContain('name="status"\r\n\r\n已发布');
  expect(writes[0].body).toContain('name="courseId"\r\n\r\nc1');
  await expect(dialog).not.toBeVisible();
});
