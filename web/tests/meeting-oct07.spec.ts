import { test, expect } from '@playwright/test';

const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
const spaces = [
  { id: 'source-space', grade: '五年级', subject: 'English', semester: 's1', phase: 'q1', level: 'S', status: '启用' },
  { id: 'target-space', grade: '五年级', subject: 'English', semester: 's1', phase: 'q1', level: '自定义班型', status: '启用' }
];
const source = { id: 'source', name: 'G5S1Q1 English S', grade: '五年级', subject: 'English', learningSpaceId: 'source-space', status: '启用', curriculum: [{ id: 'unit-one', type: 'unit', name: '第一单元', sortOrder: 1 }] };
const target = { ...source, id: 'target', name: 'G5S1Q1 English 自定义班型', learningSpaceId: 'target-space', curriculum: [{ id: 'target-unit', type: 'unit', name: '原有单元', sortOrder: 1 }] };
async function login(page: any) {
  await page.addInitScript((user: any) => {
    localStorage.setItem('starline_admin_token', 'meeting-oct07-fixture');
    localStorage.setItem('starline_admin_user', JSON.stringify(user));
    localStorage.setItem('starline:list-view:teachers', 'table');
  }, user);
}

test('new Unit saves then previews only the selected Unit and dynamic target', async ({ page }) => {
  await login(page); let saved = source; const requests: any[] = [];
  await page.route('**/api/**', route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    if (path === '/courses/source' && req.method() === 'PUT') { saved = { ...source, ...req.postDataJSON() }; return route.fulfill({ json: { code: 0, data: saved } }); }
    if (path.includes('directory-sync')) {
      const body = req.postDataJSON(); requests.push({ path, body });
      expect(saved.curriculum).toHaveLength(2);
      return route.fulfill({ json: { code: 0, data: { targets: [{ courseId: 'target', courseName: target.name, added: ['Unit 2'], updated: [], preserved: 1, snapshot: 'version-2', ...(path.endsWith('directory-sync') ? { status: '已同步' } : {}) }] } } });
    }
    const data = path === '/auth/me' ? user : path === '/courses' ? [saved, target] : path === '/learning-spaces' ? spaces : path === '/subjects' ? [{ id: 'english', name: 'English', status: '启用' }] : [];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/content');
  await page.getByRole('row').filter({ hasText: source.name }).getByRole('button', { name: '编辑', exact: true }).click();
  const drawer = page.getByRole('dialog', { name: '编辑课程', exact: true });
  await drawer.getByRole('button', { name: '新增 Unit', exact: false }).click();
  const unit = drawer.getByTestId('curriculum-unit').last();
  await unit.getByLabel('Unit名称（必填）', { exact: true }).fill('第二单元');
  await unit.getByRole('button', { name: '同步此 Unit', exact: true }).click();
  await unit.getByRole('checkbox', { name: target.name, exact: true }).check();
  await drawer.getByRole('button', { name: '保存并预览同步', exact: true }).click();
  const modal = page.getByRole('dialog', { name: `跨班型同步 · ${source.name}`, exact: true });
  await expect(modal).toContainText('同步所选 Unit 及下级目录');
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0].body).toMatchObject({ targetCourseIds: ['target'], unitIds: [saved.curriculum[1].id] });
  await page.screenshot({ path: '/tmp/starline-oct07-directory-sync.png', fullPage: false });
  await modal.getByRole('button', { name: /确认同步目录/ }).click();
  await expect(modal).toContainText('已同步');
  expect(requests[1].body).toMatchObject({ snapshots: { target: 'version-2' }, unitIds: [saved.curriculum[1].id] });
});

