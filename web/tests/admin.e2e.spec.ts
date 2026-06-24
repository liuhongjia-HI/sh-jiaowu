import { expect, test, type Page } from '@playwright/test';

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
    ['/teachers', '教师管理'],
    ['/logs', '操作记录'],
    ['/settings', '系统设置']
  ];

  for (const [path, heading] of pages) {
    await expectPageHeading(page, path, heading);
  }
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

test('退出登录会清理后台访问态', async ({ page }) => {
  await login(page, '13800000002');
  await page.getByLabel('退出登录').click();
  await expect(page.getByRole('heading', { name: 'Starline 教务后台' })).toBeVisible();

  await page.goto('/dashboard');
  await expect(page).toHaveURL(/\/login$/);
});
