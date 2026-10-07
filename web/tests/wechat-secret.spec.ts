import { test, expect } from '@playwright/test';

test('saved secrets reveal on demand and never enter save payload', async ({ page }) => {
  const user = { id: 'super', name: '配置测试', phone: '13900000001', roles: ['super_admin'], roleLabel: '超级管理员', mustChangePassword: false };
  const settings = { miniProgramName: '测试小程序', miniProgramAppId: 'wx1234567890', officialAccountName: '测试公众号', officialAccountAppId: 'wx0987654321', officialAccountOriginalId: 'gh_fixture', miniProgramSecretConfigured: true, officialAccountSecretConfigured: true, callbackTokenConfigured: true, encodingAesKeyConfigured: true };
  const reveals: string[] = [];
  let fail = false;
  let saved: Record<string, string> | undefined;
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace('/api', '');
    let data: unknown = [];
    if (path === '/auth/me') data = user;
    if (path === '/settings') data = {};
    if (path === '/wechat/settings') {
      data = settings;
      if (request.method() === 'PUT') saved = request.postDataJSON();
    }
    if (path === '/wechat/settings/reveal') {
      const field = request.postDataJSON().field;
      reveals.push(field);
      if (fail) { await route.fulfill({ status: 400, json: { code: 400, message: '读取失败', data: null } }); return; }
      data = { value: `fixture-${field}` };
    }
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.goto('/settings');
  await page.getByRole('tab', { name: '小程序与公众号' }).click();
  const fields = ['miniProgramAppSecret', 'officialAccountAppSecret', 'callbackToken', 'encodingAesKey'];
  expect(reveals).toEqual([]);
  for (const field of fields) {
    const input = page.locator(`#${field}`);
    const item = input.locator('xpath=ancestor::*[contains(@class,"ant-form-item-control-input-content")]');
    await item.getByRole('button', { name: '显示密钥', exact: true }).click();
    await expect(input).toHaveValue(`fixture-${field}`);
    await expect(input).toHaveAttribute('type', 'text');
    await item.getByRole('button', { name: '隐藏密钥', exact: true }).click();
    await expect(input).toHaveValue('');
    await expect(input).toHaveAttribute('type', 'password');
  }
  const token = page.locator('#callbackToken');
  const tokenItem = token.locator('xpath=ancestor::*[contains(@class,"ant-form-item-control-input-content")]');
  fail = true;
  await tokenItem.getByRole('button', { name: '显示密钥', exact: true }).click();
  await expect(page.getByText('读取失败', { exact: true })).toBeVisible();
  await expect(token).toHaveValue('');
  fail = false;
  await tokenItem.getByRole('button', { name: '显示密钥', exact: true }).click();
  await expect(token).toHaveValue('fixture-callbackToken');
  await page.getByRole('button', { name: /保存/ }).last().click();
  await expect.poll(() => saved).toBeTruthy();
  for (const field of fields) expect(saved?.[field] || '').toBe('');
  await expect(token).toHaveValue('');
  await token.fill('edited-token');
  const before = reveals.length;
  await tokenItem.getByRole('button', { name: '显示密钥', exact: true }).click();
  await expect(token).toHaveValue('edited-token');
  expect(reveals.length).toBe(before);
  await page.screenshot({ path: '/tmp/starline-wechat-secret.png', fullPage: true });
});
