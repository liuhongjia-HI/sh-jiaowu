import { expect, test, type Locator, type Page } from '@playwright/test';

async function login(page: Page, phone: string, password = '123456') {
  await page.goto('/login');
  await page.getByLabel('手机号').fill(phone);
  await page.getByLabel('密码').fill(password);
  await page.getByRole('button', { name: '进入工作台' }).click();
  await expect(page.getByRole('heading', { name: '今日待办' })).toBeVisible();
}

async function expectPageHeading(page: Page, path: string, heading: string) {
  await page.goto(path);
  await expect(page.getByRole('heading', { name: heading })).toBeVisible();
}

async function selectOption(page: Page, container: Locator, fieldName: string, optionText: string) {
  const field = container.locator('.ant-form-item').filter({ hasText: fieldName }).first();
  await field.locator('.ant-select-selector').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText(optionText, { exact: false }).last().click();
}

async function ensureCompactOption(page: Page, container: Locator, index: number, optionText: string) {
  const selector = container.locator('.ant-form .ant-select-selector').nth(index);
  const current = await selector.innerText();
  if (current.includes(optionText)) return;
  await selector.click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText(optionText, { exact: false }).last().click();
}

async function createQuestion(page: Page, title: string, typeText: string, stem: string, score: string, options?: string[], answer?: string) {
  await expectPageHeading(page, '/questions', '题库');
  await page.getByRole('button', { name: '新增题目' }).click();
  const dialog = page.getByRole('dialog', { name: '新增题库题目' });
  await expect(dialog).toBeVisible();

  await ensureCompactOption(page, dialog, 0, '五年级');
  await ensureCompactOption(page, dialog, 1, 'S1');
  await ensureCompactOption(page, dialog, 2, '英文');
  await dialog.getByLabel('题目名称').fill(title);
  await selectOption(page, dialog, '题型', typeText);
  await dialog.getByPlaceholder('请输入学生看到的题目内容，可添加重点、列表或图片 URL。').fill(stem);
  if (options) {
    for (const [index, option] of options.entries()) {
      await dialog.getByPlaceholder(`请输入选项 ${String.fromCharCode(65 + index)} 内容`).fill(option);
    }
  }
  if (answer) {
    await dialog.locator('label.ant-radio-wrapper').filter({ hasText: answer }).click();
  }
  await dialog.getByLabel('分值').fill(score);
  await dialog.getByRole('button', { name: '保存' }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText(title)).toBeVisible();
}

test.beforeEach(async ({ page }) => {
  await page.goto('/login');
  await page.evaluate(() => localStorage.clear());
});

test('未登录访问管理后台会回到登录页', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: 'Starline 教务后台' })).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
});

test('超级管理员可以打开管理后台全部一级功能页', async ({ page }) => {
  await login(page, '13800000001');

  const pages: Array<[string, string]> = [
    ['/dashboard', '今日待办'],
    ['/students', '学生管理'],
    ['/packages', '课程方案'],
    ['/content', '课程内容'],
    ['/scheduling', '排课管理'],
    ['/materials', '课程讲义'],
    ['/homework', '课后练习'],
    ['/review', '批改反馈'],
    ['/commercial', '商业运营'],
    ['/notices', '通知提醒'],
    ['/admin-staff', '管理人员'],
    ['/teachers', '老师管理'],
    ['/logs', '操作记录'],
    ['/settings', '系统设置']
  ];

  for (const [path, heading] of pages) {
    await expectPageHeading(page, path, heading);
  }
});

test('题库和课程内容可以切换卡片与表格视图', async ({ page }) => {
  await login(page, '13800000001');

  for (const [path, heading, storageKey] of [
    ['/questions', '题库', 'starline:list-view:questions'],
    ['/content', '课程内容', 'starline:list-view:courses']
  ] as const) {
    await expectPageHeading(page, path, heading);
    const toggle = page.getByLabel(`列表视图：${storageKey}`);
    await toggle.getByText('卡片').click();
    await expect(page.locator('.card-list-grid .info-card').first()).toBeVisible();
    await toggle.getByText('表格').click();
    await expect(page.locator('.ant-table-thead').first()).toBeVisible();
  }
});

