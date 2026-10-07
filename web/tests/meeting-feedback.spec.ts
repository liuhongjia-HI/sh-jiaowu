import { test, expect } from '@playwright/test';

test('failed teaching plan explains conversion reason and retry clears stale error', async ({ page }) => {
  const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
  let retried = false;
  const plan = { id: 'plan-failed', title: '旧版备课', grade: '五年级', subject: 'English', fileName: '旧版.doc', fileSize: 1000, fileType: 'Word', previewStatus: '转换失败', previewError: '未生成预览，请检查 Word/PPT 是否损坏或已加密', previewUrl: '/api/teaching-plans/plan-failed/preview', downloadUrl: '/api/teaching-plans/plan-failed/download' };
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'failed-plan-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname.replace('/api', '');
    if (path.endsWith('/preview/retry')) { retried = true; return route.fulfill({ json: { code: 0, data: { queued: true } } }); }
    const data = path === '/auth/me' ? user : path === '/teaching-plans' ? { plans: [{ ...plan, ...(retried ? { previewStatus: '待转换', previewError: '' } : {}) }], uploadScopes: [{ grade: '五年级', subject: 'English' }], directories: [], canUpload: true, unreadPlanIds: ['plan-failed'] } : [];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/teaching-plans');
  await page.getByRole('button', { name: '旧版备课', exact: true }).click();
  const preview = page.getByRole('dialog', { name: /^旧版备课/ });
  await expect(preview).toContainText(plan.previewError);
  await expect(preview.getByRole('button', { name: '下载原文件', exact: false })).toBeVisible();
  await page.screenshot({ path: '/tmp/starline-plan-conversion-reason.png', fullPage: false, animations: 'disabled' });
  await preview.getByRole('button', { name: '重新生成预览', exact: true }).click();
  await expect(preview).toContainText('正在生成教案预览');
  await expect(preview).not.toContainText(plan.previewError);
  expect(retried).toBe(true);
});

test('resource notice template mapping saves a closed draft with an available trigger', async ({ page }) => {
  const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
  const fields = { course_name: '课程名称', resource_title: '资料名称', resource_count: '资料数量', published_at: '发布时间', student_name: '学生姓名' };
  const binding = { kind: 'materials_published', title: '资料批量发布', templateId: '', enabled: false, ready: false, triggerReady: true, requiredFields: {}, availableFields: fields, reason: '' };
  const saved: any[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'resource-notice-mapping-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    if (req.method() === 'PUT') {
      saved.push(req.postDataJSON());
      return route.fulfill({ json: { code: 0, data: [{ ...binding, ...saved[0] }] } });
    }
    const data = path === '/auth/me' ? user : path === '/official-account/automatic-notices' ? [binding] : path === '/official-account/templates' ? [{ id: 't1', title: '资料更新提醒', status: '启用', fields: [{ key: 'thing4', label: '资料' }, { key: 'number3', label: '数量' }, { key: 'time7', label: '时间' }] }] : [];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/notices');
  await page.getByRole('row').filter({ hasText: '资料批量发布' }).getByRole('button', { name: /配\s*置/ }).click();
  const modal = page.getByRole('dialog', { name: '资料批量发布配置', exact: true });
  await modal.locator('.ant-select-selector').first().click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('资料更新提醒', { exact: true }).click();
  for (const [key, label] of [['thing4', '资料名称'], ['number3', '资料数量'], ['time7', '发布时间']]) {
    const selector = modal.locator('.ant-select').filter({ has: page.getByRole('combobox', { name: `映射 ${key}`, exact: true }) }).locator('.ant-select-selector');
    await selector.click();
    const listId = await modal.getByRole('combobox', { name: `映射 ${key}`, exact: true }).getAttribute('aria-controls');
    await page.locator('.ant-select-dropdown').filter({ has: page.locator(`[id=${JSON.stringify(listId)}]`) }).getByText(label, { exact: true }).click();
  }
  await expect(modal.getByRole('switch')).toBeEnabled();
  await expect(modal.getByRole('switch')).not.toBeChecked();
  await modal.getByText('自动推送', { exact: true }).click();
  const saveBox = await modal.getByRole('button', { name: /确\s*定/ }).boundingBox();
  expect(saveBox!.y + saveBox!.height).toBeLessThanOrEqual(page.viewportSize()!.height);
  await page.screenshot({ path: '/tmp/starline-resource-notice-mapping.png', fullPage: false });
  await modal.getByRole('button', { name: /确\s*定/ }).click();
  await expect.poll(() => saved.length).toBe(1);
  expect(saved[0]).toMatchObject({ templateId: 't1', enabled: false, fieldMappings: { thing4: 'resource_title', number3: 'resource_count', time7: 'published_at' } });
  await expect(modal).not.toBeVisible();
});

test('material upload uses one notification batch across partial failure and retry', async ({ page }) => {
  const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
  const course = { id: 'batch-course', name: '批量上传课程', learningSpaceId: 'e1', grade: '五年级', subject: 'English', status: '启用', curriculum: [{ id: 'batch-lesson', type: 'unit', name: '第一课', sortOrder: 1 }] };
  const batches: string[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'material-batch-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    if (path === '/materials' && req.method() === 'POST') {
      const body = req.postData() || '';
      batches.push(body.match(/name="batchId"\r\n\r\n([^\r]+)/)?.[1] || '');
      if (batches.length === 2) return route.fulfill({ status: 400, json: { code: 400, message: '第二份上传失败' } });
      return route.fulfill({ json: { code: 0, data: { id: `material-${batches.length}`, fileName: 'HD.pdf', tagCode: 'HD', publishStatus: '已发布', status: '启用', courseId: course.id, lessonId: 'batch-lesson' } } });
    }
    const data = path === '/auth/me' ? user : path === '/courses' ? [course] : path === '/learning-spaces' ? [{ id: 'e1', grade: '五年级', subject: 'English', semester: 's1', phase: 'q1', level: 'S', status: '启用' }] : path === '/subjects' ? [{ id: 'english', name: 'English', status: '启用' }] : [];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/content?tab=materials&courseId=batch-course');
  await expect(page.getByRole('heading', { name: course.name, exact: true })).toBeVisible();
  await expect(page.getByRole('tab', { name: '课程讲义', exact: true })).toBeVisible();
  const openUpload = async (files: string[]) => {
    await page.getByRole('button', { name: /上传讲义/ }).click();
    const drawer = page.getByRole('dialog', { name: '给课节上传资料', exact: true });
    await drawer.locator('#lessonId').click();
    await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('Unit 1 · 第一课', { exact: true }).click();
    await drawer.locator('input[type="file"]').setInputFiles(files.map(name => ({ name, mimeType: 'application/pdf', buffer: Buffer.from('%PDF-fixture') })));
    await drawer.getByRole('button', { name: /^上\s*传$/ }).click();
  };
  await openUpload(['HD-first.pdf', 'HD-second.pdf']);
  const result = page.getByRole('dialog', { name: '同步本次课程讲义', exact: true });
  await expect(result).toContainText('第二份上传失败');
  await result.getByRole('button', { name: '只重新上传失败文件', exact: true }).click();
  await expect.poll(() => batches.length).toBe(3);
  expect(batches[0]).toMatch(/^[0-9a-f-]{36}$/);
  expect(batches.slice(0, 3)).toEqual([batches[0], batches[0], batches[0]]);
  await expect(result.getByRole('button', { name: '只重新上传失败文件', exact: true })).toHaveCount(0);
  await result.getByRole('button', { name: '暂不同步', exact: true }).click();
  await openUpload(['HD-next-operation.pdf']);
  await expect.poll(() => batches.length).toBe(4);
  expect(batches[3]).not.toBe(batches[0]);
});

