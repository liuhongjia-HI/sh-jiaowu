import { expect, test } from '@playwright/test';

test('开通概览组合筛选及个人摘要', async ({ page }) => {
  const scopes = [
    { grade: '四年级', subject: 'English', level: 'S' },
    { grade: '四年级', subject: 'English', level: 'A' },
    { grade: '五年级', subject: 'Mathematics', level: '' }
  ];
  const students = [
    { id: 'one', name: '学生甲', activeOpenings: scopes },
    { id: 'two', name: '学生乙', activeOpenings: [scopes[0]] },
    { id: 'three', name: '学生丙', activeOpenings: [] }
  ].map((student) => ({
    grade: '六年级', phone: '13800000000', accountStatus: '正常',
    learningStatus: '未开始', openedPackages: ['历史套餐'], openedPackageRefs: [],
    followUpStatus: '', bindStatus: '待绑定', streakDays: 0, badgeCount: 0, ...student
  }));
  await page.addInitScript(() => localStorage.setItem('starline_admin_token', 'test-token'));
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url());
    let data: unknown = [];
    if (url.pathname.endsWith('/auth/me')) {
      data = { userId: 'test', name: '测试管理员', roles: ['super_admin'], campusId: 'test' };
    } else if (url.pathname.endsWith('/students')) {
      const state = url.searchParams.get('packageState');
      data = students.filter((student) => !state || (state === '已开通' ? student.activeOpenings.length > 0 : student.activeOpenings.length === 0));
    } else if (url.pathname.endsWith('/default')) { data = {}; }
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.goto('/students');
  await expect(page.getByText('学生甲', { exact: true })).toBeVisible();
  await expect(page.getByText('暂无有效开通', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '展开全部 3 项' }).click();
  await expect(page.getByText('五年级 · Mathematics · 未设置班型', { exact: true })).toBeVisible();
  await expect(page.getByText('当前开通分布', { exact: true })).toBeVisible();
  await expect(page.getByText('全部可见学生 · 2 个年级 · 2 门科目')).toBeVisible();
  const overview = page.locator('.opening-overview');
  const allCell = overview.getByRole('button', { name: '四年级 English 全部班型 2人', exact: true });
  await expect(allCell).toBeVisible();
  await expect(allCell).toContainText('A 1');
  await expect(allCell).toContainText('S 2');
  await allCell.click();
  await expect(page.getByText('学生乙', { exact: true })).toBeVisible();
  await expect(page.getByText('学生丙', { exact: true })).toHaveCount(0);
  await expect(allCell).toHaveAttribute('aria-pressed', 'true');
  await overview.getByRole('button', { name: 'A班', exact: true }).click();
  await expect(overview.getByRole('button', { name: '四年级 English A班 1人', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByText('学生甲', { exact: true })).toBeVisible();
  await expect(page.getByText('学生乙', { exact: true })).toHaveCount(0);
  await expect(page.getByText('学生丙', { exact: true })).toHaveCount(0);
  await expect(page.getByText('全部可见学生 · 2 个年级 · 2 门科目')).toBeVisible();
  await overview.getByRole('button', { name: '未设置班型', exact: true }).click();
  await expect(page.getByText('学生甲', { exact: true })).toHaveCount(0);
  await overview.getByRole('button', { name: '五年级 Mathematics 未设置班型 1人', exact: true }).click();
  await expect(page.getByText('学生甲', { exact: true })).toBeVisible();
  await overview.getByRole('button', { name: '查看明细', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: '1 人 · 查看学生' }).click();
  await expect(dialog).toBeHidden();
  await page.getByRole('button', { name: '查看全部学生', exact: true }).click();
  await expect(page.getByText('学生乙', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '筛选已开通课程的学生' }).click();
  await expect(page.getByText('学生丙', { exact: true })).toHaveCount(0);
  await page.getByText('表格', { exact: true }).click();
  await expect(page.getByRole('columnheader', { name: '当前已开通', exact: true })).toBeVisible();
  await expect(page.getByText('学生甲', { exact: true })).toBeVisible();
});

test('开通分布空数据与加载失败', async ({ page }) => {
  let failed = true;
  let studentCalls = 0;
  await page.addInitScript(() => localStorage.setItem('starline_admin_token', 'test-token'));
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith('/students') && ++studentCalls > 1 && failed) { await route.fulfill({ status: 500, json: { message: 'test failure' } }); return; }
    await route.fulfill({ json: { code: 0, message: 'ok', data: path.endsWith('/auth/me') ? { userId: 'test', name: '测试', roles: ['super_admin'] } : [] } });
  });
  await page.goto('/students');
  await expect(page.getByText('开通分布加载失败', { exact: true })).toBeVisible({ timeout: 20000 });
  failed = false;
  await page.locator('.opening-overview').getByRole('button', { name: /重\s*试/ }).click();
  await expect(page.getByText('暂无当前有效开通', { exact: true })).toBeVisible();
});

test('四年级五科矩阵在桌面紧凑展示，窄屏可横向查看', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const grades = ['四年级', '五年级', '六年级', '七年级'];
  const subjects = ['English', 'Mathematics', 'Science', 'History', 'Geography'];
  const students = Array.from({ length: 8 }, (_, i) => ({ id: String(i), name: `示例学生 ${i + 1}`, grade: '四年级', phone: '', accountStatus: '正常', learningStatus: '未开始', activeOpenings: grades.flatMap((grade, g) => subjects.flatMap((subject, s) => i < (g + s) % 8 + 1 ? [{ grade, subject, level: g === 3 ? 'H' : i % 2 ? 'S' : 'A' }] : [])), openedPackages: [], openedPackageRefs: [], streakDays: 0, badgeCount: 0 }));
  await page.addInitScript(() => localStorage.setItem('starline_admin_token', 'test-token'));
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname;
    await route.fulfill({ json: { code: 0, message: 'ok', data: path.endsWith('/auth/me') ? { userId: 'test', name: '测试', roles: ['super_admin'] } : path.endsWith('/students') ? students : [] } });
  });
  await page.goto('/students');
  const overview = page.locator('.opening-overview');
  await expect(overview.locator('tbody tr')).toHaveCount(4);
  await expect(overview.locator('thead th')).toHaveCount(6);
  await overview.screenshot({ path: '/tmp/starline-opening-matrix-desktop.png' });
  expect((await overview.boundingBox())!.height).toBeLessThan(460);
  await page.setViewportSize({ width: 390, height: 844 });
  const scroll = overview.locator('.opening-matrix-scroll');
  expect(await scroll.evaluate(el => el.scrollWidth > el.clientWidth)).toBe(true);
  await scroll.evaluate(el => { el.scrollLeft = el.scrollWidth; });
  await expect(overview.getByRole('button', { name: '七年级 Geography 全部班型 8人', exact: true })).toBeVisible();
});
