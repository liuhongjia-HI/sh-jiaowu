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

test('课程权限按四个 Term 分组，跨筛选勾选仍按原协议保存', async ({ page }) => {
  const student = { id: 'semester-student', name: '学期检查学生', grade: '四年级', phone: '', accountStatus: '正常', learningStatus: '未开始', activeOpenings: [], openedPackages: [], openedPackageRefs: [], streakDays: 0, badgeCount: 0 };
  // 名称刻意不带期次，验证分组使用课程范围的结构化字段。
  const spaces = [
    { id: 'second', name: '英语下阶段', semester: 'S2', subject: 'English' },
    { id: 'first', name: '英语上阶段', semester: 'S1', subject: 'English' },
    { id: 'fourth-term', name: '英语期末阶段', semester: 'S2', phase: 'Q2', subject: 'English' },
    { id: 'second-term', name: '英语进阶阶段', semester: 'S1', phase: 'Q2', subject: 'English' },
    { id: 'locked', name: '数学套餐范围', semester: 'S1', subject: 'Mathematics' },
    { id: 'custom', name: '暑期英语', semester: '暑期', phase: '', subject: 'English' },
    { id: 'missing-phase', name: '未设置阶段的英语', semester: 'S1', phase: '', subject: 'English' }
  ].map(space => ({ grade: '四年级', phase: 'Q1', level: 'S', status: '启用', ...space }));
  const matrix = [...spaces, { id: 'missing', name: '历史英语范围', subject: 'English' }].map(space => ({
    learningSpaceId: space.id, name: space.name, subject: space.subject, subjectId: space.subject, subjectSortOrder: space.subject === 'English' ? 1 : 2,
    content: ['course', 'handout', 'question', 'download'].map(contentTypeCode => ({ contentTypeCode, opened: space.id === 'locked', packageOpened: space.id === 'locked', directOpened: false, packageNames: space.id === 'locked' ? ['数学套餐'] : [], packageGrants: [], items: [] }))
  }));
  let saved: unknown;
  await page.addInitScript(() => localStorage.setItem('starline_admin_token', 'test-token'));
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = [];
    if (path.endsWith('/auth/me')) data = { userId: 'test', name: '测试管理员', roles: ['super_admin'] };
    else if (path.endsWith('/grants/direct') && route.request().method() === 'PUT') {
      saved = route.request().postDataJSON(); data = { studentName: student.name };
    } else if (path.endsWith('/grants/direct/default')) data = { startsAt: '2026-11-17', endsAt: '2027-01-15' };
    else if (path.endsWith('/students/semester-student')) data = { student, grants: [], openingMatrix: matrix, permissions: {}, learningRecords: [], notices: [], logs: [] };
    else if (path.endsWith('/students')) data = [student];
    else if (path.endsWith('/learning-spaces')) data = spaces;
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.setViewportSize({ width: 1440, height: 1100 });
  await page.goto('/students');
  await page.getByRole('button', { name: '课程开通', exact: true }).click();
  const first = page.getByRole('region', { name: 'Term 1 · S1 Q1', exact: true });
  const third = page.getByRole('region', { name: 'Term 3 · S2 Q1', exact: true });
  await expect(first).toContainText('英语上阶段');
  await expect(first).toContainText('2 个课程范围');
  await expect(page.getByRole('region', { name: 'Term 2 · S1 Q2', exact: true })).toContainText('英语进阶阶段');
  await expect(third).toContainText('英语下阶段');
  await expect(page.getByRole('region', { name: 'Term 4 · S2 Q2', exact: true })).toContainText('英语期末阶段');
  const regionNames = await page.locator('.student-opening-panel [role=region]').evaluateAll(elements => elements.map(el => el.getAttribute('aria-label')));
  expect(regionNames.slice(0, 4)).toEqual(['Term 1 · S1 Q1', 'Term 2 · S1 Q2', 'Term 3 · S2 Q1', 'Term 4 · S2 Q2']);
  expect(regionNames.slice(4)).toHaveLength(3);
  expect(await first.evaluate(el => getComputedStyle(el).backgroundColor)).not.toEqual(await third.evaluate(el => getComputedStyle(el).backgroundColor));
  await expect(page.getByRole('region', { name: '暑期 · 未设置阶段', exact: true })).toContainText('暑期英语');
  await expect(page.getByRole('region', { name: 'S1 第一学期 · 未设置阶段', exact: true })).toContainText('未设置阶段的英语');
  await expect(page.getByRole('region', { name: '未设置学期', exact: true })).toContainText('历史英语范围');
  const locked = first.locator('.ant-card').filter({ hasText: '数学套餐范围' });
  await expect(locked.getByRole('checkbox', { name: '课程学习' })).toBeDisabled();
  await expect(locked.getByRole('checkbox', { name: '课程学习' })).toBeChecked();
  const firstCourse = first.locator('.ant-card').filter({ hasText: '英语上阶段' }).getByRole('checkbox', { name: '课程学习' });
  await firstCourse.check();
  await expect(first.locator('.ant-card').filter({ hasText: '英语上阶段' }).getByRole('checkbox', { name: '讲义查看' })).toBeChecked();
  await third.getByRole('checkbox', { name: '习题练习' }).check();
  await page.getByRole('group', { name: '科目筛选' }).getByRole('button', { name: /Mathematics/ }).click();
  await expect(third).toHaveCount(0);
  await page.getByRole('group', { name: '科目筛选' }).getByRole('button', { name: /全部/ }).click();
  await expect(firstCourse).toBeChecked();
  await expect(third.getByRole('checkbox', { name: '习题练习' })).toBeChecked();
  await page.mouse.move(0, 0);
  await page.locator('.ant-drawer-body').screenshot({ path: '/tmp/starline-term-groups-desktop.png' });
  await page.setViewportSize({ width: 390, height: 844 });
  await first.scrollIntoViewIfNeeded();
  expect(await first.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
  await page.locator('.ant-drawer-body').screenshot({ path: '/tmp/starline-term-groups-narrow.png' });
  await page.getByRole('region', { name: '未设置学期', exact: true }).scrollIntoViewIfNeeded();
  await page.locator('.ant-drawer-body').screenshot({ path: '/tmp/starline-term-groups-fallback.png' });
  await page.getByRole('button', { name: /保存变更/ }).click();
  await expect.poll(() => saved).toEqual({ studentId: student.id, revokeDirectLearningSpaceIds: [], selections: [
    { learningSpaceId: 'first', contentTypeCodes: ['course', 'handout', 'question', 'download'] },
    { learningSpaceId: 'second', contentTypeCodes: ['question'] }
  ] });
});