test('teaching plans ignore legacy unread filters and preview without reading tracking', async ({ page }) => {
  const user = { userId: 'teacher', name: '只读教师', roles: ['teacher'], teacherLibrary: { canManageCourses: false }, canUploadHandout: false, canUploadQuestion: false, canReview: false };
  const version = 'version-1';
  const unread = ['p1', 'p2'];
  const reads: any[] = [];
  const plans = () => [1, 2, 3].map(i => ({ id: `p${i}`, title: `内部教案 ${i}`, readVersion: i === 1 ? version : `version-${i}`, grade: '五年级', subject: 'English', fileName: `plan${i}.pdf`, fileSize: 100, fileType: 'pdf', previewStatus: '可预览', previewUrl: `/api/teaching-plans/p${i}/preview`, uploaderName: '教师', createdAt: `2026-10-03 12:0${i}:00` }));
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'unread-plan-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    if (path.endsWith('/preview')) {
      if (path.includes('/p2/')) return route.fulfill({ status: 400, json: { code: 400, message: '教案预览暂时失败' } });
      return route.fulfill({ contentType: 'application/pdf', body: Buffer.from('%PDF-browser-flow; actual PDF bytes validated by HTTP tests') });
    }
    if (path.endsWith('/view')) {
      reads.push(req.postDataJSON());
      return route.fulfill({ json: { code: 0, data: { read: true } } });
    }
    return route.fulfill({ json: { code: 0, data: path === '/auth/me' ? user : path === '/teaching-plans' ? { plans: plans(), unreadPlanIds: unread, uploadScopes: [], canUpload: false } : [] } });
  });
  await page.goto('/teaching-plans?unread=1');
  await expect(page.getByRole('button', { name: '内部教案 3', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '新增与未读', exact: true })).toHaveCount(0);
  await expect(page.getByText('未读', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: '内部教案 2', exact: true }).click();
  const failed = page.getByRole('dialog', { name: /^内部教案 2/ });
  await expect(failed).toContainText('教案预览暂时失败');
  expect(reads).toHaveLength(0);
  await failed.getByRole('button', { name: '返回列表', exact: true }).click();
  await page.getByRole('button', { name: '内部教案 1', exact: true }).click();
  const preview = page.getByRole('dialog', { name: /^内部教案 1/ });
  await expect(preview.locator('iframe')).toBeVisible();
  await preview.getByRole('button', { name: '返回列表', exact: true }).click();
  await expect(page.getByRole('button', { name: '内部教案 1', exact: true })).toBeVisible();
  expect(reads).toHaveLength(0);
});

