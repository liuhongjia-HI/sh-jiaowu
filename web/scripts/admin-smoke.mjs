import { chromium } from 'playwright';

const baseURL = process.env.BASE_URL || 'http://127.0.0.1:5173';
const executablePath = process.env.CHROME_EXECUTABLE_PATH || '';
const headless = process.env.HEADLESS !== 'false';
const dryRun = process.env.ADMIN_SMOKE_DRY_RUN === '1';

const pages = [
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

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

async function waitForText(page, text) {
  await page.getByText(text).first().waitFor({ state: 'visible', timeout: 10_000 });
}

async function login(page, phone, password = '123456') {
  await page.goto(`${baseURL}/login`, { waitUntil: 'domcontentloaded' });
  await page.getByLabel('手机号').fill(phone);
  await page.getByLabel('密码').fill(password);
  await page.getByRole('button', { name: '进入工作台' }).click();
  await waitForText(page, '今日待办');
}

async function run() {
  if (dryRun) {
    assert(typeof chromium.launch === 'function', 'playwright chromium launcher is unavailable');
    assert(pages.length >= 16, 'admin page coverage is incomplete');
    console.log('admin smoke dry run ok');
    return;
  }

  const launchOptions = { headless, timeout: 15_000 };
  if (executablePath) launchOptions.executablePath = executablePath;
  const browser = await chromium.launch(launchOptions);
  const context = await browser.newContext();
  const page = await context.newPage();
  const failures = [];

  async function step(name, fn) {
    try {
      await fn();
      console.log(`ok - ${name}`);
    } catch (error) {
      failures.push({ name, error });
      console.error(`not ok - ${name}`);
      console.error(error instanceof Error ? error.stack || error.message : error);
    }
  }

  await step('未登录访问管理后台会回到登录页', async () => {
    await page.goto(`${baseURL}/dashboard`, { waitUntil: 'domcontentloaded' });
    await waitForText(page, 'Starline 教务后台');
    assert(page.url().includes('/login'), `expected login URL, got ${page.url()}`);
  });

  await step('超级管理员可以打开全部功能页', async () => {
    await context.clearCookies();
    await page.evaluate(() => localStorage.clear()).catch(() => undefined);
    await login(page, '13800000001');
    for (const [path, heading] of pages) {
      await page.goto(`${baseURL}${path}`, { waitUntil: 'domcontentloaded' });
      await waitForText(page, heading);
    }
  });

  await step('教师账号不能进入运营和系统高权限功能', async () => {
    await context.clearCookies();
    await page.goto(`${baseURL}/login`, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => localStorage.clear());
    await login(page, '13800000004');
    assert(await page.getByText('商业运营').count() === 0, 'teacher should not see 商业运营');
    assert(await page.getByText('管理人员').count() === 0, 'teacher should not see 管理人员');
    assert(await page.getByText('系统设置').count() === 0, 'teacher should not see 系统设置');
    await page.goto(`${baseURL}/commercial`, { waitUntil: 'domcontentloaded' });
    await waitForText(page, '当前账号不能访问这个功能');
    await page.goto(`${baseURL}/admin-staff`, { waitUntil: 'domcontentloaded' });
    await waitForText(page, '当前账号不能访问这个功能');
  });

  await step('校区管理员可以打开学生开通和权限核查入口', async () => {
    await context.clearCookies();
    await page.goto(`${baseURL}/login`, { waitUntil: 'domcontentloaded' });
    await page.evaluate(() => localStorage.clear());
    await login(page, '13800000002');
    await page.goto(`${baseURL}/students`, { waitUntil: 'domcontentloaded' });
    await waitForText(page, '学生管理');
    await page.getByRole('button', { name: '新增学生' }).waitFor({ state: 'visible', timeout: 10_000 });
    await page.getByRole('button', { name: '批量导入' }).waitFor({ state: 'visible', timeout: 10_000 });
    await page.goto(`${baseURL}/open`, { waitUntil: 'domcontentloaded' });
    await waitForText(page, '选择学生和套餐');
    await waitForText(page, '本次学习权限预览');
    await page.goto(`${baseURL}/permissions`, { waitUntil: 'domcontentloaded' });
    await page.getByRole('tab', { name: '按学生查看' }).waitFor({ state: 'visible', timeout: 10_000 });
    await page.getByRole('tab', { name: '按套餐查看' }).waitFor({ state: 'visible', timeout: 10_000 });
    await page.getByRole('tab', { name: '按内容查看' }).waitFor({ state: 'visible', timeout: 10_000 });
  });

  await step('退出登录会清理后台访问态', async () => {
    await page.getByLabel('退出登录').click();
    await waitForText(page, 'Starline 教务后台');
    await page.goto(`${baseURL}/dashboard`, { waitUntil: 'domcontentloaded' });
    await waitForText(page, 'Starline 教务后台');
    assert(page.url().includes('/login'), `expected login URL after logout, got ${page.url()}`);
  });

  await browser.close();
  if (failures.length > 0) {
    throw new Error(`${failures.length} admin smoke checks failed`);
  }
}

run().catch((error) => {
  console.error(error instanceof Error ? error.stack || error.message : error);
  process.exit(1);
});