test('新增课程方案默认带出当前学年', async ({ page }) => {
  await login(page, '13800000001');
  await expectPageHeading(page, '/packages', '课程方案');

  await page.getByRole('button', { name: '新增方案' }).click();
  const drawer = page.getByRole('dialog', { name: '新增课程方案' });
  await expect(drawer).toBeVisible();
  await expect(drawer).toHaveClass(/ant-drawer-content/);
  await expect(drawer.getByText('2026.2027学年', { exact: true })).toBeVisible();
});

test('点击课程方案名称可查看该方案的课程讲义', async ({ page }) => {
  await login(page, '13800000001');
  await expectPageHeading(page, '/packages', '课程方案');

  const firstRow = page.locator('.ant-table-tbody tr').first();
  const packageLink = firstRow.getByRole('link');
  const packageName = (await packageLink.innerText()).trim();
  await packageLink.click();

  await expect(page).toHaveURL(/\/content\?tab=materials&packageId=/);
  await expect(page.getByRole('heading', { name: '课程讲义' })).toBeVisible();
  await expect(page.getByText(`正在查看“${packageName}”套餐包含的全部课程讲义。`)).toBeVisible();
  await expect(page.getByRole('button', { name: '查看全部讲义' })).toBeVisible();
});

test('教师账号不能进入运营和系统高权限功能', async ({ page }) => {
  await login(page, '13800000004');

  await expect(page.getByText('商业运营')).toHaveCount(0);
  await expect(page.getByText('管理人员')).toHaveCount(0);
  await expect(page.getByText('系统设置')).toHaveCount(0);

  await page.goto('/commercial');
  await expect(page.getByText('当前账号不能访问这个功能')).toBeVisible();

  await page.goto('/admin-staff');
  await expect(page.getByText('当前账号不能访问这个功能')).toBeVisible();
});

test('学生表格将年级和学校显示在学生列之前', async ({ page }) => {
  await login(page, '13800000002');

  await expectPageHeading(page, '/students', '学生管理');
  await page.getByLabel('列表视图：starline:list-view:students').getByText('表格').click();
  const studentTableHeaders = await page.locator('.student-table thead th').allTextContents();
  expect(studentTableHeaders.slice(0, 5).map((header) => header.trim())).toEqual(['年级', '学校', '学生', '操作', '家长姓名']);
});

test('学生列表直接展示辅导老师并在悬停时显示匹配详情', async ({ page }) => {
  await login(page, '13800000002');
  await page.route('**/api/students*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue();
      return;
    }
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        message: 'ok',
        data: [{
          id: 'assigned-student', name: '已分配老师学生', phone: '13900000156', grade: '五年级', schoolName: '星河小学',
          openedPackages: [], openedPackageRefs: [], learningStatus: '未开始', accountStatus: '正常',
          streakDays: 0, averageScore: 0, badgeCount: 0, bindStatus: '待绑定', createdAt: '2026-08-30 20:00:00',
          activeTutoringAssignments: [{ teacherId: 'user-teacher', teacherName: '英语老师', subjectName: '英文', levelCode: 'S', role: 'primary', startsAt: '2026-08-30' }]
        }]
      })
    });
  });

  await expectPageHeading(page, '/students', '学生管理');
  await page.getByLabel('列表视图：starline:list-view:students').getByText('表格').click();
  await expect(page.locator('.student-table thead')).toContainText('辅导老师');
  const teacherTag = page.locator('.student-table tbody tr', { hasText: '已分配老师学生' }).getByText('英语老师', { exact: true });
  await teacherTag.hover();
  await expect(page.getByText('英语老师 · 主辅导 · 英文 S级 · 2026-08-30 起')).toBeVisible();
});

