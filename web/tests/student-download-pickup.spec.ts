import { test, expect } from '@playwright/test';
const challenge = 'a'.repeat(48), browserKey = 'b'.repeat(48), jobId = 'download-' + 'c'.repeat(32);
const job = { studentName: '张同学', scope: { subject: 'Geography' }, count: 1, size: 1024, materials: [{ fileName: 'Themes and Elements.pdf', unit: 'Unit 1', chapter: 'Chapter 1', lesson: 'Lesson 2' }] };
test('parent pickup opens without admin login and authorizes only this browser session', async ({ page }) => {
 let approved = false; let downloaded = 0; const secrets: string[] = [];
 await page.route('**/api/**', route => {
  const req = route.request(), path = new URL(req.url()).pathname;
  if (req.method() === 'POST') { expect(req.postDataJSON()).toEqual({ jobId }); return route.fulfill({ json: { code: 0, data: { challenge, browserKey, expiresAt: '2026-10-07T14:00:00Z' } } }); }
  secrets.push(req.headers().authorization);
  if (path.endsWith('/archive')) { downloaded++; return route.fulfill({ contentType: 'application/zip', body: 'zip-fixture' }); }
  return route.fulfill({ json: { code: 0, data: approved ? { status: 'approved', job } : { status: 'waiting' } } });
 });
 await page.goto(`/student-download?job=${jobId}`);
 await expect(page.getByText('等待手机确认 · 二维码有效期 10 分钟')).toBeVisible();
 await expect(page).not.toHaveURL(/login/);
 await expect(page.getByText('张同学 · Geography')).toHaveCount(0);
 await page.screenshot({ path: '/tmp/starline-student-pickup-waiting.png', fullPage: true });
 approved = true;
 await expect(page.getByRole('button', { name: '下载 ZIP 文件', exact: true })).toBeVisible();
 await expect(page.getByText('Themes and Elements.pdf')).toBeVisible();
 const download = page.waitForEvent('download'); await page.getByRole('button', { name: '下载 ZIP 文件', exact: true }).click();
 expect((await download).suggestedFilename()).toContain('Geography'); expect(downloaded).toBe(1);
 expect(secrets.every(key => key === `Bearer ${browserKey}`)).toBeTruthy();
 expect(page.url()).not.toContain(browserKey);
 await page.screenshot({ path: '/tmp/starline-student-pickup-approved.png', fullPage: true });
});
test('expired confirmation can refresh and a revoked package displays the server reason', async ({ page }) => {
 let starts = 0;
 await page.route('**/api/**', route => {
  const req = route.request();
  if (req.method() === 'POST') { starts++; return route.fulfill({ json: { code: 0, data: { challenge, browserKey, expiresAt: '2026-10-07T14:00:00Z' } } }); }
  if (starts === 1) return route.fulfill({ status: 400, json: { code: 400, message: '二维码已过期，请在电脑上刷新' } });
  if (new URL(req.url()).pathname.endsWith('/archive')) return route.fulfill({ status: 403, json: { code: 403, message: '资料或访问权限已变化，请重新生成下载包' } });
  return route.fulfill({ json: { code: 0, data: { status: 'approved', job } } });
 });
 await page.goto(`/student-download?job=${jobId}`);
 await expect(page.getByText('二维码已过期，请在电脑上刷新')).toBeVisible();
 await page.getByRole('button', { name: '刷新二维码' }).click();
 await page.getByRole('button', { name: '下载 ZIP 文件' }).click();
 await expect(page.getByText('资料或访问权限已变化，请重新生成下载包')).toBeVisible();
 expect(starts).toBe(2);
});
test('invalid link requests no session and phone sized pickup does not overflow', async ({ page }) => {
 let requests = 0; await page.route('**/api/**', route => { requests++; return route.abort(); });
 await page.setViewportSize({ width: 390, height: 844 });
 await page.goto('/student-download?job=invalid');
 await expect(page.getByText('领取链接无效，请从小程序复制电脑领取链接。')).toBeVisible();
 expect(requests).toBe(0);
 expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)).toBe(false);
});
