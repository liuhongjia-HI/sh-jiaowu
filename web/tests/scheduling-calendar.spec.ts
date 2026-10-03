import { test, expect, type Page } from '@playwright/test';
const date = new Date();
const today = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
const day = date.getDay() || 7;
const user = { userId: 'ops', name: '测试教务', roles: ['super_admin'], campusId: 'campus-main' };
const teachers = [{ id: 't1', name: 'Clara', grades: ['五年级'], subjects: ['English'], learningSpaceIds: ['e1'], accountStatus: '正常' }, { id: 't2', name: 'James', grades: ['五年级'], subjects: ['English'], learningSpaceIds: ['e1'], accountStatus: '正常' }];
const students = [{ id: 's1', name: 'Zoe', grade: '五年级' }, { id: 's2', name: 'Arthur', grade: '五年级' }];
const courses = [{ id: 'c1', name: '英语课程', subject: 'English', grade: '五年级', learningSpaceId: 'e1', status: '启用' }];
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
      const days = [{ date: body.startDate, startTime: body.startTime, endTime: body.endTime }, ...(body.repeat?.freq === 'custom' ? body.repeat.dates.map((item: any) => ({ date: item.date, startTime: item.startTime || body.startTime, endTime: item.endTime || body.endTime })) : [])];
      data = { canSave: !conflict, lessons: days.map((item: any) => ({ ...item, errors: conflict ? [`Clara 与「${conflict.name}」冲突（10:00–11:30）`] : [], warnings: [] })) };
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

test('same-name students survive smaller class switch; capacity blocks save and empty plan remains optional', async ({ page }) => {
  await page.route('**/api/students', route => route.fulfill({ json: { code: 0, data: [
    { id: 'student-000001', name: '小明', grade: '五年级' },
    { id: 'student-000002', name: '小明', grade: '五年级' },
    { id: 'student-000003', name: '小航', grade: '五年级' }
  ] } }));
  await page.reload();
  await choosePerson(page, 'Clara');
  await page.getByRole('button', { name: '新建课程', exact: true }).click();
  const draft = page.getByRole('dialog', { name: '新建课程', exact: true });
  const selectClass = async (value: string) => {
    await draft.locator('.ant-select').filter({ has: page.locator('#classType') }).locator('.ant-select-selector').click();
    await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option-content').filter({ hasText: new RegExp(`^${value}$`) }).click();
  };
  await draft.locator('#courseId').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('英语课程', { exact: false }).click();
  await draft.locator('#startTime').fill('16:00');
  await draft.locator('#endTime').fill('17:00');
  await selectClass('1V2');
  await draft.locator('#studentIds').click();
  await draft.locator('#studentIds').fill('小明');
  const options = page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)');
  await options.getByText('小明 · 五年级 · 000001', { exact: true }).click();
  await draft.locator('#studentIds').click();
  await draft.locator('#studentIds').fill('小明');
  await options.getByText('小明 · 五年级 · 000002', { exact: true }).click();
  await page.keyboard.press('Escape');
  const selected = draft.locator('.ant-select').filter({ has: page.locator('#studentIds') });
  await expect(selected.locator('.ant-select-selection-item')).toHaveCount(2);
  await draft.locator('#studentIds').click();
  await draft.locator('#studentIds').fill('小航');
  await expect(page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option').filter({ hasText: '小航' })).toHaveClass(/ant-select-item-option-disabled/);
  await page.keyboard.press('Escape');
  await draft.locator('#expectedStudentCount').fill('2');
  await selectClass('1V1');
  await draft.getByRole('button', { name: '创建课程', exact: true }).click();
  await expect(draft.getByText('当前班型最多 1 名学生，请调整班型或学生', { exact: true })).toBeVisible();
  await expect(draft.getByText('计划招收人数应为 1 至 1 的整数', { exact: true })).toBeVisible();
  await expect(selected.locator('.ant-select-selection-item')).toHaveCount(2);
  expect(writes).toHaveLength(0);
  await page.screenshot({ path: '/tmp/starline-scheduling-capacity-error.png', fullPage: true });
  await selectClass('1V2');
  await draft.locator('#expectedStudentCount').fill('');
  await draft.getByRole('button', { name: '创建课程', exact: true }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].body).toMatchObject({ classType: '1V2', studentIds: ['student-000001', 'student-000002'], startTime: '16:00', endTime: '17:00' });
  expect(writes[0].body.expectedStudentCount).toBeFalsy();
});

