import { test, expect, type Page } from '@playwright/test';

async function fixture(page: Page, view: 'card' | 'table') {
  const user = { userId: 'super', name: '管理员', roles: ['super_admin'] };
  let staff: Record<string, unknown>[] = [];
  let resetCount = 0;
  await page.addInitScript(({ user, view }) => {
    localStorage.setItem('starline_admin_token', 'staff-ui-fixture');
    localStorage.setItem('starline_admin_user', JSON.stringify(user));
    localStorage.setItem('starline:list-view:admin-staff', view);
  }, { user, view });
  await page.route('**/api/**', async route => {
    const req = route.request();
    const path = new URL(req.url()).pathname.replace('/api', '');
    let data: unknown = [];
    if (path === '/auth/me') data = user;
    if (path === '/admin-staff' && req.method() === 'GET') data = staff;
    if (path === '/admin-staff' && req.method() === 'POST') {
      const created = { ...req.postDataJSON(), id: 'staff1', bindStatus: '待绑定', accountStatus: '正常', passwordEnabled: true, mustChangePassword: true };
      staff = [created];
      data = { ...created, temporaryPassword: 'Fixture2026Create' };
    }
    if (path === '/admin-staff/staff1/reset-password') {
      resetCount++;
      staff = staff.map(row => ({ ...row, mustChangePassword: true }));
      data = { userId: 'staff1', temporaryPassword: 'Fixture2026Reset', mustChangePassword: true };
    }
    if (path === '/admin-staff/staff1' && req.method() === 'PUT') {
      staff = [{ ...staff[0], ...req.postDataJSON() }]; data = staff[0];
    }
    await route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/admin-staff');
  return () => resetCount;
}

for (const view of ['card', 'table'] as const) {
  test(`${view}: create handoff, reset confirmation and disabled state`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    const resetCount = await fixture(page, view);
    await page.getByRole('button', { name: '新增人员' }).click();
    const drawer = page.getByRole('dialog', { name: '新增管理人员' });
    await expect(drawer).toContainText('保存后生成临时密码');
    await drawer.getByLabel('姓名', { exact: true }).fill('测试教务');
    await drawer.getByLabel('手机号', { exact: true }).fill('13911223344');
    await drawer.getByRole('button', { name: /保\s*存/ }).click();
    let handoff = page.getByRole('dialog', { name: '交接登录信息' });
    await expect(handoff).toContainText('测试教务 · 13911223344');
    await expect(handoff).toContainText('Fixture2026Create');
    await expect(handoff.getByRole('link')).toHaveAttribute('href', 'http://127.0.0.1:5189/login');
    await expect(handoff).toContainText('复制完整登录信息');
    await handoff.getByRole('button', { name: '我已记录' }).click();
    await page.reload();
    await expect(page.getByText('首次登录需改密', { exact: true })).toBeVisible();
    const reset = page.getByRole('button', { name: '重置密码', exact: true });
    await expect(reset).toBeInViewport();
    await page.screenshot({ path: testInfo.outputPath(`staff-${view}.png`), fullPage: true });
    await reset.click();
    const confirmation = page.getByRole('dialog', { name: '重置管理人员密码' });
    await expect(confirmation).toContainText('测试教务（13911223344）');
    await confirmation.getByRole('button', { name: /取\s*消/ }).click();
    expect(resetCount()).toBe(0);
    await reset.click();
    await confirmation.getByRole('button', { name: '确认重置' }).click();
    handoff = page.getByRole('dialog', { name: '交接登录信息' });
    await expect(handoff).toContainText('Fixture2026Reset');
    expect(resetCount()).toBe(1);
    await handoff.getByRole('button', { name: '我已记录' }).click();
    await page.getByRole('button', { name: '编辑', exact: true }).click();
    const edit = page.getByRole('dialog', { name: '编辑管理人员' });
    await edit.getByRole('switch').click();
    await edit.getByRole('button', { name: /保\s*存/ }).click();
    await expect(page.getByText('不可登录', { exact: true })).toBeVisible();
    await expect(reset).toBeDisabled();
    await page.reload();
    await expect(page.getByText('不可登录', { exact: true })).toBeVisible();
  });
}