for (const scope of ['whole', 'unit', 'save-failure'] as const) {
  test(`upload directory maintenance supports existing sync: ${scope}`, async ({ page }) => {
    await login(page);
    let saved = source; let failSave = scope === 'save-failure'; const writes: any[] = [];
    await page.route('**/api/**', route => {
      const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
      if (req.method() !== 'GET') writes.push({ path, body: req.postDataJSON() });
      if (path === '/courses/source' && req.method() === 'PUT') {
        if (failSave) return route.fulfill({ status: 400, json: { code: 400, message: '目录保存失败' } });
        saved = { ...source, ...req.postDataJSON() };
        return route.fulfill({ json: { code: 0, data: saved } });
      }
      if (path.includes('directory-sync')) return route.fulfill({ json: { code: 0, data: { targets: [{ courseId: target.id, courseName: target.name, added: ['第一单元'], updated: [], preserved: 1, snapshot: 'upload-directory-version', ...(path.endsWith('directory-sync') ? { status: '已同步' } : {}) }] } } });
      const data = path === '/auth/me' ? user : path === '/courses' ? [saved, target] : path === '/learning-spaces' ? spaces : path === '/subjects' ? [{ id: 'english', name: 'English', status: '启用' }] : [];
      return route.fulfill({ json: { code: 0, data } });
    });
    await page.goto('/content?tab=materials&courseId=source');
    await expect(page.getByRole('heading', { name: source.name, exact: true })).toBeVisible();
    await page.getByRole('button', { name: /上传讲义/ }).click();
    const upload = page.getByRole('dialog', { name: '给课节上传资料', exact: true });
    await upload.getByRole('button', { name: /维护本课程目录/ }).click();
    const editor = page.getByRole('dialog', { name: '编辑课程', exact: true });
    await expect(editor.getByRole('button', { name: '同步整套目录', exact: true })).toBeVisible();
    await expect(editor.getByRole('button', { name: '同步此 Unit', exact: true })).toBeVisible();
    await editor.getByRole('button', { name: scope === 'unit' ? '同步此 Unit' : '同步整套目录', exact: true }).click();
    await editor.getByRole('checkbox', { name: target.name, exact: true }).check();
    await editor.getByRole('button', { name: '保存并预览同步', exact: true }).click();
    const modal = page.getByRole('dialog', { name: `跨班型同步 · ${source.name}`, exact: true });
    if (failSave) {
      await expect(page.getByText('目录保存失败', { exact: true })).toBeVisible();
      await expect(editor).toBeVisible();
      await expect(editor.getByRole('checkbox', { name: target.name, exact: true })).toBeChecked();
      await expect(modal).toHaveCount(0);
      expect(writes.map(item => item.path)).toEqual(['/courses/source']);
      failSave = false;
      await editor.getByRole('button', { name: '保存并预览同步', exact: true }).click();
    }
    await expect(modal).toBeVisible();
    await expect.poll(() => writes.filter(item => item.path === '/courses/directory-sync-preview').length).toBe(1);
    const syncRequest = { sourceCourseId: source.id, targetCourseIds: [target.id], ...(scope === 'unit' ? { unitIds: ['unit-one'] } : {}) };
    expect(writes.find(item => item.path === '/courses/directory-sync-preview').body).toEqual(syncRequest);
    await modal.getByRole('button', { name: '确认同步目录', exact: true }).click();
    await expect(modal).toContainText('已同步');
    expect(writes.at(-1)).toEqual({ path: '/courses/directory-sync', body: { ...syncRequest, snapshots: { [target.id]: 'upload-directory-version' } } });
    await modal.getByRole('button', { name: /^关\s*闭$/ }).click();
    await expect(upload).toBeVisible();
    await expect(upload.getByRole('button', { name: /维护本课程目录/ })).toBeVisible();
    await upload.getByRole('button', { name: /维护本课程目录/ }).click();
    await expect(editor.getByRole('button', { name: '同步此 Unit', exact: true })).toBeVisible();
    if (scope === 'unit') {
      await expect.poll(async () => editor.evaluate(el => Math.round(el.getBoundingClientRect().right))).toBe(1280);
      await page.screenshot({ path: '/tmp/starline-upload-directory-sync-fixed.png', fullPage: false });
    }
  });
}