test('dropped teaching plans keep per-file chapters when only failed file is retried', async ({ page }) => {
  const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
  const directory = { id: 'drag-course', name: '五年级英语目录', grade: '五年级', subject: 'English', curriculum: [1, 2, 3].map(i => ({ id: `u${i}`, type: 'unit', name: `第${i}课`, sortOrder: i })) };
  const uploads: string[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'drag-plan-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', route => {
    const request = route.request(); const path = new URL(request.url()).pathname.replace('/api', '');
    if (path === '/teaching-plans' && request.method() === 'POST') {
      uploads.push(request.postData() || '');
      if (uploads.length === 2) return route.fulfill({ status: 400, json: { code: 400, message: '第二份教案暂时失败' } });
      return route.fulfill({ json: { code: 0, data: { id: `plan-${uploads.length}` } } });
    }
    const data = path === '/auth/me' ? user : path === '/teaching-plans' ? { plans: [], canUpload: true, uploadScopes: [{ grade: '五年级', subject: 'English' }], directories: [directory] } : [];
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/teaching-plans');
  await page.getByRole('button', { name: /上传教案/ }).click();
  const drawer = page.getByRole('dialog', { name: '上传教案', exact: true });
  const transfer = await page.evaluateHandle(() => {
    const data = new DataTransfer();
    data.items.add(new File(['%PDF-test'], '第一份.pdf', { type: 'application/pdf', lastModified: 100 }));
    data.items.add(new File(['%PDF-test'], '第一份.pdf', { type: 'application/pdf', lastModified: 100 }));
    data.items.add(new File(['docx-test'], '第二份.docx', { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', lastModified: 100 }));
    return data;
  });
  const drop = drawer.getByLabel('选择教案文件', { exact: true }).locator('..');
  await drop.dispatchEvent('drop', { dataTransfer: transfer });
  await drop.dispatchEvent('drop', { dataTransfer: transfer });
  await expect(drawer.getByRole('textbox', { name: '第 1 份教案标题', exact: true })).toHaveValue('第一份');
  await expect(drawer.getByRole('textbox', { name: '第 2 份教案标题', exact: true })).toHaveValue('第二份');
  await expect(drawer.getByRole('textbox', { name: '第 3 份教案标题', exact: true })).toHaveCount(0);
  const chooseChapter = async (label: string, index: number) => {
    const selector = drawer.locator('.ant-select').filter({ has: page.getByRole('combobox', { name: label, exact: true }) }).locator('.ant-select-selector');
    await selector.scrollIntoViewIfNeeded();
    await selector.click();
    const listId = await drawer.getByRole('combobox', { name: label, exact: true }).getAttribute('aria-controls');
    await page.locator('.ant-select-dropdown').filter({ has: page.locator(`[id=${JSON.stringify(listId)}]`) }).locator('.ant-select-item-option').nth(index).click();
  };
  await chooseChapter('教案章节', 0);
  await chooseChapter('第 2 份教案章节', 2);
  await drawer.getByRole('textbox', { name: '第 2 份教案标题', exact: true }).fill('第二课备课');
  await drawer.getByRole('button', { name: '上传 2 份', exact: true }).click();
  await expect(drawer).toContainText('第二份教案暂时失败');
  await expect(drawer.getByRole('textbox', { name: '第 1 份教案标题', exact: true })).toBeDisabled();
  expect(uploads[0]).toContain('name="lessonId"\r\n\r\nu1');
  expect(uploads[1]).toContain('name="lessonId"\r\n\r\nu2');
  await chooseChapter('教案章节', 2);
  await drawer.getByRole('button', { name: '重试此文件', exact: true }).click();
  await expect(drawer).toBeHidden();
  expect(uploads).toHaveLength(3);
  expect(uploads[2]).toContain('filename="第二份.docx"');
  expect(uploads[2]).toContain('name="lessonId"\r\n\r\nu2');
  expect(uploads[2]).toContain('name="title"\r\n\r\n第二课备课');
  expect(uploads[2]).not.toContain('filename="第一份.pdf"');
});

test('scope switch shows affected resources and blocks incompatible plans or failed checks', async ({ page }) => {
  const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
  const course = { id: 'scope-course', name: '范围变更课程', grade: '五年级', subject: 'English', learningSpaceId: 'e1', status: '启用', curriculum: [{ id: 'u1', type: 'unit', name: '第一课', sortOrder: 1 }] };
  const spaces = ['e1', 'e2', 'e3'].map((id, index) => ({ id, name: id, grade: '五年级', subject: 'English', semester: 'S1', phase: index === 1 ? 'Q2' : 'Q1', level: index === 2 ? 'S+' : 'S', status: '启用' }));
  let fail = true;
  const writes: any[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'scope-impact-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', route => {
    const request = route.request(); const path = new URL(request.url()).pathname.replace('/api', '');
    let data: any = [];
    if (path === '/auth/me') data = user;
    if (path === '/courses') data = [course];
    if (path === '/subjects') data = [{ id: 'english', name: 'English', status: '启用' }];
    if (path === '/learning-spaces') data = spaces;
    if (path.endsWith('/curriculum-references')) {
      if (fail) return route.fulfill({ status: 400, json: { code: 400, message: '范围检查失败' } });
      const target = request.postDataJSON().targetLearningSpaceId;
      data = [{ id: 'plan1', kind: '教案', title: '教师备课文件', blocking: target === 'e2' }, { id: 'm1', kind: '讲义', title: '学生讲义', blocking: false }];
    }
    if (request.method() === 'PUT') { writes.push(request.postDataJSON()); data = course; }
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/content');
  await page.getByRole('row').filter({ hasText: course.name }).getByRole('button', { name: '编辑', exact: true }).click();
  const editor = page.getByRole('dialog', { name: '编辑课程', exact: true });
  await editor.getByLabel('课程名称', { exact: true }).fill('保留名称草稿');
  const chooseScope = async (index: number) => {
    await editor.locator('.ant-select').filter({ has: page.locator('#learningSpaceId') }).locator('.ant-select-selector').click();
    await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option').nth(index).click();
  };
  await chooseScope(1);
  await expect(editor.getByText('引用检查失败，暂不能保存范围变更', { exact: true })).toBeVisible();
  await expect(editor.getByRole('button', { name: /保\s*存/ })).toBeDisabled();
  fail = false;
  await editor.getByRole('button', { name: '重新检查', exact: true }).click();
  await expect(editor).toContainText('教案与目标年级、学科、学期或阶段不匹配');
  await expect(editor).toContainText('学生讲义');
  await expect(editor.getByRole('button', { name: /保\s*存/ })).toBeDisabled();
  await expect(editor.getByLabel('课程名称', { exact: true })).toHaveValue('保留名称草稿');
  await editor.getByText('教学范围变更影响', { exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: '/tmp/starline-curriculum-scope-impact.png', fullPage: false });
  expect(writes).toHaveLength(0);
  await chooseScope(2);
  await expect(editor.getByText('以下资料将随课程进入新的教学范围', { exact: true })).toBeVisible();
  await expect(editor.getByRole('button', { name: /保\s*存/ })).toBeEnabled();
  await editor.getByRole('button', { name: /保\s*存/ }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0]).toMatchObject({ name: '保留名称草稿', learningSpaceId: 'e3', curriculum: course.curriculum });
});

test('chapter deletion previews bindings, preserves draft on failure and requires reassignment', async ({ page }) => {
  const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
  const course = { id: 'ref-course', name: '引用检查课程', grade: '五年级', subject: 'English', learningSpaceId: 'e1', status: '启用', curriculum: [{ id: 'u1', type: 'unit', name: '第一课', sortOrder: 1 }, { id: 'u2', type: 'unit', name: '第二课', sortOrder: 2 }] };
  let fail = true;
  let linked = true;
  const writes: any[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'chapter-reference-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    let data: any = [];
    if (path === '/auth/me') data = user;
    if (path === '/courses') data = [course];
    if (path === '/subjects') data = [{ id: 'english', name: 'English', status: '启用' }];
    if (path === '/learning-spaces') data = [{ id: 'e1', name: '五年级英语', grade: '五年级', subject: 'English', semester: 'S1', phase: 'Q1', status: '启用' }];
    if (path.endsWith('/curriculum-references')) {
      expect(req.postDataJSON()).toEqual({ nodeIds: ['u1'] });
      if (fail) return route.fulfill({ status: 400, json: { code: 400, message: '引用检查暂时失败' } });
      data = linked ? [{ id: 'plan1', kind: '教案', title: '教师备课文件', courseName: '引用检查课程' }] : [];
    }
    if (path === '/courses/ref-course' && req.method() === 'PUT') { writes.push(req.postDataJSON()); data = course; }
    return route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/content');
  await page.getByRole('row').filter({ hasText: course.name }).getByRole('button', { name: '编辑', exact: true }).click();
  const editor = page.getByRole('dialog', { name: '编辑课程', exact: true });
  const first = editor.getByTestId('curriculum-unit').first();
  await first.getByRole('button', { name: '删除', exact: true }).click();
  await expect(page.getByText('引用检查暂时失败', { exact: true })).toBeVisible();
  await expect(editor.getByTestId('curriculum-unit')).toHaveCount(2);
  fail = false;
  await first.getByRole('button', { name: '删除', exact: true }).click();
  const impact = page.getByRole('dialog', { name: '目录引用影响', exact: true });
  await expect(impact).toContainText('教师备课文件');
  await expect.poll(() => impact.evaluate(el => getComputedStyle(el).transform)).toBe('none');
  await expect.poll(() => impact.evaluate(el => { const r = el.getBoundingClientRect(); return el.contains(document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2)); })).toBe(true);
  await impact.evaluate(async el => {
    const root = el.closest('.ant-modal-root') || el;
    await Promise.all(root.getAnimations({ subtree: true }).filter(animation => animation.effect?.getComputedTiming().iterations !== Infinity).map(animation => animation.finished.catch(() => {})));
  });
  await page.screenshot({ path: '/tmp/starline-curriculum-reference-impact.png', fullPage: false });
  await expect(impact.getByRole('link')).toHaveAttribute('href', '/teaching-plans');
  await expect(editor.getByTestId('curriculum-unit')).toHaveCount(2);
  await impact.getByRole('button', { name: /知道了|确\s*定|OK/ }).click();
  linked = false;
  await first.getByRole('button', { name: '删除', exact: true }).click();
  await page.getByRole('dialog', { name: '删除目录', exact: true }).getByRole('button', { name: /保\s*留/ }).click();
  await expect(editor.getByTestId('curriculum-unit')).toHaveCount(2);
  await first.getByRole('button', { name: '删除', exact: true }).click();
  await page.getByRole('dialog', { name: '删除目录', exact: true }).getByRole('button', { name: /删\s*除/ }).click();
  await expect(editor.getByTestId('curriculum-unit')).toHaveCount(1);
  expect(writes).toHaveLength(0);
  await editor.getByRole('button', { name: /保\s*存/ }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].curriculum.map((node: any) => node.id)).toEqual(['u2']);
});

test('course return restores grade, subject, search, pagination and actual content scroll', async ({ page }) => {
  const user = { userId: 'ops', name: '测试教务', roles: ['super_admin'], campusId: 'campus-main' };
  const courses = Array.from({ length: 25 }, (_, i) => ({ id: `c${i}`, name: `英语备课 ${String(i + 1).padStart(2, '0')}`, grade: '五年级', subject: 'English', status: '启用', learningSpaceId: 'e1', curriculum: [], lessonCount: 0 }));
  await page.setViewportSize({ width: 1440, height: 650 });
  await page.addInitScript(user => {
    localStorage.setItem('starline_admin_token', 'meeting-ui-fixture');
    localStorage.setItem('starline_admin_user', JSON.stringify(user));
  }, user);
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname.replace('/api', '');
    const data = path === '/auth/me' ? user : path === '/subjects' ? [{ id: 'english', name: 'English', status: '启用' }] : path === '/courses' ? courses : path === '/learning-spaces' ? [{ id: 'e1', grade: '五年级', subject: 'English', semester: 's1', phase: 'q1', level: '1V1', status: '启用' }] : [];
    return route.fulfill({ json: { code: 0, message: 'ok', data } });
  });
  await page.goto('/content');
  await page.getByPlaceholder('搜索课程').fill('英语备课');
  for (const [label, value] of [['年级', '五年级'], ['学科', 'English']]) {
    await page.getByRole('group', { name: label, exact: true }).getByRole('button', { name: value, exact: true }).click();
  }
  await page.locator('.ant-pagination-item-2').click();
  const course = page.getByText('英语备课 18', { exact: true });
  await course.scrollIntoViewIfNeeded();
  const saved = await page.evaluate(() => document.scrollingElement!.scrollTop);
  expect(saved).toBeGreaterThan(0);
  await course.click();
  await expect(page).toHaveURL(/tab=materials/);
  await page.getByRole('tab', { name: '课程', exact: true }).click();
  await expect(page.getByPlaceholder('搜索课程')).toHaveValue('英语备课');
  await expect(page.locator('.ant-pagination-item-active')).toHaveText('2');
  await expect(course).toBeVisible();
  await expect(page.locator('.course-filter-bar')).toContainText('五年级');
  await expect(page.locator('.course-filter-bar')).toContainText('English');
  await expect.poll(() => page.evaluate(() => document.scrollingElement!.scrollTop)).toBe(saved);
});

test('directory editor previews per-target changes and keeps material sync separate', async ({ page }) => {
  const user = { userId: 'ops', name: '测试教务', roles: ['super_admin'] };
  const source = { id: 'source', name: '源课程', grade: '五年级', subject: 'English', learningSpaceId: 'e1', status: '启用', curriculum: [{ id: 'u1', type: 'unit', name: '第一课', sortOrder: 1 }] };
  const target = { ...source, id: 'target', name: '目标课程', learningSpaceId: 'e2', curriculum: [] };
  const spaces = ['e1', 'e2'].map((id, i) => ({ id, grade: '五年级', subject: 'English', semester: 'S1', phase: 'Q1', level: i ? 'S+' : 'S', status: '启用' }));
  const writes: any[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'meeting-ui-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', async route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    let data: any = [];
    if (path === '/auth/me') data = user;
    if (path === '/courses') data = [source, target];
    if (path === `/courses/${source.id}` && route.request().method() === 'PUT') data = { ...source, ...route.request().postDataJSON() };
    if (path === '/materials') data = [{ id: 'm1', courseId: 'source', course: '源课程', learningSpaceId: 'e1', lessonId: 'u1', title: '课节讲义', fileId: 'file1', fileName: 'HD.pdf', tagCode: 'HD', status: '启用', publishStatus: '已发布', type: '课程讲义', previewStatus: '可预览', allowDownload: true }];
    if (path === '/subjects') data = [{ id: 'english', name: 'English', status: '启用' }];
    if (path === '/learning-spaces') data = spaces;
    if (path === '/courses/directory-sync-preview' || path === '/courses/directory-sync') {
      writes.push({ path, body: req.postDataJSON() });
      data = { targets: [{ courseId: 'target', courseName: '目标课程', added: ['第一课'], updated: [], preserved: 2, snapshot: 'checked-directory', ...(path.endsWith('preview') ? {} : { status: '已同步' }) }] };
    }
    await route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/content');
  await page.getByRole('row').filter({ hasText: '源课程' }).getByRole('button', { name: '编辑', exact: true }).click();
  const editor = page.getByRole('dialog', { name: '编辑课程', exact: true });
  await expect(editor.locator('.curriculum-section-heading')).toContainText('课程目录');
  await expect(editor.locator('.curriculum-section-heading').getByRole('button', { name: '同步整套目录', exact: true })).toBeVisible();
  await editor.getByRole('button', { name: '同步整套目录', exact: true }).click();
  await editor.getByRole('checkbox', { name: '目标课程', exact: true }).check();
  await page.screenshot({ path: '/tmp/starline-whole-directory-entry.png', fullPage: false });
  await editor.getByRole('button', { name: '保存并预览同步', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: '跨班型同步 · 源课程', exact: true });
  await expect(editor).toBeHidden();
  await expect(dialog).toContainText('新增 1 · 更新 0 · 保留 2');
  await page.screenshot({ path: '/tmp/starline-directory-sync-preview.png', fullPage: true });
  await dialog.getByRole('button', { name: /确认同步目录/ }).click();
  await expect(dialog).toContainText('已同步');
  expect(writes[1]).toEqual({ path: '/courses/directory-sync', body: { sourceCourseId: 'source', targetCourseIds: ['target'], snapshots: { target: 'checked-directory' } } });
  await dialog.getByRole('button', { name: '同步该课节讲义', exact: true }).click();
  await expect(page).toHaveURL(/tab=materials.*courseId=source.*syncLessonId=u1/);
  await expect(page.getByRole('dialog', { name: '跨班型同步课节讲义', exact: true })).toBeVisible();
});

for (const failure of ['database-reject', 'lost-response']) {
  test(`directory sync ${failure} preserves targets and requires a fresh preview`, async ({ page }) => {
    const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
    const source = { id: 'retry-source', name: '重试源课程', grade: '五年级', subject: 'English', learningSpaceId: 'retry-e1', status: '启用', curriculum: [{ id: 'retry-u1', type: 'unit', name: '第一课', sortOrder: 1 }] };
    const target = { ...source, id: 'retry-target', name: '重试目标课程', learningSpaceId: 'retry-e2', curriculum: [] as any[] };
    let previews = 0, attempts = 0, writes = 0;
    const snapshots: string[] = [];
    await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'directory-retry-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
    await page.route('**/api/**', async route => {
      const path = new URL(route.request().url()).pathname.replace('/api', '');
      let data: any = [];
      if (path === '/auth/me') data = user;
      if (path === '/courses') data = [source, target];
    if (path === `/courses/${source.id}` && route.request().method() === 'PUT') data = { ...source, ...route.request().postDataJSON() };
      if (path === '/subjects') data = [{ id: 'english', name: 'English', status: '启用' }];
      if (path === '/learning-spaces') data = ['retry-e1', 'retry-e2'].map(id => ({ id, grade: '五年级', subject: 'English', semester: 'S1', phase: 'Q1', status: '启用' }));
      if (path === '/courses/directory-sync-preview') {
        previews++;
        data = { targets: [{ courseId: target.id, courseName: target.name, added: writes ? [] : ['第一课'], updated: [], preserved: writes, snapshot: `fresh-${previews}` }] };
      }
      if (path === '/courses/directory-sync') {
        attempts++; snapshots.push(route.request().postDataJSON().snapshots[target.id]);
        if (attempts === 1 && failure === 'database-reject') return route.fulfill({ status: 400, json: { code: 400, message: '同步保存失败' } });
        if (!writes) { writes++; target.curriculum = [{ ...source.curriculum[0], id: 'stable-target-u1' }]; }
        if (attempts === 1) return route.abort('failed');
        data = { targets: [{ courseId: target.id, courseName: target.name, added: [], updated: [], preserved: 1, snapshot: 'confirmed', status: '已同步' }] };
      }
      return route.fulfill({ json: { code: 0, data } });
    });
    await page.goto('/content');
    await page.getByRole('row').filter({ hasText: source.name }).getByRole('button', { name: '编辑', exact: true }).click();
    const editor = page.getByRole('dialog', { name: '编辑课程', exact: true });
    await editor.getByRole('button', { name: '同步整套目录', exact: true }).click();
    await editor.getByRole('checkbox', { name: target.name, exact: true }).check();
    await editor.getByRole('button', { name: '保存并预览同步', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: `跨班型同步 · ${source.name}`, exact: true });
    await dialog.getByRole('button', { name: /确认同步目录/ }).click();
    await expect(dialog).toContainText('操作结果未确认');
    await expect(dialog.getByRole('checkbox', { name: target.name, exact: true })).toBeChecked();
    await expect(dialog.getByRole('button', { name: /确认同步目录/ })).toHaveCount(0);
    const retryPreview = dialog.getByRole('button', { name: /预览目录变更/ });
    await expect(retryPreview).toBeEnabled();
    await page.screenshot({ path: `/tmp/starline-directory-${failure}.png`, fullPage: false, animations: 'disabled' });
    await retryPreview.click();
    await expect(dialog).not.toContainText('操作结果未确认');
    await expect(dialog).toContainText(failure === 'lost-response' ? '新增 0' : '新增 1');
    await dialog.getByRole('button', { name: /确认同步目录/ }).click();
    await expect(dialog).toContainText('已同步');
    expect(previews).toBe(2); expect(attempts).toBe(2); expect(writes).toBe(1);
    expect(snapshots).toEqual(['fresh-1', 'fresh-2']);
  });
}

test('multi-file teaching plan upload retries only the failed file', async ({ page }) => {
  const user = { userId: 'ops', name: '测试教务', roles: ['super_admin'] };
  const posted: string[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'meeting-ui-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', async route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    if (req.method() === 'POST' && path === '/teaching-plans') {
      posted.push(req.postData() || '');
      if (posted.length === 2) { await route.fulfill({ status: 400, json: { code: 400, message: '第二份上传失败' } }); return; }
      await route.fulfill({ json: { code: 0, data: { id: `p${posted.length}` } } }); return;
    }
    await route.fulfill({ json: { code: 0, data: path === '/auth/me' ? user : path === '/teaching-plans' ? { plans: [], uploadScopes: [{ grade: '五年级', subject: 'English' }], canUpload: true, directories: [] } : [] } });
  });
  await page.goto('/teaching-plans');
  await page.getByRole('button', { name: /上传教案/ }).click();
  const drawer = page.getByRole('dialog', { name: '上传教案' });
  await drawer.getByLabel('选择教案文件', { exact: true }).setInputFiles([
    { name: 'success.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-first') },
    { name: 'retry.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-second') },
  ]);
  await drawer.getByRole('button', { name: '上传 2 份', exact: true }).click();
  await expect(drawer).toContainText('第二份上传失败');
  await expect(drawer).toContainText('已上传');
  await page.screenshot({ path: '/tmp/starline-teaching-plan-retry.png', fullPage: true });
  await drawer.getByRole('button', { name: '重试此文件', exact: true }).click();
  await expect.poll(() => posted.length).toBe(3);
  expect(posted[2]).toContain('filename="retry.pdf"');
  expect(posted[2]).not.toContain('filename="success.pdf"');
  await expect(drawer).toBeHidden();
});

test('teaching plans reuse curriculum on upload and let old plans change chapter', async ({ page }) => {
  const user = { userId: 'ops', name: '测试教务', roles: ['super_admin'], campusId: 'campus-main' };
  const directory = { id: 'c1', name: '五年级英语 S1 Q1', grade: '五年级', subject: 'English', curriculum: [{ id: 'u1', type: 'unit', name: '第一课', sortOrder: 1 }, { id: 'u2', type: 'unit', name: '第二课', sortOrder: 2 }] };
  const plan = { id: 'p1', title: '旧教案', grade: '五年级', subject: 'English', fileName: 'old.pdf', previewStatus: '待转换', uploaderName: '教师', createdAt: '2026-10-03' };
  const writes: { path: string; body: string | object }[] = [];
  await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'meeting-ui-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
  await page.route('**/api/**', async route => {
    const req = route.request(); const path = new URL(req.url()).pathname.replace('/api', '');
    let data: any = [];
    if (path === '/auth/me') data = user;
    if (path === '/teaching-plans') data = { plans: [plan], uploadScopes: [{ grade: '五年级', subject: 'English' }], canUpload: true, directories: [directory] };
    if (req.method() === 'POST' && path === '/teaching-plans') { writes.push({ path, body: req.postData() || '' }); data = { ...plan, id: 'p2' }; }
    if (req.method() === 'PUT') { writes.push({ path, body: req.postDataJSON() }); data = plan; }
    await route.fulfill({ json: { code: 0, data } });
  });
  await page.goto('/teaching-plans');
  await expect(page.getByText('未关联课程', { exact: true }).first()).toBeVisible();
  await page.getByRole('button', { name: /上传教案/ }).click();
  const drawer = page.getByRole('dialog', { name: '上传教案', exact: true });
  await expect(drawer).toContainText('五年级英语 S1 Q1');
  await drawer.locator('.ant-select').filter({ has: page.getByRole('combobox', { name: '教案章节', exact: true }) }).locator('.ant-select-selector').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option').first().click();
  await drawer.getByLabel('选择教案文件', { exact: true }).setInputFiles({ name: '章节教案.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-test') });
  await drawer.getByRole('button', { name: '上传 1 份', exact: true }).click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].body).toContain('name="courseId"\r\n\r\nc1');
  expect(writes[0].body).toContain('name="lessonId"\r\n\r\nu1');
  await expect(drawer).toBeHidden();
  await page.getByRole('button', { name: '关联章节', exact: true }).click();
  const modal = page.getByRole('dialog', { name: '关联教案章节' });
  await modal.locator('.ant-select').first().locator('.ant-select-selector').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option').first().click();
  await modal.locator('.ant-select').last().locator('.ant-select-selector').click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option').last().click();
  await page.screenshot({ path: '/tmp/starline-teaching-plan-chapters.png', fullPage: true });
  await modal.getByRole('button', { name: /确.*定/ }).click();
  await expect.poll(() => writes.length).toBe(2);
  expect(writes[1]).toEqual({ path: '/teaching-plans/p1/chapter', body: { courseId: 'c1', lessonId: 'u2' } });
});

