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
  await dialog.locator('.ant-modal-footer .ant-btn-primary').click();
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
    ['/packages', '学习套餐'],
    ['/open', '开通套餐'],
    ['/permissions', '学习权限'],
    ['/content', '课程内容'],
    ['/scheduling', '排课管理'],
    ['/materials', '学习资料'],
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

test('新增学习套餐默认带出当前学年', async ({ page }) => {
  await login(page, '13800000001');
  await expectPageHeading(page, '/packages', '学习套餐');

  await page.getByRole('button', { name: '新增套餐' }).click();
  const dialog = page.getByRole('dialog', { name: '新增学习套餐' });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText('2026.2027学年', { exact: true })).toBeVisible();
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

test('校区管理员可以打开学生开通和权限核查入口', async ({ page }) => {
  await login(page, '13800000002');

  await expectPageHeading(page, '/students', '学生管理');
  await expect(page.getByRole('button', { name: '新增学生' })).toBeVisible();
  await expect(page.getByRole('button', { name: '批量导入' })).toBeVisible();

  await expectPageHeading(page, '/open', '开通套餐');
  await expect(page.getByText('选择学生和套餐')).toBeVisible();
  await expect(page.getByText('本次学习权限预览')).toBeVisible();

  await expectPageHeading(page, '/permissions', '学习权限');
  await expect(page.getByRole('tab', { name: '按学生查看' })).toBeVisible();
  await expect(page.getByRole('tab', { name: '按套餐查看' })).toBeVisible();
  await expect(page.getByRole('tab', { name: '按内容查看' })).toBeVisible();
});

test('教师可以进入题库并打开手动组卷入口', async ({ page }) => {
  await login(page, '13800000004');

  await expectPageHeading(page, '/questions', '题库');
  await page.getByRole('button', { name: '新增题目' }).click();
  const questionDialog = page.getByRole('dialog', { name: '新增题库题目' });
  await expect(questionDialog).toBeVisible();
  await page.locator('.question-dialog .ant-modal-footer button').filter({ hasText: /取\s*消/ }).click();

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
  await dialog.getByLabel('截止时间').fill('2026-12-31');
  await selectOption(page, dialog, '选择题目', singleTitle);
  await selectOption(page, dialog, '选择题目', textTitle);
  await dialog.locator('.ant-modal-footer .ant-btn-primary').click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText(homeworkTitle)).toBeVisible();
});

test('校区管理员可以从周历入口新建排课', async ({ page }) => {
  await login(page, '13800000002');

  await expectPageHeading(page, '/scheduling', '排课管理');
  await expect(page.getByText('周排班工作台')).toBeVisible();
  await expect(page.locator('.schedule-timeline-grid')).toBeVisible();
  await expect(page.locator('.schedule-day-head')).toHaveCount(7);
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

test('退出登录会清理后台访问态', async ({ page }) => {
  await login(page, '13800000002');
  await page.getByRole('button', { name: '账号菜单' }).click();
  await page.getByRole('menuitem', { name: /退出登录/ }).click();
  await expect(page.getByRole('heading', { name: 'Starline 教务后台' })).toBeVisible();

  await page.goto('/dashboard');
  await expect(page).toHaveURL(/\/login$/);
});