test('course-only teaching plan uploads keep course and retry only failed file', async ({ page }) => {
  await login(page); const uploads: string[] = []; let count = 0;
  await page.route('**/api/**', route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    if (path === '/teaching-plans' && req.method() === 'POST') {
      uploads.push(req.postData() || ''); count++;
      if (count === 2) return route.fulfill({ status: 400, json: { code: 400, message: '第二份暂时上传失败' } });
      return route.fulfill({ json: { code: 0, data: { id: `plan-${count}` } } });
    }
    const data = path === '/auth/me' ? user : path === '/teaching-plans' ? { plans: [], directories: [source], uploadScopes: [{ grade: '五年级', subject: 'English' }], canUpload: true } : [];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/teaching-plans'); await page.getByRole('button', { name: /上传教案/ }).click();
  const drawer = page.getByRole('dialog', { name: '上传教案', exact: true });
  await drawer.getByLabel('选择教案文件', { exact: true }).setInputFiles(['one.pdf', 'two.pdf'].map(name => ({ name, mimeType: 'application/pdf', buffer: Buffer.from('%PDF-fixture') })));
  await drawer.getByRole('button', { name: /上传 2 份/ }).click();
  await expect(drawer).toContainText('第二份暂时上传失败');
  await drawer.getByRole('button', { name: '重试此文件', exact: true }).click();
  await expect(drawer).not.toBeVisible();
  expect(uploads).toHaveLength(3);
  for (const body of uploads) { expect(body).toContain('name="courseId"\r\n\r\nsource'); expect(body).not.toContain('name="lessonId"'); }
  expect(uploads.map(body => body.match(/name="batchId"\r\n\r\n([^\r]+)/)?.[1])).toEqual(Array(3).fill(uploads[0].match(/name="batchId"\r\n\r\n([^\r]+)/)?.[1]));
});

test('teaching plans browse course groups and retain selected course on reload', async ({ page }) => {
  await login(page);
  const courses = [source, { ...target, name: '另一个课程' }];
  const plans = courses.map((course, i) => ({ id: `plan-${i}`, courseId: course.id, title: `备课${i}`, grade: course.grade, subject: course.subject, semester: 's1', phase: 'q1', fileName: `plan${i}.pdf`, fileSize: 100, previewStatus: '可预览', uploaderName: '王老师', createdAt: '2026-10-07' }));
  await page.route('**/api/**', route => { const path = new URL(route.request().url()).pathname.replace('/api', ''); return route.fulfill({ json: { code: 0, data: path === '/auth/me' ? user : path === '/teaching-plans' ? { plans, courses: courses.map(course => ({ ...course, semester: 's1', phase: 'q1', level: 'S' })), uploadScopes: [], canUpload: false } : [] } }); });
  await page.goto('/teaching-plans');
  await expect(page.getByRole('button', { name: source.name, exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '备课0', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: source.name, exact: true }).click();
  await expect(page.getByRole('button', { name: '备课0', exact: true })).toBeVisible();
  await expect(page.getByText('课程通用', { exact: true })).toBeVisible();
  await page.reload(); await expect(page.getByRole('button', { name: '备课0', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '返回课程列表', exact: true }).click();
  await page.screenshot({ path: '/tmp/starline-oct07-teaching-plans.png', fullPage: false });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('group', { name: '年级', exact: true })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
  expect(overflow).toBe(false);
  await page.screenshot({ path: '/tmp/starline-oct07-teaching-plans-mobile.png', fullPage: false });
});

test('teacher table keeps common actions and name on left and account actions last', async ({ page }) => {
  await login(page); await page.setViewportSize({ width: 1280, height: 900 });
  const teacher = { id: 'teacher', name: '一位姓名比较长的授课老师', phone: '19900000001', accountStatus: '正常', learningSpaceIds: spaces.map(s => s.id), canUploadHandout: true, canUploadQuestion: true, canReview: true, teacherLibrary: { scopes: [{ grade: '五年级', subject: 'English' }] } };
  await page.route('**/api/**', route => { const path = new URL(route.request().url()).pathname.replace('/api', ''); return route.fulfill({ json: { code: 0, data: path === '/auth/me' ? user : path === '/teachers' ? [teacher] : path === '/learning-spaces' ? spaces : [] } }); });
  await page.goto('/teachers');
  await expect(page.locator('.ant-table-thead th')).toHaveText(['常用操作', '姓名', '资料查阅范围', '授课范围', '可上传内容', '可批改', '手机号', '登录方式', '账号状态', '备注', '账号操作']);
  await page.locator('.ant-table-content').evaluate(el => { el.scrollLeft = el.scrollWidth; });
  const edit = page.getByRole('button', { name: '编辑', exact: true });
  await expect(edit).toBeVisible();
  const bounds = await edit.boundingBox(); expect(bounds!.x).toBeGreaterThanOrEqual(0); expect(bounds!.x + bounds!.width).toBeLessThan(1280);
  await page.screenshot({ path: '/tmp/starline-oct07-teachers.png', fullPage: false });
});
