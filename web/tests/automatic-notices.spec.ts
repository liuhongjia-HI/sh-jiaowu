import { test, expect } from '@playwright/test';
const user = { id: 'user-ops', name: '运营测试', phone: '13900000003', roles: ['ops_staff'], roleLabel: '运营', campusId: 'campus-main', mustChangePassword: false };
const bindings = [ { kind: 'schedule_confirmed', title: '排课确认', templateId: 'confirmed', enabled: false, ready: true, requiredFields: { thing1: '课程', thing2: '学生', time4: '时间' } }, { kind: 'schedule_changed', title: '调课成功', templateId: '', enabled: false, ready: false, reason: '请选择已同步的模板', requiredFields: { thing12: '课程', time2: '调前', time4: '调后', thing7: '学生' } }, { kind: 'review_exception', title: '作业批改异常', templateId: '', enabled: false, ready: false, reason: '后续阶段暂不可启用', requiredFields: { thing7: '作业', thing5: '班级', const2: '原因' } } ];
const tasks = [ { id: 'retry', studentName: '孩子甲', guardianName: '妈妈', kind: 'schedule_confirmed', status: '发送失败', failureReason: '微信暂时限流', dueAt: '2030-10-03T10:00:00+08:00', attempts: 4, retryable: true }, { id: 'unknown', studentName: '孩子乙', guardianName: '爸爸', kind: 'schedule_changed', status: '结果待确认', dueAt: '2030-10-03T10:00:00+08:00', attempts: 1, retryable: false } ];
test('configuration, template matching, filtering and eligible retry', async ({ page }) => {
 const writes: any[] = [];
 await page.addInitScript(user => { localStorage.setItem('starline_admin_token','notice-fixture'); localStorage.setItem('starline_admin_user',JSON.stringify(user)); }, user);
 await page.route('**/api/**', async route => {
  const request=route.request(); const path=new URL(request.url()).pathname.replace('/api',''); let data: any=[];
  if(path==='/auth/me') data=user;
  if(path==='/official-account/automatic-notices') data=bindings;
  if(path==='/official-account/automatic-notice-tasks') data=tasks;
  if(path==='/students') data=[{id:'s1',name:'孩子甲',grade:'G5'}];
  if(path==='/official-account/templates') data=[{id:'confirmed',title:'排课时间已确认通知',fields:[{key:'thing1'},{key:'thing2'},{key:'time4'}]},{id:'wrong',title:'不匹配模板',fields:[{key:'thing12'}]}];
  if(request.method()==='PUT'){writes.push(request.postDataJSON());data=bindings;}
  if(path.endsWith('/retry')){writes.push({retry:path});data={};}
  await route.fulfill({json:{code:0,message:'ok',data}});
 });
 await page.goto('/notices');
 await expect(page.getByRole('tab',{name:'自动通知',exact:true})).toHaveAttribute('aria-selected','true');
 await page.getByRole('row').filter({has:page.getByText('排课确认',{exact:true})}).getByRole('button',{name:/配\s*置/}).first().click();
 const dialog=page.getByRole('dialog',{name:'排课确认配置'});
 await dialog.locator('.ant-select-selector').first().click();
 await expect(page.locator('.ant-select-item-option-disabled').filter({hasText:'不匹配模板'})).toBeVisible();
 await page.keyboard.press('Escape'); await dialog.getByRole('switch').click();
 await dialog.getByRole('button',{name:/确\s*定/}).click(); await page.getByRole('button',{name:/开\s*启/}).click();
 await expect.poll(()=>writes.length).toBe(1); expect(writes[0].enabled).toBe(true);
 await expect(page.getByRole('row').filter({has:page.getByText('作业批改异常',{exact:true})}).getByRole('button',{name:/配\s*置/})).toBeDisabled();
 await expect(page.getByRole('button',{name:'补发',exact:true})).toHaveCount(1);
 await page.getByPlaceholder('学生、家长或失败原因').fill('孩子乙'); await expect(page.getByRole('button',{name:'补发',exact:true})).toHaveCount(0);
 await page.getByPlaceholder('学生、家长或失败原因').fill('孩子甲'); await page.getByRole('button',{name:'补发',exact:true}).click();
 await expect.poll(()=>writes.length).toBe(2); await page.getByPlaceholder('学生、家长或失败原因').fill('');
 await page.screenshot({path:'/tmp/starline-automatic-notices.png',fullPage:true});
});