test('failed material notice completion survives reload and retries without uploading again', async ({ page }) => {
 const user = { userId: 'ops', name: '管理员', roles: ['super_admin'] };
 const course = { id: 'recover-course', name: '提醒恢复课程', learningSpaceId: 'e1', grade: '五年级', subject: 'English', status: '启用', curriculum: [{ id: 'recover-lesson', type: 'unit', name: '第一课', sortOrder: 1 }] };
 let uploads = 0, completions = 0, batchId = '', pending = false;
 await page.addInitScript(user => { localStorage.setItem('starline_admin_token', 'recover-fixture'); localStorage.setItem('starline_admin_user', JSON.stringify(user)); }, user);
 await page.route('**/api/**', route => {
  const req = route.request(), path = new URL(req.url()).pathname.replace('/api', '');
  if (path === '/materials' && req.method() === 'POST') {
   uploads++; pending = true; batchId = (req.postData() || '').match(/name="batchId"\r\n\r\n([^\r]+)/)?.[1] || '';
   return route.fulfill({json:{code:0,data:{id:'recovered-material',fileName:'HD.pdf',tagCode:'HD',publishStatus:'已发布',status:'启用',courseId:course.id,lessonId:'recover-lesson'}}});
  }
  if (path.endsWith('/complete')) {
   completions++;
   if (completions === 1) return route.fulfill({status:500,json:{code:500,message:'提醒汇总暂时失败'}});
   expect(req.postDataJSON()).toEqual({courseId:course.id});
   expect(path).toContain(batchId); pending = false;
   return route.fulfill({json:{code:0,data:{resourceCount:1,recipientCount:1,alreadyCompleted:false}}});
  }
  const data = path === '/auth/me' ? user : path === '/courses' ? [course] : path === '/materials/notification-batches' ? (pending ? [{batchId,courseId:course.id,resourceCount:1}] : []) : path === '/learning-spaces' ? [{id:'e1',grade:'五年级',subject:'English',semester:'s1',phase:'q1',level:'S',status:'启用'}] : [];
  return route.fulfill({json:{code:0,data}});
 });
 await page.goto('/content?tab=materials&courseId=recover-course');
 await page.getByRole('button',{name:/上传讲义/}).click();
 const drawer = page.getByRole('dialog',{name:'给课节上传资料',exact:true});
 await drawer.locator('#lessonId').click();
 await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('Unit 1 · 第一课',{exact:true}).click();
 await drawer.locator('input[type="file"]').setInputFiles({name:'HD.pdf',mimeType:'application/pdf',buffer:Buffer.from('%PDF-fixture')});
 await drawer.getByRole('button',{name:/^上\s*传$/}).click();
 await expect(page.getByRole('dialog',{name:'同步本次课程讲义',exact:true})).toContainText('资料已上传，提醒汇总未完成');
 await page.reload();
 await expect(page.getByText('1 次上传的提醒待汇总',{exact:true})).toBeVisible();
 await page.screenshot({path:'/tmp/starline-material-notice-recovery.png',fullPage:false});
 await page.getByRole('button',{name:'汇总提醒',exact:true}).click();
 await expect(page.getByText('1 次上传的提醒待汇总',{exact:true})).toHaveCount(0);
 expect(uploads).toBe(1); expect(completions).toBe(2);
});

