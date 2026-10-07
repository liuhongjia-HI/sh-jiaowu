import { expect, test, type Page } from '@playwright/test';
const user = { userId: 'list-ops', name: '测试教务', roles: ['super_admin'] };
const courses = Array.from({ length: 25 }, (_, i) => ({ id: `c${i}`, name: `英语课程 ${i+1}`, grade: '五年级', subject: 'English', status: '启用', learningSpaceId: 'e1', curriculum: [{ id: 'u1', type: 'unit', sortOrder: 1, title: 'Unit 1', name: 'Unit 1' }], lessonCount: 1 }));
const materials = courses.map((c,i) => ({ id: `m${i}`, courseId: c.id, course: c.name, lessonId: 'u1', title: '英语讲义', fileName: 'HD.pdf', tagCode: 'HD', status: '启用', publishStatus: '已发布', previewStatus: '可预览', allowDownload: true, createdAt: '2026-10-07 09:00:00' }));
async function fixture(page: Page) {
 await page.addInitScript(user => { localStorage.setItem('starline_admin_token','fixture'); localStorage.setItem('starline_admin_user',JSON.stringify(user)); },user);
 await page.route('**/api/**',route => {
  const path=new URL(route.request().url()).pathname.replace('/api','');
  const data=path==='/auth/me'?user:path==='/dashboard/overview'?null:path==='/courses'?courses:path==='/materials'?materials:path.startsWith('/materials/m')?materials[0]:path==='/subjects'?[{id:'english',name:'English',status:'启用'}]:path==='/materials/overview'?{lessons:[],cells:[],grades:[],subjects:[],semesters:[],phases:[],levels:[]}:path==='/teacher/library'?{courses,materials,spaces:[],recentMaterialIds:[],unreadMaterialIds:[],canDownload:true,policy:{}}:path==='/teaching-plans'?{plans:[],scopes:[],canUpload:true}:path==='/learning-spaces'?[{id:'e1',grade:'五年级',subject:'English',semester:'s1',phase:'q1',level:'1V1',status:'启用'}]:[];
  return route.fulfill({json:{code:0,message:'ok',data}});
 });
}
for(const [url,placeholder] of [
 ['/content?tab=materials&courseId=c0','搜索课节或文件名'],['/content','搜索课程'],['/content?tab=homework','搜索练习标题'],['/packages','搜索课程方案'],['/questions','搜索题目'],['/students','搜索姓名或手机号'],['/teaching-plans','搜索教案、文件名或上传人'],['/logs','搜索操作、对象或操作人']]) {
 test(`${url}: back and reload retain search`,async({page})=>{
  await fixture(page);await page.goto(url);await page.getByPlaceholder(placeholder).fill('英语');
  await page.locator('a[href="/dashboard"]').click();await page.goBack();
  await expect(page.getByPlaceholder(placeholder)).toHaveValue('英语');await page.reload();
  await expect(page.getByPlaceholder(placeholder)).toHaveValue('英语');
 });
}
test('materials: edit, tab return, dates, tags and reset',async({page})=>{
 await fixture(page);await page.goto('/content?tab=materials&courseId=c0');
 const search=()=>page.getByPlaceholder('搜索课节或文件名');await search().fill('HD');
 await page.getByRole('button',{name:'编辑HD',exact:true}).click();
 const dialog=page.getByRole('dialog');
 await dialog.getByPlaceholder('例如：五年级英语 S1 Q1 核心资料').fill('英语讲义');
 await dialog.locator('.ant-form-item').filter({hasText:'课程范围'}).locator('.ant-select-selector').click();
 await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('英语课程 1',{exact:true}).click();
 await dialog.locator('.ant-form-item').filter({hasText:'课节'}).locator('.ant-select-selector').click();
 await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('Unit 1',{exact:true}).click();
 await dialog.getByRole('button',{name:/保\s*存/}).click();
 await expect(dialog).not.toBeVisible();await expect(search()).toHaveValue('HD');
 await page.locator('input[type=date]').first().fill('2026-10-01');await page.getByRole('button',{name:'HD',exact:true}).click();
 await page.getByRole('tab',{name:'课程',exact:true}).click();await page.getByRole('tab',{name:'课程讲义',exact:true}).click();
 await expect(search()).toHaveValue('HD');await expect(page.locator('input[type=date]').first()).toHaveValue('2026-10-01');
 await expect(page.getByRole('button',{name:'HD',exact:true})).toHaveClass(/ant-btn-primary/);
 await page.getByRole('button',{name:/重\s*置/,exact:true}).click();await page.reload();
 await expect(search()).toHaveValue('');await expect(page.locator('input[type=date]').first()).toHaveValue('');
});
test('material pagination survives back; filter changes reset page',async({page})=>{
 await fixture(page);await page.goto('/content?tab=materials');await page.locator('.ant-pagination-item-2').last().click();
 await page.locator('a[href="/dashboard"]').click();await page.goBack();
 await expect(page.locator('.ant-pagination-item-active').last()).toHaveText('2');await page.getByPlaceholder('搜索课节或文件名').fill('HD');
 await expect(page.locator('.ant-pagination-item-active').last()).toHaveText('1');
});
test('account isolation and invalid storage fallback',async({page})=>{
 await fixture(page);await page.goto('/questions');await page.getByPlaceholder('搜索题目').fill('数学');
 await page.evaluate(()=>{const u=JSON.parse(localStorage.getItem('starline_admin_user')!);u.userId='other';localStorage.setItem('starline_admin_user',JSON.stringify(u));});
 await page.locator('a[href="/dashboard"]').click();await page.locator('a[href="/questions"]').click();await expect(page.getByPlaceholder('搜索题目')).toHaveValue('');
 await page.locator('a[href="/dashboard"]').click();await page.evaluate(()=>sessionStorage.setItem('starline:list-state:v1:other:questions:keyword','{invalid'));
 await page.locator('a[href="/questions"]').click();await expect(page.getByPlaceholder('搜索题目')).toHaveValue('');
});

test('teacher library restores URL filters on menu return and reset clears defaults',async({page})=>{
 await fixture(page);await page.goto('/teacher-library?q=英语&tag=HD');
 const search=()=>page.getByPlaceholder('搜索课程、课节、讲义或文件名');
 await expect(search()).toHaveValue('英语');
 await page.locator('a[href="/dashboard"]').click();
 await page.goto('/teacher-library');
 await expect(search()).toHaveValue('英语');
 await page.getByRole('button',{name:/重\s*置/,exact:true}).click();await expect(search()).toHaveValue('');
 await page.reload();await expect(search()).toHaveValue('');
});

test('calendar dates and filters restore after reload as usable dates',async({page})=>{
 await fixture(page);await page.goto('/scheduling');
 await page.getByRole('button',{name:'下一期',exact:true}).click();
 const date=await page.locator('.calendar-toolbar strong').textContent();
 await page.locator('summary').filter({hasText:'更多筛选'}).click();
 await page.locator('.ant-select').filter({has:page.getByRole('combobox',{name:'年级筛选',exact:true})}).locator('.ant-select-selector').click();
 await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText('五年级',{exact:true}).click();
 await page.reload();await expect(page.locator('.calendar-toolbar strong')).toHaveText(date!);
 await expect(page.locator('summary').filter({hasText:'更多筛选'})).toContainText('已生效');
 await page.getByRole('button',{name:'下一期',exact:true}).click();await expect(page.locator('.calendar-toolbar strong')).not.toHaveText(date!);
});
