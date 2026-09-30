import { test, expect, type Page } from '@playwright/test';
const date = new Date();
const today = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
const day = date.getDay() || 7;
const user = { userId: 'ops', name: '测试教务', roles: ['super_admin'], campusId: 'campus-main' };
const teachers = [{ id: 't1', name: 'Clara', grades: ['五年级'], subjects: ['English'] }, { id: 't2', name: 'James', grades: ['五年级'], subjects: ['English'] }];
const students = [{ id: 's1', name: 'Zoe', grade: '五年级' }, { id: 's2', name: 'Arthur', grade: '五年级' }];
const courses = [{ id: 'c1', name: '英语课程', subject: 'English', grade: '五年级' }];
const baseLesson = { id: 'l1', name: 'Clara G5 English Zoe', courseId: 'c1', courseName: '英语课程', teacherId: 't1', teacherName: 'Clara', lessonDate: today, dayOfWeek: day, startDate: today, endDate: today, startTime: '10:00', endTime: '11:30', durationMinutes: 90, campusId: 'campus-main', roomName: '', classType: '1V1', capacity: 1, students: [students[0]], expectedStudentCount: 1, status: '已确认', auditStatus: '已通过', seriesId: 'series1', detached: false };
let writes: { path: string; body: any }[];
test.beforeEach(async ({ page }) => {
  writes = [];
  let lessons: any[] = [structuredClone(baseLesson)];
  const slots = [...teachers.map(t => ({ id: `av-${t.id}`, ownerType: 'teacher', ownerId: t.id, ownerName: t.name, dayOfWeek: day, startTime: '08:00', endTime: '22:00' })), ...students.map(s => ({ id: `av-${s.id}`, ownerType: 'student', ownerId: s.id, ownerName: s.name, dayOfWeek: day, startTime: '08:00', endTime: '22:00' }))];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'calendar-ui-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', async route => {
    const request = route.request(); const path = new URL(request.url()).pathname.replace('/api', '');
    const body = request.postDataJSON(); let data: any = [];
    if (path === '/auth/me') data = user;
    if (path === '/teachers') data = teachers;
    if (path === '/students') data = students;
    if (path === '/courses') data = courses;
    if (path === '/availability/overview') data = slots;
    if (path === '/schedule-classes') data = lessons;
    if (path === '/schedule-classes/preview') {
      const conflict = lessons.find(l => l.id !== body.id && l.lessonDate === body.startDate && l.teacherId === body.teacherId && l.startTime < body.endTime && l.endTime > body.startTime && l.status !== '已取消');
      data = { canSave: !conflict, lessons: [{ date: body.startDate, errors: conflict ? [`Clara 与「${conflict.name}」冲突（10:00–11:30）`] : [], warnings: [] }] };
    } else if (request.method() === 'PUT' && path.startsWith('/schedule-classes/')) {
      writes.push({ path, body }); lessons = lessons.map(l => l.id === 'l1' ? { ...l, ...body, lessonDate: body.startDate } : l); data = lessons[0];
    } else if (request.method() === 'POST' && path === '/schedule-classes') {
      writes.push({ path, body }); const lesson = { ...baseLesson, ...body, id: 'l2', lessonDate: body.startDate }; lessons = [...lessons, lesson]; data = lesson;
    } else if (path.endsWith('/cancel')) {
      writes.push({ path, body }); lessons = lessons.map(l => ({ ...l, status: '已取消' })); data = lessons[0];
    }
    await route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.goto('/scheduling');
  await expect(page.locator('.calendar-workbench')).toBeVisible();
});
async function choosePerson(page: Page, name: string) {
  await page.getByRole('combobox', { name: '选择人员日历' }).click();
  await page.getByRole('combobox', { name: '选择人员日历' }).fill(name);
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText(name, { exact: false }).last().click();
  await page.keyboard.press('Escape');
  await page.locator('.calendar-toolbar strong').click();
}

test('teacher and student calendars share the same lesson; workweek and preferences', async ({ page }) => {
  await choosePerson(page, 'Clara'); await choosePerson(page, 'Zoe');
  await expect(page.locator('.calendar-column')).toHaveCount(2);
  await expect(page.locator('.calendar-column[data-owner="teacher:t1"] .is-class')).toHaveCount(1);
  await expect(page.locator('.calendar-column[data-owner="student:s1"] .is-class')).toHaveCount(1);
  await page.screenshot({ path: '/tmp/starline-calendar-acceptance.png', fullPage: true });
  await page.getByText('工作周', { exact: true }).click();
  await expect(page.locator('.calendar-column')).toHaveCount(5);
  await page.reload(); await expect(page.locator('.calendar-column')).toHaveCount(5);
});

test('double click carries teacher, date and exact time into form', async ({ page }) => {
  const column = page.locator('.calendar-column[data-owner="teacher:t2"]');
  await column.dblclick({ position: { x: 60, y: 44 * 2 + 10 } });
  const dialog = page.getByRole('dialog', { name: '新建课程' });
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#teacherId')).toHaveAttribute('value', '');
  await expect(dialog.locator('.ant-select-selection-item').filter({ hasText: 'James' })).toBeVisible();
  await expect(dialog.locator('#startTime')).toHaveValue('09:00');
  await expect(dialog.locator('#endTime')).toHaveValue('10:30');
  await expect(dialog.locator('#startDate')).toHaveValue(today);
});

test('drag selection carries student and duration', async ({ page }) => {
  await choosePerson(page, 'Zoe');
  const column = page.locator('.calendar-column[data-owner="student:s1"]');
  await page.locator('.calendar-scroll').evaluate(el => el.scrollTop = 0);
  const bounds = await column.boundingBox(); if (!bounds) throw new Error('missing column');
  await page.mouse.move(bounds.x + 70, bounds.y + 10); await page.mouse.down();
  await page.mouse.move(bounds.x + 70, bounds.y + 65, { steps: 5 }); await page.mouse.up();
  const dialog = page.getByRole('dialog', { name: '新建课程' });
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('#startTime')).toHaveValue('08:00');
  await expect(dialog.locator('#endTime')).toHaveValue('09:00');
  await expect(dialog.locator('#durationMinutes')).toHaveValue('60');
  await expect(dialog.locator('.ant-select-selection-item').filter({ hasText: 'Zoe' })).toBeVisible();
});