test('新增教师时可按年级和学科筛选负责学习空间', async ({ page }) => {
  await login(page, '13800000001');
  await expectPageHeading(page, '/teachers', '老师管理');
  await page.getByRole('button', { name: '新增教师' }).click();
  const drawer = page.getByRole('dialog', { name: '新增教师' });
  await expect(drawer.getByText('快捷筛选负责范围')).toBeVisible();

  const filter = drawer.locator('.ant-form-item').filter({ hasText: '快捷筛选负责范围' });
  await filter.locator('.ant-select-selector').first().click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('五年级', { exact: true }).last().click();
  await filter.locator('.ant-select-selector').nth(1).click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('英文', { exact: true }).last().click();

  const scope = drawer.locator('.ant-form-item').filter({ hasText: '负责学习空间' }).last();
  await scope.locator('.ant-select-selector').click();
  await expect(page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').last()).toContainText('五年级英文');
});

test('未开通课程学生在表格中高亮提示运营跟进', async ({ page }) => {
  await login(page, '13800000002');
  await page.route('**/api/students*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue();
      return;
    }
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        message: 'ok',
        data: [{
          id: 'follow-up-highlight', name: '运营待跟进高亮学生', phone: '13900000157', grade: '五年级', schoolName: '星河小学',
          openedPackages: [], openedPackageRefs: [], learningStatus: '未开始', accountStatus: '正常', followUpStatus: '待跟进',
          streakDays: 0, averageScore: 0, badgeCount: 0, bindStatus: '待绑定', createdAt: '2026-08-30 20:00:00'
        }]
      })
    });
  });

  await expectPageHeading(page, '/students', '学生管理');
  await page.getByLabel('列表视图：starline:list-view:students').getByText('表格').click();
  const followUpRow = page.locator('.student-table tbody tr', { hasText: '运营待跟进高亮学生' });
  await expect(followUpRow).toBeVisible();
  await expect(followUpRow).toHaveClass(/student-follow-up-row/);
});

