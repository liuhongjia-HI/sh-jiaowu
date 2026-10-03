import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

test('actual teaching plan uploads render independent files and recover a missing preview', async ({ page, request }) => {
  test.skip(!process.env.STARLINE_REAL_API, 'launched by the isolated Go browser integration');
  const token = process.env.STARLINE_REAL_TOKEN!;
  const user = JSON.parse(process.env.STARLINE_REAL_USER!);
  const root = process.env.STARLINE_REAL_PLAN_FILES!;
  await page.addInitScript(({ token, user }) => { localStorage.setItem('starline_admin_token', token); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, { token, user });
  // All /api requests go through Vite's proxy to the actual temporary Go server.
  // There are no route mocks or simulated response bodies in this test.
  await page.goto('/teaching-plans');
  await page.getByRole('button', { name: /上传教案/ }).click();
  const drawer = page.getByRole('dialog', { name: '上传教案', exact: true });
  const select = async (label: string, option: string) => {
    const selector = drawer.locator('.ant-select').filter({ has: page.getByRole('combobox', { name: label, exact: true }) }).locator('.ant-select-selector');
    await selector.scrollIntoViewIfNeeded();
    await selector.click();
    if (label === '选择教案所属年级学科' || label === '教案课程目录') {
      await drawer.getByRole('combobox', { name: label, exact: true }).fill(option);
    }
    const listId = await drawer.getByRole('combobox', { name: label, exact: true }).getAttribute('aria-controls');
    await page.locator('.ant-select-dropdown').filter({ has: page.locator(`[id=${JSON.stringify(listId)}]`) }).locator('.ant-select-item-option').filter({ hasText: option }).click();
  };
  await select('选择教案所属年级学科', '五年级 · 英语');
  await select('教案课程目录', '真实联验教案目录');
  await select('教案章节', 'Unit 1 · 第一课');
  await drawer.getByLabel('选择教案文件').setInputFiles([join(root, '独立第一.pdf'), join(root, '独立第二.pdf')]);
  await select('第 2 份教案章节', 'Unit 2 · 第二课');
  await drawer.getByRole('button', { name: '上传 2 份', exact: true }).click();
  await expect(drawer).not.toBeVisible();
  const pendingResponse = await request.get(`${process.env.STARLINE_REAL_API}/api/teaching-plans/notification-batches`, { headers: { Authorization: `Bearer ${token}` } });
  expect(pendingResponse.ok()).toBeTruthy();
  expect((await pendingResponse.json()).data).toEqual([]);
  for (const name of ['独立第一', '独立第二']) {
    const row = page.getByRole('row').filter({ has: page.getByRole('button', { name, exact: true }) });
    await expect(row).toContainText('可预览', { timeout: 30000 });
    await page.getByRole('button', { name, exact: true }).click();
    const preview = page.getByRole('dialog', { name: new RegExp(`^${name}`) });
    const iframe = preview.locator('iframe');
    await expect(iframe).toBeVisible();
    const data = await iframe.evaluate(async el => Array.from(new Uint8Array(await (await fetch((el as HTMLIFrameElement).src)).arrayBuffer())));
    expect(Buffer.from(data)).toEqual(readFileSync(join(root, `${name}.pdf`)));
    const downloadEvent = page.waitForEvent('download');
    await preview.getByRole('button', { name: /下载原文件/ }).click();
    const download = await downloadEvent;
    expect(readFileSync((await download.path())!)).toEqual(readFileSync(join(root, `${name}.pdf`)));
    await preview.getByRole('button', { name: '返回列表', exact: true }).click();
    await expect(row.getByText('未读', { exact: true })).toHaveCount(0);
  }
  const listResponse = await request.get(`${process.env.STARLINE_REAL_API}/api/teaching-plans`, { headers: { Authorization: `Bearer ${token}` } });
  expect(listResponse.ok()).toBeTruthy();
  const list = (await listResponse.json()).data;
  const first = list.plans.find((plan: { title: string }) => plan.title === '独立第一');
  const removed = await request.post(`${process.env.STARLINE_REAL_API}/test/remove-preview?id=${first.id}`, { headers: { Authorization: `Bearer ${token}` } });
  expect(removed.status()).toBe(204);
  await page.getByRole('button', { name: '独立第一', exact: true }).click();
  const failed = page.getByRole('dialog', { name: /^独立第一/ });
  await expect(failed.getByRole('button', { name: '重试打开', exact: true })).toBeVisible();
  await failed.getByRole('button', { name: '返回列表', exact: true }).click();
  await page.getByRole('button', { name: /刷\s*新/ }).click();
  const firstRow = page.getByRole('row').filter({ has: page.getByRole('button', { name: '独立第一', exact: true }) });
  await expect(firstRow).toContainText('转换失败');
  await page.getByRole('button', { name: '独立第二', exact: true }).click();
  const secondPreview = page.getByRole('dialog', { name: /^独立第二/ });
  await expect(secondPreview.locator('iframe')).toBeVisible();
  await secondPreview.getByRole('button', { name: '返回列表', exact: true }).click();
  await page.getByRole('button', { name: '独立第一', exact: true }).click();
  await failed.getByRole('button', { name: '重新生成预览', exact: true }).click();
  await expect(failed.locator('iframe')).toBeVisible({ timeout: 30000 });
  const recovered = await failed.locator('iframe').evaluate(async el => Array.from(new Uint8Array(await (await fetch((el as HTMLIFrameElement).src)).arrayBuffer())));
  expect(Buffer.from(recovered)).toEqual(readFileSync(join(root, '独立第一.pdf')));
  await page.waitForLoadState('networkidle');
  await page.screenshot({ path: '/tmp/starline-real-teaching-plan-preview.png', fullPage: false });
  await failed.getByRole('button', { name: '返回列表', exact: true }).click();
  await page.goto(`/teaching-plans?plan=${encodeURIComponent(first.id)}`);
  const linkedPreview = page.getByRole('dialog', { name: /^独立第一/ });
  await expect(linkedPreview.locator('iframe')).toBeVisible();
  const linkedBytes = await linkedPreview.locator('iframe').evaluate(async el => Array.from(new Uint8Array(await (await fetch((el as HTMLIFrameElement).src)).arrayBuffer())));
  expect(Buffer.from(linkedBytes)).toEqual(readFileSync(join(root, '独立第一.pdf')));
  await linkedPreview.getByRole('button', { name: '返回列表', exact: true }).click();
  await expect(page).toHaveURL(/\/teaching-plans$/);
});