test('assistant shows occupied student time and fills a shared free slot', async ({ page }) => {
  await choosePerson(page, 'Clara'); await choosePerson(page, 'Zoe');
  await page.getByRole('button', { name: '新建课程', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '新建课程' });
  await dialog.getByRole('button', { name: '找共同时间' }).click();
  await expect(dialog.getByRole('button', { name: 'Zoe（五年级） 10:00 busy', exact: true })).toBeVisible();
  await dialog.getByRole('button', { name: '08:00', exact: true }).click();
  await expect(dialog.locator('#startTime')).toHaveValue('08:00');
  await expect(dialog.locator('#endTime')).toHaveValue('09:30');
});

test('series editing and cancellation carry explicit scope', async ({ page }) => {
  await page.locator('.calendar-column[data-owner="teacher:t1"] .is-class').click();
  const dialog = page.getByRole('dialog', { name: '课程详情' });
  await dialog.locator('.ant-form-item').filter({ hasText: '本次修改范围' }).locator('.ant-select-selector').click();
  await page.getByText('此课次及后续', { exact: true }).last().click();
  await dialog.getByRole('button', { name: '保存调课' }).click();
  await expect.poll(() => writes.find(w => w.path === '/schedule-classes/l1')?.body.editScope).toBe('thisAndFuture');
  await expect(dialog).toBeHidden();
  await page.locator('.calendar-column[data-owner="teacher:t1"] .is-class').click();
  await page.getByRole('dialog', { name: '课程详情' }).getByRole('button', { name: '取消课程' }).click();
  const cancel = page.getByRole('dialog', { name: '取消课程', exact: true });
  await cancel.locator('.ant-select-selector').click();
  await page.getByText('整个系列（未来课次）', { exact: true }).last().click();
  await cancel.getByRole('button', { name: '取消课程', exact: true }).click();
  await expect.poll(() => writes.find(w => w.path.endsWith('/cancel'))?.body.editScope).toBe('all');
});

test('conflict preview blocks create without writing', async ({ page }) => {
  await choosePerson(page, 'Clara');
  await page.getByRole('button', { name: '新建课程', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '新建课程' });
  await dialog.locator('#courseId').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('英语课程', { exact: false }).last().click();
  await dialog.locator('#startTime').fill('10:00');
  await dialog.locator('#endTime').fill('11:30');
  await dialog.getByRole('button', { name: '创建课程', exact: true }).click();
  await expect(page.getByRole('dialog', { name: '无法排课' })).toContainText('Clara');
  expect(writes).toHaveLength(0);
});

test('coordinated availability override requires explicit confirmation', async ({ page }) => {
  await choosePerson(page, 'Clara');
  await page.route('**/api/schedule-classes/preview', route => route.fulfill({ json: { code: 0, data: { canSave: true, lessons: [{ date: today, errors: [], warnings: ['Clara 未登记可上课时间'] }] } } }));
  await page.getByRole('button', { name: '新建课程', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '新建课程' });
  await dialog.locator('#courseId').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('英语课程', { exact: false }).last().click();
  await dialog.getByRole('button', { name: '创建课程', exact: true }).click();
  const confirm = page.getByRole('dialog', { name: '确认超出可上课时间' });
  await expect(confirm).toContainText('未登记');
  expect(writes).toHaveLength(0);
  await confirm.getByRole('button', { name: '已协调，继续排课' }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].body.ignoreWarnings).toBe(true);
});

test('drag across week dates updates the date with explicit series scope', async ({ page }) => {
  await page.getByText('周', { exact: true }).click();
  const source = page.locator('.calendar-column .is-class').first();
  const target = page.locator('.calendar-column').filter({ hasNot: page.locator('.is-class') }).first();
  const targetDate = await target.getAttribute('data-date');
  await source.dragTo(target, { targetPosition: { x: 40, y: 5 * 44 + 5 } });
  const scope = page.getByRole('dialog', { name: '调整重复课程' });
  await expect(scope).toBeVisible();
  await scope.getByRole('button', { name: '确认调整' }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].body.startDate).toBe(targetDate);
  expect(writes[0].body.editScope).toBe('this');
});

test('wide list scroll stays inside table and keeps calendar controls visible', async ({ page }) => {
  await page.getByText('列表', { exact: true }).click();
  const toolbar = page.locator('.calendar-toolbar');
  const before = await toolbar.boundingBox();
  await page.getByRole('row').filter({ hasText: `${today} 10:00-11:30` }).getByRole('button', { name: '调课' }).click();
  await expect(page.getByRole('dialog', { name: '课程详情' })).toBeVisible();
  const after = await toolbar.boundingBox();
  expect(after?.x).toBe(before?.x);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
});