for (const entry of ['login', 'password-change', 'signed-in-login']) {
 test(`teacher plan reminder preserves target through ${entry}`, async ({page}) => {
  let changed = entry !== 'password-change';
  const teacher = {userId:'teacher',name:'教师',roles:['teacher'],teacherLibrary:{canManageCourses:false},canUploadHandout:false,canUploadQuestion:false,canReview:false};
  const plan = {id:'plan-reminder',title:'提醒中的教案',grade:'五年级',subject:'English',fileName:'reminder.pdf',fileSize:120,fileType:'pdf',previewStatus:'可预览',previewUrl:'/api/teaching-plans/plan-reminder/preview',createdAt:'2026-10-03 12:00:00'};
  if (entry === 'signed-in-login') await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','plan-reminder-fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},teacher);
  let previewRequests = 0;
  await page.route('**/api/**',route=>{
   const path = new URL(route.request().url()).pathname.replace('/api','');
   const user = {...teacher,mustChangePassword:!changed,authMethod:'password'};
   if (path.endsWith('/preview')) {previewRequests++;expect(route.request().headers()['authorization']).toBe('Bearer plan-reminder-fixture');return route.fulfill({contentType:'application/pdf',body:Buffer.from('%PDF-navigation-fixture')});}
   if (path === '/auth/change-password') changed = true;
   return route.fulfill({json:{code:0,data:path === '/auth/admin-password-login' ? {token:'plan-reminder-fixture',user,authMethod:'password'} : path === '/auth/me' ? user : path === '/teaching-plans' ? {plans:[plan],uploadScopes:[],canUpload:false} : []}});
  });
  await page.goto(entry === 'signed-in-login' ? '/login?plan=plan-reminder' : '/teaching-plans?plan=plan-reminder');
  if (entry !== 'signed-in-login') {
   await expect(page).toHaveURL('/login?plan=plan-reminder');
   await page.getByLabel('手机号',{exact:true}).fill('13800000004');
   await page.getByLabel('密码',{exact:true}).fill('temporary-password');
   await page.locator('button[type=submit]').click();
  }
  if (!changed) {
   await expect(page.getByText('修改初始密码',{exact:true})).toBeVisible();
   await page.getByLabel('临时密码',{exact:true}).fill('temporary-password');
   await page.getByLabel('新密码',{exact:true}).fill('new-password123');
   await page.getByLabel('确认新密码',{exact:true}).fill('new-password123');
   await page.getByRole('button',{name:'保存并重新登录'}).click();
   await expect(page).toHaveURL('/login?plan=plan-reminder');
   await page.getByLabel('手机号',{exact:true}).fill('13800000004');
   await page.getByLabel('密码',{exact:true}).fill('new-password123');
   await page.locator('button[type=submit]').click();
  }
  await expect(page).toHaveURL('/teaching-plans?plan=plan-reminder');
  const modal = page.getByRole('dialog').filter({hasText:'提醒中的教案'});
  await expect(modal).toBeVisible();
  await expect.poll(()=>previewRequests).toBe(1);
  await modal.getByRole('button',{name:'返回列表',exact:true}).click();
  await expect(page).toHaveURL('/teaching-plans');
  await expect(modal).not.toBeVisible();
 });
}