test('inline teacher scope failure and retry preserve schedule draft and refresh courses', async ({ page }) => {
  const spaces = [
    { id: 'e1', name: '五年级英语', grade: '五年级', subject: 'English', status: '启用' },
    { id: 'm1', name: '五年级数学', grade: '五年级', subject: 'Math', status: '启用' }
  ];
  let current = { ...teachers[0], phone: '13800000004', canUploadHandout: true, canUploadQuestion: false, canReview: true, teacherLibrary: { spaceIds: ['e1'], scopes: [], canDownload: false, canViewDrafts: false, canManageCourses: true } };
  let failSave = true;
  const scopeWrites: any[] = [];
  await page.route('**/api/teachers', route => route.fulfill({ json: { code: 0, data: [current, teachers[1]] } }));
  await page.route('**/api/learning-spaces', route => route.fulfill({ json: { code: 0, data: spaces } }));
  await page.route('**/api/subjects', route => route.fulfill({ json: { code: 0, data: ['English', 'Math'].map(name => ({ id: name, name, status: '启用' })) } }));
  await page.route('**/api/courses', route => route.fulfill({ json: { code: 0, data: [...courses, { ...courses[0], id: 'c2', name: '数学课程', subject: 'Math', learningSpaceId: 'm1' }] } }));
  await page.route('**/api/teachers/t1', route => {
    const body = route.request().postDataJSON(); scopeWrites.push(body);
    if (failSave) return route.fulfill({ status: 400, json: { code: 400, message: '范围保存暂时失败' } });
    current = { ...current, ...body };
    return route.fulfill({ json: { code: 0, data: current } });
  });
  await page.reload();
  await choosePerson(page, 'Clara');
  await page.getByRole('button', { name: '新建课程', exact: true }).click();
  const draft = page.getByRole('dialog', { name: '新建课程', exact: true });
  await draft.locator('.ant-select').filter({ has: page.locator('#courseId') }).locator('.ant-select-selector').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('英语课程', { exact: false }).click();
  await draft.locator('#startTime').fill('16:00');
  await draft.locator('#endTime').fill('17:00');
  await draft.locator('#studentIds').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('Zoe', { exact: true }).click();
  await page.keyboard.press('Escape');
  await draft.getByRole('button', { name: '编辑老师授课范围', exact: true }).click();
  const scope = page.getByRole('dialog', { name: '编辑老师授课范围', exact: true });
  await scope.getByRole('button', { name: '添加授课范围', exact: true }).click();
  for (const [label, value] of [['授课年级', '五年级'], ['授课学科', 'Mathematics']]) {
    await scope.locator('.ant-select').filter({ has: page.getByRole('combobox', { name: label, exact: true }) }).last().locator('.ant-select-selector').click();
    await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option-content').filter({ hasText: new RegExp(`^${value}$`) }).last().click();
  }
  await scope.getByRole('button', { name: '保存范围', exact: true }).click();
  await expect(page.getByText('范围保存暂时失败', { exact: true })).toBeVisible();
  await expect(scope).toBeVisible();
  failSave = false;
  await scope.getByRole('button', { name: '保存范围', exact: true }).click();
  await expect(scope).toBeHidden();
  await expect(draft.locator('#startDate')).toHaveValue(today);
  await expect(draft.locator('#startTime')).toHaveValue('16:00');
  await expect(draft.locator('#endTime')).toHaveValue('17:00');
  await expect(draft.locator('.ant-select').filter({ has: page.locator('#studentIds') })).toContainText('Zoe');
  await expect(draft.locator('.ant-select').filter({ has: page.locator('#courseId') })).toContainText('英语课程');
  await draft.locator('.ant-select').filter({ has: page.locator('#courseId') }).locator('.ant-select-selector').click();
  await expect(page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('数学课程', { exact: false })).toBeVisible();
  expect(scopeWrites).toHaveLength(2);
  expect(scopeWrites[1]).toMatchObject({ learningSpaceIds: ['e1', 'm1'], canUploadHandout: true, canUploadQuestion: false, canReview: true, teacherLibrary: { spaceIds: ['e1', 'm1'], canDownload: false, canViewDrafts: false, canManageCourses: true } });
  expect(writes).toHaveLength(0);
  await page.keyboard.press('Escape');
  await draft.getByRole('button', { name: '创建课程', exact: true }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].body).toMatchObject({ teacherId: 't1', courseId: 'c1', studentIds: ['s1'], startDate: today, startTime: '16:00', endTime: '17:00' });
});

