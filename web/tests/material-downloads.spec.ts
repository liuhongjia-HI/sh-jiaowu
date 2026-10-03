import { test, expect } from '@playwright/test';

const jobId = `download-${'a'.repeat(32)}`;
const user = { userId: 'teacher', name: '测试老师', roles: ['teacher'], teacherLibrary: { spaceIds: ['e1'], canDownload: true, canManageCourses: false }, canUploadHandout: false, canUploadQuestion: false, canReview: false };
const course = { id: 'c1', name: '五年级英语 S', grade: '五年级', subject: 'English', learningSpaceId: 'e1', status: '启用', curriculum: [{ id: 'u1', name: '第一课', type: 'unit', sortOrder: 1 }], lessonCount: 1 };
const spaces = [{ id: 'e1', name: '五年级英语', grade: '五年级', subject: 'English', semester: 'S1', phase: 'Q1', status: '启用' }];
const ready = { id: jobId, scope: { subject: 'English' }, status: '可下载', count: 2, size: 2048, createdAt: '2026-10-03T01:00:00Z', expiresAt: '2099-10-04T01:00:00Z' };

async function seed(page: import('@playwright/test').Page) {
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'download-ui-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
}

test('batch download narrows chapters, previews count, downloads authenticated blob and retries failed task', async ({ page }) => {
  await seed(page);
  let jobs: any[] = [];
  const requests: any[] = [];
  let failPickup = false;
  await page.route('**/api/**', async route => {
    const request = route.request(); const path = new URL(request.url()).pathname.replace('/api', '');
    let data: any = [];
    if (path === '/auth/me') data = user;
    if (path === '/subjects') data = [{ id: 'english', name: 'English', status: '启用' }];
    if (path === '/teacher/library') data = { courses: [course], spaces, materials: [], recentMaterialIds: [], canDownload: true, policy: user.teacherLibrary };
    if (path === '/material-downloads/selection') { requests.push({ path, body: request.postDataJSON() }); data = { count: 2, size: 2048, courses: [course.name] }; }
    if (path === '/material-downloads' && request.method() === 'POST') { requests.push({ path, body: request.postDataJSON() }); jobs = [ready]; data = ready; }
    if (path === '/material-downloads' && request.method() === 'GET') data = jobs;
    if (path.endsWith('/retry')) { requests.push({ path }); jobs = [{ ...ready, id: `download-${'b'.repeat(32)}`, status: '准备中' }, ...jobs]; data = jobs[0]; }
    if (path.endsWith('/archive')) {
      expect(request.headers()['authorization']).toBe('Bearer download-ui-fixture');
      if (failPickup) { jobs = [{ ...ready, status: '失败', error: '下载包文件不可用，请重新生成' }]; return route.fulfill({ status: 400, json: { code: 400, message: '下载包文件不可用，请重新生成' } }); }
      return route.fulfill({ contentType: 'application/zip', body: Buffer.from('mock ZIP bytes; actual ZIP validated by Go tests') });
    }
    return route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.goto('/teacher-library');
  await page.getByRole('button', { name: '批量下载讲义', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '批量下载讲义', exact: true });
  await expect(dialog.getByText('2 份已发布讲义 · 2 KB', { exact: true })).toBeVisible();
  await dialog.locator('.ant-select').filter({ has: page.getByRole('combobox', { name: '下载课程', exact: true }) }).locator('.ant-select-selector').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText(course.name, { exact: true }).click();
  await page.keyboard.press('Escape');
  await dialog.locator('.ant-select').filter({ has: page.getByRole('combobox', { name: '下载章节', exact: true }) }).locator('.ant-select-selector').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText(`${course.name} / 第一课`, { exact: true }).click();
  await page.keyboard.press('Escape');
  await dialog.getByRole('button', { name: '生成下载包', exact: true }).click();
  await expect(dialog.getByRole('button', { name: '下载 ZIP', exact: true })).toBeVisible();
  const created = requests.find(r => r.path === '/material-downloads');
  expect(created.body).toEqual({ subject: 'English', courseIds: ['c1'], lessonIds: ['u1'] });
  const downloadEvent = page.waitForEvent('download');
  await dialog.getByRole('button', { name: '下载 ZIP', exact: true }).click();
  expect((await downloadEvent).suggestedFilename()).toMatch(/讲义\.zip$/);
  await dialog.getByRole('button', { name: '复制电脑领取链接', exact: true }).click();
  await expect(dialog.getByRole('textbox', { name: '电脑领取链接', exact: true })).toHaveValue(`http://127.0.0.1:5186/teacher-library?downloadJob=${jobId}`);
  failPickup = true;
  await dialog.getByRole('button', { name: '下载 ZIP', exact: true }).click();
  await expect(dialog.getByRole('button', { name: '重新生成', exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: '重新生成', exact: true }).click();
  await expect(dialog.getByText('准备中', { exact: true })).toBeVisible();
  expect(requests.filter(r => r.path.endsWith('/retry'))).toHaveLength(1);
});

test('mobile pickup opens own task, supports expired regeneration and keeps dialog within viewport', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 }); await seed(page);
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname.replace('/api', '');
    const data = path === '/auth/me' ? user : path === '/teacher/library' ? { courses: [course], spaces, materials: [], canDownload: true, recentMaterialIds: [] } : path === '/material-downloads' ? [{ ...ready, status: '已过期' }] : [];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto(`/teacher-library?downloadJob=${jobId}`);
  const dialog = page.getByRole('dialog', { name: '批量下载讲义', exact: true });
  await expect(dialog.getByRole('button', { name: '重新生成', exact: true })).toBeVisible();
  const bounds = await dialog.boundingBox(); expect(bounds!.x).toBeGreaterThanOrEqual(0); expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(375);
  expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
  await expect.poll(() => dialog.evaluate(el => getComputedStyle(el).transform)).toBe('none');
  await page.screenshot({ path: '/tmp/starline-material-download-mobile.png', fullPage: false, animations: 'disabled' });
});

