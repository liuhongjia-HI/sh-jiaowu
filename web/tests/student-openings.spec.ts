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
  await page.getByText('当前开通概览', { exact: true }).click();
  await expect(page.getByText('覆盖 2 个年级 · 2 门科目 · 2 种班型')).toBeVisible();
  await page.locator('.ant-collapse').getByRole('button', { name: '1 人 · 查看学生' }).first().click();
  await expect(page.getByText('学生甲', { exact: true })).toBeVisible();
  await expect(page.getByText('学生乙', { exact: true })).toHaveCount(0);
  await expect(page.getByText('学生丙', { exact: true })).toHaveCount(0);
  await expect(page.getByText('覆盖 2 个年级 · 2 门科目 · 2 种班型')).toBeVisible();
  await page.getByRole('button', { name: '查看全部学生', exact: true }).click();
  await expect(page.getByText('学生乙', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '筛选已开通课程的学生' }).click();
  await expect(page.getByText('学生丙', { exact: true })).toHaveCount(0);
  await page.getByText('表格', { exact: true }).click();
  await expect(page.getByRole('columnheader', { name: '当前已开通', exact: true })).toBeVisible();
  await expect(page.getByText('学生甲', { exact: true })).toBeVisible();
});