test('学生列表会按剩余时间提示已开通课程的结束时间', async ({ page }) => {
  await login(page, '13800000002');
  const formatEndTime = (daysFromNow: number) => {
    const date = new Date();
    date.setDate(date.getDate() + daysFromNow);
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day} 23:59:59`;
  };
  const warningEndTime = formatEndTime(10);
  const urgentEndTime = formatEndTime(5);

  await page.route('**/api/students*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue();
      return;
    }
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        message: 'ok',
        data: [
          {
            id: 'expiry-warning', name: '两周内结束学生', phone: '13900000158', grade: '五年级', schoolName: '星河小学',
            openedPackages: ['五年级英语'], openedPackageRefs: [{ packageId: 'pkg-warning', packageName: '五年级英语' }], effectiveUntil: warningEndTime,
            learningStatus: '已开通', accountStatus: '正常', streakDays: 0, averageScore: 0, badgeCount: 0, bindStatus: '待绑定', createdAt: '2026-08-30 20:00:00'
          },
          {
            id: 'expiry-urgent', name: '一周内结束学生', phone: '13900000159', grade: '五年级', schoolName: '星河小学',
            openedPackages: ['五年级数学'], openedPackageRefs: [{ packageId: 'pkg-urgent', packageName: '五年级数学' }], effectiveUntil: urgentEndTime,
            learningStatus: '已开通', accountStatus: '正常', streakDays: 0, averageScore: 0, badgeCount: 0, bindStatus: '待绑定', createdAt: '2026-08-30 20:00:00'
          }
        ]
      })
    });
  });

  await expectPageHeading(page, '/students', '学生管理');
  await page.getByLabel('列表视图：starline:list-view:students').getByText('表格').click();

  const warningRow = page.locator('.student-table tbody tr', { hasText: '两周内结束学生' });
  const urgentRow = page.locator('.student-table tbody tr', { hasText: '一周内结束学生' });
  const warningReminder = warningRow.getByText(`结束时间：${warningEndTime.slice(0, 10)}`);
  const urgentReminder = urgentRow.getByText(`结束时间：${urgentEndTime.slice(0, 10)}`);
  await expect(warningReminder).toHaveClass(/student-package-expiry-warning/);
  await expect(urgentReminder).toHaveClass(/student-package-expiry-urgent/);
  await expect(warningReminder).not.toContainText('23:59:59');
  await expect(urgentReminder).not.toContainText('23:59:59');
});

test('学生列表课程列只展示开通方案摘要', async ({ page }) => {
  await login(page, '13800000002');
  await page.route('**/api/students*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue();
      return;
    }
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        message: 'ok',
        data: [{
          id: 'plan-summary', name: '方案摘要学生', phone: '13900000160', grade: '五年级', schoolName: '星河小学',
          openedPackages: ['五年级英语 S1 方案'], openedPackageRefs: [{
            packageId: 'pkg-plan', packageName: '五年级英语 S1 方案',
            openCourses: ['验证五年级英语课程-07110752'], openMaterials: ['验证讲义'], openHomework: ['验证练习']
          }],
          learningStatus: '已开通', accountStatus: '正常', streakDays: 0, averageScore: 0, badgeCount: 0, bindStatus: '待绑定', createdAt: '2026-08-30 20:00:00'
        }]
      })
    });
  });

  await expectPageHeading(page, '/students', '学生管理');
  await page.getByLabel('列表视图：starline:list-view:students').getByText('表格').click();
  const row = page.locator('.student-table tbody tr', { hasText: '方案摘要学生' });
  await expect(row.getByText('五年级英语 S1 方案', { exact: true })).toBeVisible();
  await expect(row.getByText(/课程：|讲义：|题目：/)).toHaveCount(0);
});

test('iPad 横屏时学生姓名完整显示且表格可横向查看', async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await login(page, '13800000002');

  await expectPageHeading(page, '/students', '学生管理');
  await page.getByLabel('列表视图：starline:list-view:students').getByText('表格').click();
  const firstName = page.locator('.student-table .student-name').first();
  await expect(firstName).toBeVisible();
  await expect(firstName).not.toHaveText('');
  const table = page.locator('.student-table .ant-table-content');
  await expect.poll(() => table.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(true);
});

test('点击学生课程标签查看该学生已开通的课程', async ({ page }) => {
  await login(page, '13800000002');
  await expectPageHeading(page, '/students', '学生管理');
  await page.getByLabel('列表视图：starline:list-view:students').getByText('表格').click();

  const packageLink = page.locator('.student-table tbody tr a').first();
  const row = packageLink.locator('xpath=ancestor::tr');
  const studentName = (await row.locator('.student-name').innerText()).trim();
  const packageName = (await packageLink.innerText()).trim();
  await packageLink.click();

  await expect(page).toHaveURL(/\/students$/);
  const drawer = page.locator('.ant-drawer-content').last();
  await expect(drawer).toBeVisible();
  await expect(drawer.getByText(studentName, { exact: true }).first()).toBeVisible();
  await expect(drawer.getByRole('tab', { name: '开通课程' })).toHaveAttribute('aria-selected', 'true');
  await expect(drawer.getByText(packageName, { exact: true })).toBeVisible();
  await expect(drawer.getByText('开放课程', { exact: true })).toBeVisible();
});

test('校区管理员可以在学生管理直接开通课程', async ({ page }) => {
  await login(page, '13800000002');

  await expectPageHeading(page, '/students', '学生管理');
  await expect(page.getByRole('button', { name: '新增学生' })).toBeVisible();
  await expect(page.getByRole('button', { name: '批量导入' })).toBeVisible();
  await page.getByRole('button', { name: '开通学习内容' }).first().click();
  const drawer = page.locator('.ant-drawer-content').last();
  await expect(drawer).toBeVisible();
  await drawer.getByRole('tab', { name: '开通学习内容' }).click();
  await expect(drawer.getByText('按需开通学习内容')).toBeVisible();
  await expect(drawer.getByText('课程范围与学习内容', { exact: true })).toBeVisible();
  await expect(drawer.getByText(/当前年级：/)).toBeVisible();
  const subjectFilter = drawer.getByRole('group', { name: '科目筛选' });
  await expect(subjectFilter).toBeVisible();
  await expect(subjectFilter.getByRole('button', { name: /全部（\d+）/ })).toBeVisible();
  const englishFilter = subjectFilter.getByRole('button', { name: /英文（\d+）/ });
  await expect(englishFilter).toBeVisible();

  await englishFilter.click();
  const firstEnglishContent = drawer.getByLabel(/^五年级英文.*学习内容$/).first();
  const firstEnglishCourse = firstEnglishContent.getByRole('checkbox', { name: '课程', exact: true });
  await firstEnglishCourse.check();
  await expect(drawer.getByText('已选 1 个课程范围')).toBeVisible();
  await expect(firstEnglishCourse).toBeChecked();

  await expect(firstEnglishContent.getByRole('checkbox', { name: '课程', exact: true })).toBeVisible();
  await expect(firstEnglishContent.getByRole('checkbox', { name: '题目', exact: true })).toBeVisible();
  await expect(firstEnglishContent.getByRole('checkbox', { name: '讲义', exact: true })).toBeVisible();
  await expect(drawer.getByLabel('开通时间')).toBeVisible();
  await expect(drawer.getByLabel('结束时间')).toBeVisible();
  await expect(drawer.getByText('课程方案', { exact: true })).toHaveCount(0);

  await page.goto('/open');
  await expect(page).toHaveURL(/\/students$/);
  await expect(page.getByRole('heading', { name: '学生管理' })).toBeVisible();
  await page.goto('/permissions');
  await expect(page).toHaveURL(/\/students$/);
});

test('教师可以进入题库并打开手动组卷入口', async ({ page }) => {
  await login(page, '13800000004');

  await expectPageHeading(page, '/questions', '题库');
  await page.getByRole('button', { name: '新增题目' }).click();
  const questionDialog = page.getByRole('dialog', { name: '新增题库题目' });
  await expect(questionDialog).toBeVisible();
  await expect(questionDialog).toHaveClass(/ant-drawer-content/);
  await questionDialog.getByRole('button', { name: '取消' }).click();

  await expectPageHeading(page, '/homework', '课后练习');
  await page.getByRole('button', { name: '手动组卷' }).click();
  const homeworkDialog = page.getByRole('dialog', { name: '组卷发布小挑战' });
  await expect(homeworkDialog).toBeVisible();
  await expect(page.getByText('先选课程，再从同年级、同学期、同学科的题库中手动选题组卷。')).toBeVisible();
});

test('教师可以新增题目并手动组卷发布小挑战', async ({ page }) => {
  test.setTimeout(90_000);
  await login(page, '13800000004');
  const suffix = Date.now();
  const singleTitle = `验收单选题 ${suffix}`;
  const textTitle = `验收简答题 ${suffix}`;
  const homeworkTitle = `验收组卷小挑战 ${suffix}`;

  await createQuestion(page, singleTitle, '单选题', 'Which word means apple?', '10', ['apple', 'book', 'desk', 'pen'], 'apple');
  await createQuestion(page, textTitle, '简答题', '用一句话说说今天学到的阅读方法。', '20');

  await expectPageHeading(page, '/homework', '课后练习');
  await page.getByRole('button', { name: '手动组卷' }).click();
  const dialog = page.getByRole('dialog', { name: '组卷发布小挑战' });
  await expect(dialog).toBeVisible();
  await dialog.getByLabel('练习标题').fill(homeworkTitle);
  await selectOption(page, dialog, '课程范围', '五年级英文S1Q1课程');
  await dialog.getByLabel('截止时间').fill('2026-12-31T18:00');
  await dialog.getByRole('checkbox', { name: singleTitle, exact: false }).check();
  await dialog.getByRole('checkbox', { name: textTitle, exact: false }).check();
  await dialog.getByRole('button', { name: '发布' }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText(homeworkTitle)).toBeVisible();
});

test('校区管理员可以从周历入口新建排课', async ({ page }) => {
  await login(page, '13800000002');

  await expectPageHeading(page, '/scheduling', '排课管理');
  await expect(page.getByText('排班工作台', { exact: false }).first()).toBeVisible();
  // 默认落在资源泳道日视图，这条用例验的是周视图入口，先切过去。
  await page.locator('.ant-segmented-item-label', { hasText: '周视图' }).click();
  await expect(page.locator('.schedule-timeline-grid')).toBeVisible();
  await expect(page.locator('.schedule-day-head')).toHaveCount(7);
  // 侧栏默认收起，展开后才该看到学科日历。
  await expect(page.getByText('学科日历')).toBeHidden();
  await page.locator('.schedule-sidebar-rail').click();
  await expect(page.getByText('学科日历')).toBeVisible();
  await expect(page.getByText('老师可授课').first()).toBeVisible();
  await expect(page.getByText('学生可上课').first()).toBeVisible();
  await expect(page.locator('.schedule-timeline-grid')).toContainText('19:00-21:00');
  await expect(page.locator('.schedule-timeline-grid')).toContainText('英语老师');
  await expect(page.getByText('教室/资源')).toHaveCount(0);
  await page.locator('.schedule-day-empty-slot').first().click();
  await expect(page.getByRole('dialog', { name: '新建课程' })).toBeVisible();
  await expect(page.getByRole('dialog', { name: '新建课程' }).getByText('教室/资源')).toHaveCount(0);
});

test('校区管理员可以右键复制课程并只修改日期创建新课', async ({ page }) => {
  await login(page, '13800000002');

  const { sourceDate, copiedDate, startTime, endTime } = await page.evaluate(async () => {
    const formatDate = (date: Date) => {
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      return `${year}-${month}-${day}`;
    };
    const source = new Date();
    const copied = new Date(source);
    copied.setDate(copied.getDate() + 1);
    const token = localStorage.getItem('starline_admin_token');
    const user = JSON.parse(localStorage.getItem('starline_admin_user') ?? '{}') as { userId?: string; name?: string };
    const headers = {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      'X-Operator-ID': user.userId ?? '',
      'X-Operator-Name': encodeURIComponent(user.name ?? '')
    };
    const existingResponse = await fetch('/api/schedule-classes', { headers });
    if (!existingResponse.ok) throw new Error(`读取现有课程失败：${await existingResponse.text()}`);
    const existingBody = await existingResponse.json() as { data?: Array<{ teacherId: string; lessonDate: string; startTime: string; endTime: string; status: string }> };
    const sourceDate = formatDate(source);
    const copiedDate = formatDate(copied);
    const overlaps = (lessonDate: string, candidateStart: string, candidateEnd: string) =>
      (existingBody.data ?? []).some((item) => item.status !== '已取消' && item.teacherId === 'user-teacher'
        && item.lessonDate === lessonDate && candidateStart < item.endTime && candidateEnd > item.startTime);
    const availableSlot = Array.from({ length: 24 }, (_, index) => {
      const hour = String(index).padStart(2, '0');
      const nextHour = String(index + 1).padStart(2, '0');
      return { startTime: `${hour}:00`, endTime: `${nextHour}:00` };
    }).find((slot) => !overlaps(sourceDate, slot.startTime, slot.endTime) && !overlaps(copiedDate, slot.startTime, slot.endTime));
    if (!availableSlot) throw new Error('今天和明天没有可用于快捷复制验收的空闲时段');
    const response = await fetch('/api/schedule-classes', {
      method: 'POST',
      headers,
      body: JSON.stringify({
        courseId: 'course-g05-english-s1-q1',
        teacherId: 'user-teacher',
        campusId: 'campus-main',
        classType: '1V1',
        durationMinutes: 60,
        startTime: availableSlot.startTime,
        endTime: availableSlot.endTime,
        startDate: sourceDate,
        studentIds: [],
        expectedStudentCount: 1,
        reservationNote: '快捷复制端到端验收源课程',
        ignoreWarnings: true
      })
    });
    if (!response.ok) throw new Error(`创建验收源课程失败：${await response.text()}`);
    return { sourceDate, copiedDate, ...availableSlot };
  });

  await expectPageHeading(page, '/scheduling', '排课管理');
  await expect(page.getByText('右键课程可快速复制')).toBeVisible();
  const sourceClass = page.locator('.schedule-timeline-block.is-class').filter({ hasText: `${startTime}-${endTime}` }).first();
  await expect(sourceClass).toBeVisible();
  await sourceClass.click({ button: 'right' });
  await page.getByRole('menuitem', { name: '复制这节课' }).dispatchEvent('click');

  const drawer = page.getByRole('dialog', { name: '复制课程' });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByText('已带入原课程信息，请修改上课日期、时间或其他属性。')).toBeVisible();
  await expect(drawer.getByLabel('上课日期')).toHaveValue(sourceDate);
  await drawer.getByLabel('上课日期').fill(copiedDate);
  await drawer.getByRole('button', { name: '创建复制课程' }).click();

  await expect(drawer).toBeHidden();
  await expect(page.getByText('复制课程已创建，课表已更新')).toBeVisible();
});

test('校区管理员可以按固定周次和多个星期创建重复课程', async ({ page }) => {
  await login(page, '13800000002');
  let submitted: Record<string, any> | null = null;
  await page.route('**/api/schedule-classes', async (route) => {
    if (route.request().method() !== 'POST') {
      await route.continue();
      return;
    }
    submitted = route.request().postDataJSON() as Record<string, any>;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        message: 'ok',
        data: { id: 'schedule-repeat-e2e', status: '已确认', auditStatus: '已通过' }
      })
    });
  });

  await expectPageHeading(page, '/scheduling', '排课管理');
  await page.locator('.ant-segmented-item-label', { hasText: '周视图' }).click();
  await page.locator('.schedule-day-empty-slot').first().click();
  const drawer = page.getByRole('dialog', { name: '新建课程' });
  await expect(drawer).toBeVisible();
  for (const fieldName of ['课程', '老师']) {
    const field = drawer.locator('.ant-form-item').filter({ hasText: fieldName }).first();
    await field.locator('.ant-select-selector').click();
    await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').last().locator('.ant-select-item-option').first().click();
  }

  const repeatSection = drawer.locator('.ant-form-item').filter({ hasText: '重复' }).last();
  await repeatSection.locator('.ant-switch').click();
  await expect(repeatSection.getByText('支持按日或按周自定义重复周期；固定周次例如隔周上课，设置为“每 2 周”。')).toBeVisible();
  await expect(repeatSection.getByText('按月和特殊日期重复将在后续开放。')).toBeVisible();
  const repeatSelects = repeatSection.locator('.ant-select-selector');
  await repeatSelects.nth(0).click();
  const repeatModeDropdown = page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)');
  await expect(repeatModeDropdown.getByText('按日', { exact: true })).toBeVisible();
  await expect(repeatModeDropdown.getByText('按月', { exact: true })).toHaveCount(0);
  await repeatModeDropdown.getByText('按周', { exact: true }).click();
  await repeatSection.locator('.ant-input-number-input').first().fill('2');
  await repeatSelects.nth(2).click();
  const weekdayDropdown = page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').last();
  await weekdayDropdown.getByText('周一', { exact: true }).click();
  await weekdayDropdown.getByText('周三', { exact: true }).click();
  await repeatSection.locator('.ant-input-number-input').last().fill('6');
  await drawer.getByRole('button', { name: '创建课程' }).click();

  await expect.poll(() => submitted).not.toBeNull();
  expect(submitted?.repeat).toEqual({ freq: 'weekly', interval: 2, byDay: [1, 3], count: 6 });
});

test('退出登录会清理后台访问态', async ({ page }) => {
  await login(page, '13800000002');
  await page.getByRole('button', { name: '账号菜单' }).click();
  await page.getByRole('menuitem', { name: /退出登录/ }).click();
  await expect(page.getByRole('heading', { name: 'Starline 教务后台' })).toBeVisible();

  await page.goto('/dashboard');
  await expect(page).toHaveURL(/\/login$/);
});