test('teacher plan reminder rejects missing and malformed targets without requesting files', async ({page})=>{
 const teacher={userId:'teacher',name:'教师',roles:['teacher'],teacherLibrary:{canManageCourses:false},canUploadHandout:false,canUploadQuestion:false,canReview:false};
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','plan-reminder-fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},teacher);
 let previews=0;
 await page.route('**/api/**',route=>{
  const path=new URL(route.request().url()).pathname.replace('/api','');
  if(path.endsWith('/preview')) previews++;
  return route.fulfill({json:{code:0,data:path==='/auth/me'?teacher:path==='/teaching-plans'?{plans:[],uploadScopes:[],canUpload:false}:[]}});
 });
 await page.goto('/teaching-plans?plan=private-plan');
 await expect(page.getByText('该教案不存在或当前账号无权查看',{exact:true})).toBeVisible();
 await expect(page.getByRole('dialog')).toHaveCount(0);
 await page.goto('/teaching-plans?plan=..%2Fsecret');
 await expect(page.getByRole('heading',{name:'教案',exact:true})).toBeVisible();
 await expect(page.getByRole('dialog')).toHaveCount(0);
 expect(previews).toBe(0);
});

test('teacher plan reminder clears the open preview after scope is withdrawn',async({page})=>{
 const user={userId:'teacher',name:'教师',roles:['teacher'],teacherLibrary:{canManageCourses:false},canUploadHandout:false,canUploadQuestion:false,canReview:false};
 const plan={id:'plan-revocable',title:'可撤回教案',grade:'五年级',subject:'English',fileName:'plan.pdf',previewStatus:'可预览',previewUrl:'/api/teaching-plans/plan-revocable/preview',createdAt:'2026-10-03 12:00:00'};
 let allowed=true;
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','plan-reminder-fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**',route=>{
  const path=new URL(route.request().url()).pathname.replace('/api','');
  if(path.endsWith('/preview'))return route.fulfill({contentType:'application/pdf',body:Buffer.from('%PDF-navigation-fixture')});
  return route.fulfill({json:{code:0,data:path==='/auth/me'?user:path==='/teaching-plans'?{plans:allowed?[plan]:[],uploadScopes:[],canUpload:false}:[]}});
 });
 await page.goto('/teaching-plans?plan=plan-revocable');
 await expect(page.locator('iframe[title="教案预览：可撤回教案"]')).toBeVisible();
 allowed=false;
 await page.evaluate(()=>{Object.defineProperty(document,'visibilityState',{configurable:true,value:'hidden'});window.dispatchEvent(new Event('visibilitychange'));Object.defineProperty(document,'visibilityState',{configurable:true,value:'visible'});window.dispatchEvent(new Event('visibilitychange'));});
 await expect(page.getByRole('dialog')).not.toBeVisible();
 await expect(page.locator('iframe[title="教案预览：可撤回教案"]')).toHaveCount(0);
 await expect(page.getByText('该教案不存在或当前账号无权查看',{exact:true})).toBeVisible();
 await page.screenshot({path:'/tmp/starline-teaching-plan-entry-revoked.png',fullPage:false});
 allowed=true;
 await page.evaluate(()=>{Object.defineProperty(document,'visibilityState',{configurable:true,value:'hidden'});window.dispatchEvent(new Event('visibilitychange'));Object.defineProperty(document,'visibilityState',{configurable:true,value:'visible'});window.dispatchEvent(new Event('visibilitychange'));});
 await expect(page.getByText('该教案不存在或当前账号无权查看',{exact:true})).toHaveCount(0);
 await expect(page.getByRole('dialog')).not.toBeVisible();
 await expect(page.locator('iframe[title="教案预览：可撤回教案"]')).toHaveCount(0);
});