test('teacher and student calendars share the same lesson; week and preferences', async ({ page }) => {
  await choosePerson(page, 'Clara'); await choosePerson(page, 'Zoe');
  await expect(page.locator('.calendar-column')).toHaveCount(2);
  await expect(page.locator('.calendar-column[data-owner="teacher:t1"] .is-class')).toHaveCount(1);
  await expect(page.locator('.calendar-column[data-owner="student:s1"] .is-class')).toHaveCount(1);
  await page.screenshot({ path: '/tmp/starline-calendar-acceptance.png', fullPage: true });
  await expect(page.getByText('工作周', { exact: true })).toHaveCount(0);
  await page.getByText('周', { exact: true }).click();
  await expect(page.locator('.calendar-column')).toHaveCount(7);
  await page.reload(); await expect(page.locator('.calendar-column')).toHaveCount(7);
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
  // Native drag needs both columns on screen; scrolling dragTo's target into
  // view can move its source off screen and cancel the browser's drag session.
  await page.setViewportSize({ width: 2400, height: 1000 });
  await page.getByText('周', { exact: true }).click();
  await page.locator('.calendar-scroll').evaluate(el => el.scrollTop = 0);
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

test('custom dates retain per-day times and preview every lesson before create', async ({ page }) => {
  await choosePerson(page, 'Clara');
  await page.getByRole('button', { name: '新建课程', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '新建课程' });
  await dialog.locator('#courseId').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('英语课程', { exact: false }).last().click();
  await dialog.locator('#startTime').fill('08:00');
  await dialog.locator('#endTime').fill('09:00');
  await dialog.getByRole('switch').click();
  await dialog.locator('.ant-select').filter({ has: page.getByRole('combobox', { name: '排课重复方式' }) }).locator('.ant-select-selector').click();
  await page.getByText('自定义日期', { exact: true }).last().click();
  await dialog.getByRole('button', { name: '添加上课日期' }).click();
  const next = new Date(`${today}T12:00:00`); next.setDate(next.getDate() + 3);
  const nextDate = `${next.getFullYear()}-${String(next.getMonth()+1).padStart(2,'0')}-${String(next.getDate()).padStart(2,'0')}`;
  await dialog.locator('[aria-label="其他上课日期 1"]').fill(nextDate);
  await dialog.locator('[aria-label="其他开始时间 1"]').fill('14:00');
  await dialog.locator('[aria-label="其他结束时间 1"]').fill('15:30');
  await expect(dialog.locator('.preview-result')).toContainText('2 节课');
  await dialog.getByText('查看全部课次', { exact: true }).click();
  await expect(dialog.locator('.preview-result')).toContainText(`${nextDate} · 14:00–15:30`);
  await page.screenshot({ path: '/tmp/starline-custom-dates.png', fullPage: true });
  await dialog.getByRole('button', { name: '创建课程', exact: true }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].body.repeat).toEqual({ freq: 'custom', interval: 1, dates: [{ date: nextDate, startTime: '14:00', endTime: '15:30' }] });
});

test('compact list opens lesson details and keeps calendar controls visible', async ({ page }) => {
  await page.getByText('列表', { exact: true }).click();
  const toolbar = page.locator('.calendar-toolbar');
  const before = await toolbar.boundingBox();
  await page.locator('.schedule-lesson-list-row[data-lesson-id="l1"] .month-class').click();
  await expect(page.getByRole('dialog', { name: '课程详情' })).toBeVisible();
  const after = await toolbar.boundingBox();
  expect(after?.x).toBe(before?.x);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
});


test('calendar names fit within headers and legacy workweek becomes week', async ({ page }) => {
  await page.locator('.calendar-scroll').evaluate(el => el.scrollTop = 0);
  const headers = page.locator('.calendar-scroll .schedule-lane-head');
  const result = await headers.evaluateAll(elements => {
    elements.forEach(el => { el.querySelector('strong')!.textContent = '王小明老师（国际课程教学负责人）'; });
    return elements.map(el => {
      const text = document.createRange(); text.selectNodeContents(el.querySelector('strong')!);
      const bounds = el.getBoundingClientRect();
      return { contained: Array.from(text.getClientRects()).every(rect => rect.top >= bounds.top && rect.bottom <= bounds.bottom && rect.left >= bounds.left && rect.right <= bounds.right), bottom: bounds.bottom };
    });
  });
  expect(result.every(item => item.contained)).toBeTruthy();
  const columnTop = await page.locator('.calendar-column').first().evaluate(el => el.getBoundingClientRect().top);
  expect(Math.abs(result[0].bottom - columnTop)).toBeLessThan(1);
  await page.screenshot({ path: '/tmp/starline-calendar-name-fix.png', fullPage: true });
  await page.evaluate(() => localStorage.setItem('starline-calendar-view:ops', 'workweek'));
  await page.reload();
  await expect(page.locator('.calendar-column')).toHaveCount(7);
  await expect(page.getByText('工作周', { exact: true })).toHaveCount(0);
});

test('dense month day opens every lesson with readable long names and closes before editing', async ({ page }) => {
  const longTeacher = 'Clara 教师姓名很长需要完整显示';
  const many = Array.from({ length: 12 }, (_, i) => ({ ...baseLesson, id: `dense-${i}`, teacherName: longTeacher, startTime: `${String(8+i).padStart(2,'0')}:00`, endTime: `${String(9+i).padStart(2,'0')}:00`, durationMinutes: 60, students: [{ ...students[0], name: 'Zoe 学生姓名很长需要完整显示' }] }));
  await page.route('**/api/schedule-classes', route => route.fulfill({ json: { code: 0, data: many } }));
  await page.route('**/api/teachers', route => route.fulfill({ json: { code: 0, data: teachers.map(t => t.id === 't1' ? { ...t, name: longTeacher } : t) } }));
  await page.reload();
  await page.getByText('月', { exact: true }).click();
  const dayCell = page.locator(`.month-day[data-date="${today}"]`);
  await expect(dayCell.locator('.month-class')).toHaveCount(3);
  await expect(dayCell.getByRole('button', { name: '另 9 节 · 查看全部', exact: true })).toBeVisible();
  await dayCell.getByRole('button', { name: `${today} 查看全部 12 节课`, exact: true }).click();
  const drawer = page.getByRole('dialog', { name: `${today} · 全部 12 节课`, exact: true });
  await expect(drawer.locator('.month-class')).toHaveCount(12);
  const names = await drawer.locator('.month-class-text').evaluateAll(elements => elements.every(el => getComputedStyle(el).whiteSpace === 'normal' && el.scrollWidth <= el.clientWidth));
  expect(names).toBe(true);
  const last = drawer.locator('[data-lesson-id="dense-11"]');
  await last.scrollIntoViewIfNeeded(); await expect(last).toContainText('19:00-20:00');
  await expect(last).toContainText(longTeacher); await expect(last).toContainText('Zoe 学生姓名很长需要完整显示');
  await page.screenshot({ path: '/tmp/starline-month-complete-day.png', animations: 'disabled' });
  await last.click();
  await expect(drawer).toBeHidden();
  const details = page.getByRole('dialog', { name: '课程详情', exact: true });
  await expect(details).toBeVisible();
  await expect(details.locator('#startTime')).toHaveValue('19:00');
  expect(writes).toHaveLength(0);
});

test('narrow list paginates all lessons in time order and handles a shrinking result', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const many = Array.from({ length: 45 }, (_, i) => {
    const minutes = 8 * 60 + i * 15;
    const time = (value: number) => `${String(Math.floor(value / 60)).padStart(2, '0')}:${String(value % 60).padStart(2, '0')}`;
    return { ...baseLesson, id: `page-${i}`, startTime: time(minutes), endTime: time(minutes + 15), durationMinutes: 15, teacherName: 'Clara 教师完整姓名', students: [{ ...students[0], name: 'Zoe 学生完整姓名需要换行' }] };
  });
  let records = [...many].reverse();
  await page.route('**/api/schedule-classes', route => route.fulfill({ json: { code: 0, data: records } }));
  await page.reload();
  await page.getByText('列表', { exact: true }).click();
  const rows = page.locator('.schedule-lesson-list-row');
  await expect(rows).toHaveCount(20);
  expect(await rows.first().getAttribute('data-lesson-id')).toBe('page-0');
  expect(await rows.last().getAttribute('data-lesson-id')).toBe('page-19');
  await page.locator('.schedule-lesson-list .ant-pagination-item-2').click();
  await expect(rows.first()).toHaveAttribute('data-lesson-id', 'page-20');
  await expect(rows.last()).toHaveAttribute('data-lesson-id', 'page-39');
  await page.locator('.schedule-lesson-list .ant-pagination-item-3').click();
  await expect(rows).toHaveCount(5);
  await expect(rows.last()).toHaveAttribute('data-lesson-id', 'page-44');
  expect(await rows.evaluateAll(elements => elements.every(el => el.scrollWidth <= el.clientWidth))).toBe(true);
  await rows.last().scrollIntoViewIfNeeded();
  await page.screenshot({ path: '/tmp/starline-schedule-list-narrow.png', fullPage: false });
  records = many.slice(0, 2);
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(rows).toHaveCount(2);
  await expect(page.locator('.schedule-lesson-list .ant-pagination-item-active')).toHaveText('1');
  await expect(rows.first()).toHaveAttribute('data-lesson-id', 'page-0');
  records = [...many].reverse();
  await page.getByRole('button', { name: '刷新', exact: true }).click();
  await expect(rows).toHaveCount(20);
  await expect(page.locator('.schedule-lesson-list .ant-pagination-item-active')).toHaveText('1');
  await expect(rows.first()).toHaveAttribute('data-lesson-id', 'page-0');
});