test('computer pickup survives password login without adding credentials to URL', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname.replace('/api', '');
    const data = path === '/auth/admin-password-login' ? { token: 'download-ui-fixture', user, authMethod: 'password' } : path === '/auth/me' ? user : path === '/teacher/library' ? { courses: [course], spaces, materials: [], canDownload: true, recentMaterialIds: [] } : path === '/material-downloads' ? [ready] : [];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto(`/teacher-library?downloadJob=${jobId}`);
  await expect(page).toHaveURL(`/login?downloadJob=${jobId}`);
  await page.getByLabel('手机号', { exact: true }).fill('13800000004');
  await page.getByLabel('密码', { exact: true }).fill('test-password');
  await page.locator('button[type=submit]').click();
  await expect(page).toHaveURL(`/teacher-library?downloadJob=${jobId}`);
  await expect(page.getByRole('dialog', { name: '批量下载讲义', exact: true }).getByRole('button', { name: '下载 ZIP', exact: true })).toBeVisible();
});

test('pickup survives mandatory password change and subsequent login', async ({ page }) => {
  let changed = false;
  let logins = 0;
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname.replace('/api', '');
    const current = { ...user, authMethod: 'password', mustChangePassword: !changed };
    let data: any = [];
    if (path === '/auth/admin-password-login') { logins++; data = { token: 'download-ui-fixture', user: current, authMethod: 'password' }; }
    if (path === '/auth/me') data = current;
    if (path === '/auth/change-password') {
      expect(route.request().postDataJSON()).toEqual({ oldPassword: 'temporary-password', newPassword: 'new-password123' });
      changed = true; data = { changed: true };
    }
    if (path === '/teacher/library') data = { courses: [course], spaces, materials: [], canDownload: true, recentMaterialIds: [] };
    if (path === '/material-downloads') data = [ready];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto(`/teacher-library?downloadJob=${jobId}`);
  await page.getByLabel('手机号', { exact: true }).fill('13800000004');
  await page.getByLabel('密码', { exact: true }).fill('temporary-password');
  await page.locator('button[type=submit]').click();
  await expect(page.getByText('修改初始密码', { exact: true })).toBeVisible();
  await page.getByLabel('临时密码', { exact: true }).fill('temporary-password');
  await page.getByLabel('新密码', { exact: true }).fill('new-password123');
  await page.getByLabel('确认新密码', { exact: true }).fill('new-password123');
  await page.getByRole('button', { name: '保存并重新登录' }).click();
  await expect(page).toHaveURL(`/login?downloadJob=${jobId}`);
  await page.getByLabel('手机号', { exact: true }).fill('13800000004');
  await page.getByLabel('密码', { exact: true }).fill('new-password123');
  await page.locator('button[type=submit]').click();
  await expect(page).toHaveURL(`/teacher-library?downloadJob=${jobId}`);
  await expect(page.getByRole('dialog', { name: '批量下载讲义', exact: true }).getByRole('button', { name: '下载 ZIP', exact: true })).toBeVisible();
  expect(logins).toBe(2);
});