test('teacher plan notice saves teacher-only draft mapping and controlled web origin',async({page})=>{
 const user={userId:'ops',name:'管理员',roles:['super_admin']};
 const binding={kind:'teaching_plans_uploaded',title:'教师教案上传',templateId:'',enabled:false,ready:false,triggerReady:true,requiredFields:{},availableFields:{resource_title:'教案名称',resource_count:'教案数量',published_at:'上传时间',teacher_name:'教师姓名',teaching_scope:'年级学科'},reason:''};
 const saved:any[]=[];
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','teacher-notice-fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**',route=>{
  const request=route.request(),path=new URL(request.url()).pathname.replace('/api','');
  if(request.method()==='PUT') {saved.push(request.postDataJSON());return route.fulfill({json:{code:0,data:[{...binding,...saved[0]}]}});}
  const data=path==='/auth/me'?user:path==='/official-account/automatic-notices'?[binding]:path==='/official-account/templates'?[{id:'teacher-template',title:'教案更新提醒',status:'启用',fields:[{key:'thing4',label:'教案'},{key:'thing5',label:'教师'},{key:'number3',label:'数量'},{key:'time7',label:'时间'}]}]:path==='/teachers'?[{id:'teacher-one',name:'张老师',accountStatus:'正常'},{id:'teacher-stopped',name:'停用老师',accountStatus:'停用'}]:path==='/students'?[{id:'student-one',name:'测试学生',grade:'五年级'}]:[];
  return route.fulfill({json:{code:0,data}});
 });
 await page.goto('/notices');
 await expect(page.getByRole('row').filter({hasText:'教师教案上传'})).toContainText('相关教师');
 await page.getByRole('row').filter({hasText:'教师教案上传'}).getByRole('button',{name:/配\s*置/}).click();
 const modal=page.getByRole('dialog',{name:'教师教案上传配置',exact:true});
 await modal.locator('.ant-select-selector').first().click();
 await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('教案更新提醒',{exact:true}).click();
 for(const [key,label] of [['thing4','教案名称'],['thing5','教师姓名'],['number3','教案数量'],['time7','上传时间']]) {
  const selector=modal.locator('.ant-select').filter({has:page.getByRole('combobox',{name:`映射 ${key}`,exact:true})}).locator('.ant-select-selector');
  await selector.scrollIntoViewIfNeeded();await selector.click();
  const listID=await modal.getByRole('combobox',{name:`映射 ${key}`,exact:true}).getAttribute('aria-controls');
  await page.locator('.ant-select-dropdown').filter({has:page.locator(`[id=${JSON.stringify(listID)}]`)}).getByText(label,{exact:true}).click();
 }
 const teacherSelect=modal.locator('.ant-select').filter({has:page.getByRole('combobox',{name:'试运行教师',exact:true})}).locator('.ant-select-selector');
 await teacherSelect.scrollIntoViewIfNeeded();await teacherSelect.click();
 const dropdown=page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)');
 await expect(dropdown.getByText('停用老师',{exact:true})).toHaveCount(0);
 await expect(dropdown.getByText('测试学生',{exact:true})).toHaveCount(0);
 await dropdown.getByText('张老师',{exact:true}).click();await page.keyboard.press('Escape');
 await modal.getByRole('textbox',{name:'教师网页域名',exact:true}).scrollIntoViewIfNeeded();
 await modal.getByRole('textbox',{name:'教师网页域名',exact:true}).fill('https://school.example');
 await modal.getByText('教师网页域名',{exact:true}).click();
 await expect(page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)')).not.toBeVisible();
 await expect(modal.getByRole('switch')).toBeEnabled();
 await expect(modal.getByRole('switch')).not.toBeChecked();
 await page.screenshot({path:'/tmp/starline-teacher-notice-template.png',fullPage:false});
 await modal.getByRole('button',{name:/确\s*定/}).click();
 await expect.poll(()=>saved.length).toBe(1);
 expect(saved[0]).toMatchObject({teacherIds:['teacher-one'],webOrigin:'https://school.example',enabled:false,fieldMappings:{thing4:'resource_title',thing5:'teacher_name',number3:'resource_count',time7:'published_at'}});
 expect(saved[0].studentIds?.length||0).toBe(0);
});