test('narrow dense month exposes every lesson in the complete day drawer', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const many = Array.from({ length: 30 }, (_, i) => ({ ...baseLesson, id: `narrow-day-${i}`, students: [{ ...students[0], name: `第 ${i + 1} 节学生姓名很长仍需要完整显示` }] }));
  await page.route('**/api/schedule-classes', route => route.fulfill({ json: { code: 0, data: many } }));
  await page.reload();
  await page.getByText('月', { exact: true }).click();
  await page.locator(`.month-day[data-date="${today}"]`).getByRole('button', { name: `${today} 查看全部 30 节课`, exact: true }).click();
  const drawer = page.getByRole('dialog', { name: `${today} · 全部 30 节课`, exact: true });
  await expect(drawer.locator('.month-class')).toHaveCount(30);
  await expect.poll(() => drawer.evaluate(el => getComputedStyle(el).transform)).toBe('none');
  await expect.poll(() => drawer.evaluate(el => { const r = el.getBoundingClientRect(); return r.x >= 0 && r.right <= window.innerWidth; })).toBe(true);
  const bounds = await drawer.boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(375);
  expect(await drawer.locator('.month-class-text').evaluateAll(elements => elements.every(el => el.scrollWidth <= el.clientWidth))).toBe(true);
  const last = drawer.locator('[data-lesson-id="narrow-day-29"]');
  await last.scrollIntoViewIfNeeded();
  await expect(last).toContainText('第 30 节学生姓名很长仍需要完整显示');
  await page.screenshot({ path: '/tmp/starline-schedule-month-narrow.png', fullPage: false });
  await last.click();
  await expect(drawer).toBeHidden();
  await expect(page.getByRole('dialog', { name: '课程详情', exact: true })).toBeVisible();
  expect(writes).toHaveLength(0);
});