test('teacher batch completion recovers after reload without uploading successful files again', async ({ page }) => {
 const user={userId:'ops',name:'管理员',roles:['super_admin']};
 const uploads:string[]=[];const completions:string[]=[];let registeredBatch='';let pending=false;
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','teacher-batch-fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**',route=>{
  const request=route.request(),path=new URL(request.url()).pathname.replace('/api','');
  if(path==='/teaching-plans'&&request.method()==='POST') {
   const raw=request.postData()||'';uploads.push(raw);
   const id=raw.match(/name="batchId"\r\n\r\n([^\r\n]+)/)?.[1]||'';
   expect(id).not.toBe('');if(registeredBatch)expect(id).toBe(registeredBatch);registeredBatch=id;pending=true;
   return route.fulfill({json:{code:0,data:{id:`plan-${uploads.length}`}}});
  }
  if(path.endsWith('/complete')) {
   completions.push(path);expect(path).toBe(`/teaching-plans/notification-batches/${registeredBatch}/complete`);
   if(completions.length===1)return route.fulfill({status:500,json:{code:500,message:'提醒汇总暂时失败'}});
   pending=false;return route.fulfill({json:{code:0,data:{resourceCount:2,recipientCount:1,alreadyCompleted:false}}});
  }
  const data=path==='/auth/me'?user:path==='/teaching-plans'?{plans:[],canUpload:true,uploadScopes:[{grade:'五年级',subject:'English'}],directories:[]}:path==='/teaching-plans/notification-batches'?(pending?[{batchId:registeredBatch,resourceCount:2}]:[]):[];
  return route.fulfill({json:{code:0,data}});
 });
 await page.goto('/teaching-plans');await page.getByRole('button',{name:/上传教案/}).click();
 const drawer=page.getByRole('dialog',{name:'上传教案',exact:true});
 await drawer.getByLabel('选择教案文件',{exact:true}).setInputFiles([{name:'one.pdf',mimeType:'application/pdf',buffer:Buffer.from('%PDF-one')},{name:'two.pdf',mimeType:'application/pdf',buffer:Buffer.from('%PDF-two')}]);
 await drawer.getByRole('button',{name:'上传 2 份',exact:true}).click();
 await expect(drawer).toContainText('提醒汇总暂时失败');
 await expect(drawer.getByRole('button',{name:'上传 0 份',exact:true})).toBeDisabled();
 expect(uploads).toHaveLength(2);expect(completions).toHaveLength(1);
 await page.reload();await expect(page.getByText('有 1 批教案待汇总提醒',{exact:true})).toBeVisible();
 await page.screenshot({path:'/tmp/starline-teacher-batch-recovery.png',fullPage:false});
 await page.getByRole('button',{name:'汇总提醒',exact:true}).click();
 await expect(page.getByText('有 1 批教案待汇总提醒',{exact:true})).toHaveCount(0);
 expect(uploads).toHaveLength(2);expect(completions).toHaveLength(2);
});