test('compact list cancellation uses existing series scope and leaves a visible canceled record', async ({ page }) => {
 await page.getByText('列表', { exact: true }).click();
 await page.getByRole('button', { name: '取消课次 l1', exact: true }).click();
 const scope = page.getByRole('dialog', { name: '取消课程', exact: true });
 await expect(scope).toBeVisible();
 await expect(scope).toContainText('仅此课次');
 await scope.getByRole('button', { name: '取消课程', exact: true }).click();
 await expect.poll(() => writes.length).toBe(1);
 expect(writes[0].body.editScope).toBe('this');
 await expect(page.locator('.schedule-lesson-list-row[data-lesson-id="l1"]')).toContainText('已取消');
 await expect(page.getByRole('button', { name: '取消课次 l1', exact: true })).toHaveCount(0);
});

test('restoring canceled card previews first and rechecks the save response', async ({ page }) => {
 let cancelled=true;let saveFails=true;const calls:any[]=[];
 await page.route('**/api/schedule-classes',route=>route.fulfill({json:{code:0,data:[{...baseLesson,status:cancelled?'已取消':'已确认'}]}}));
 await page.route('**/api/schedule-classes/l1/restore-preview',route=>{calls.push({path:'preview'});return route.fulfill({json:{code:0,data:{canSave:true,lessons:[{date:today,startTime:'10:00',endTime:'11:30',errors:[],warnings:[]}]}}});});
 await page.route('**/api/schedule-classes/l1/restore',route=>{
  calls.push({path:'restore',body:route.request().postDataJSON()});
  if(saveFails)return route.fulfill({status:400,json:{code:400,message:'该时间已被其他课程占用'}});
  cancelled=false;return route.fulfill({json:{code:0,data:{...baseLesson}}});
 });
 await page.reload();await page.getByText('列表',{exact:true}).click();
 await page.getByRole('button',{name:'恢复课次 l1',exact:true}).click();
 const confirm=page.getByRole('dialog',{name:'恢复这节课程',exact:true});
 await expect(confirm).toBeVisible();expect(calls.map(c=>c.path)).toEqual(['preview']);
 await confirm.getByRole('button',{name:'恢复课程',exact:true}).click();
 await expect(page.getByText('该时间已被其他课程占用',{exact:true})).toBeVisible();
 await expect(confirm).toBeVisible();await expect(page.locator('.schedule-lesson-list-row')).toContainText('已取消');
 saveFails=false;
 await confirm.getByRole('button',{name:'恢复课程',exact:true}).click();
 await expect(confirm).toBeHidden();
 await expect(page.getByRole('button',{name:'恢复课次 l1',exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'取消课次 l1',exact:true})).toBeVisible();
 expect(calls.filter(c=>c.path==='restore').map(c=>c.body)).toEqual([{ignoreWarnings:false},{ignoreWarnings:false}]);
});


test('completed lesson updates only scheduling status and hides repeat action', async ({ page }) => {
  let lesson = { ...baseLesson, startTime: '00:00', endTime: '00:01' };
  await page.route('**/api/schedule-classes', route => route.fulfill({ json: { code: 0, data: [lesson] } }));
  await page.route('**/api/schedule-classes/l1/completed', async route => {
    writes.push({ path: '/schedule-classes/l1/completed', body: route.request().postDataJSON() });
    lesson = { ...lesson, status: '已上课' };
    await route.fulfill({ json: { code: 0, data: lesson } });
  });
  await page.reload();
  await page.getByText('列表', { exact: true }).click();
  await page.locator('.schedule-lesson-list-row[data-lesson-id="l1"] .month-class').click();
  const detail = page.getByRole('dialog', { name: '课程详情' });
  await detail.getByRole('button', { name: '标记已上课', exact: true }).click();
  await expect(page.getByText('已标记已上课', { exact: true })).toBeVisible();
  await expect(detail.getByRole('button', { name: '标记已上课', exact: true })).toHaveCount(0);
  await expect(detail.getByRole('button', { name: '保存调课', exact: true })).toBeDisabled();
  expect(writes).toEqual([{ path: '/schedule-classes/l1/completed', body: {} }]);
});
