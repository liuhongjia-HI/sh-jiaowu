import { test, expect } from '@playwright/test';
const user = {id:'ops',name:'运营',roles:['ops_staff'],roleLabel:'运营',mustChangePassword:false};
test('指定目标查询、发送及清除过期目标', async ({page}) => {
 const writes: any[]=[];
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**', async route=>{
  const req=route.request();const url=new URL(req.url());const path=url.pathname.replace('/api','');let data:any=[];
  if(path==='/auth/me')data=user;
  if(path==='/official-account/templates')data=[{id:'test',title:'服务通知',fields:[{key:'thing1',label:'通知内容',maxLength:20}]}];
  if(path==='/official-account/recipient')data={guardianId:'g1',name:'测试家长',phone:url.searchParams.get('phone'),studentNames:['学生甲'],reachable:true};
  if(path==='/official-account/campaigns/preview'){const body=req.postDataJSON();expect(body.recipientMode).toBe('specified');expect(body.guardianIds).toEqual(['g1']);expect(body.grades).toEqual([]);data={studentCount:1,guardianCount:1,reachableCount:1,unreachableCount:0};}
  if(path==='/official-account/campaigns'&&req.method()==='POST'){writes.push(req.postDataJSON());data={id:'c1',targetCount:1,status:'发送中'};}
  await route.fulfill({json:{code:0,message:'ok',data}});
 });
 await page.goto('/notices');await page.getByRole('tab',{name:'消息推送',exact:true}).click();
 const send=page.locator('.official-actionbar').getByRole('button',{name:/确\s*认\s*发\s*送/});await expect(send).toBeDisabled();
 await page.getByPlaceholder('输入已登录小程序的家长手机号').fill('18518673993');await page.getByRole('button',{name:/查\s*询\s*账\s*号/}).click();
 await expect(page.getByText('已匹配 测试家长',{exact:false})).toBeVisible();await page.getByPlaceholder('请输入通知内容').fill('推送联调测试');await expect(send).toBeEnabled();
 await send.click();await page.getByRole('dialog').getByRole('button',{name:/确\s*认\s*发\s*送/}).click();await expect.poll(()=>writes.length).toBe(1);
 await expect(page.getByRole('dialog')).toHaveCount(0); expect(writes[0].guardianIds).toEqual(['g1']);expect(writes[0].requestId).toBeTruthy();expect(writes[0].grades).toEqual([]);
 await page.getByPlaceholder('输入已登录小程序的家长手机号').fill('18518673994');await expect(send).toBeDisabled();
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:'/tmp/starline-official-targeted-narrow.png',fullPage:true});
});

test('课程取消模板仅提供匹配类型的业务字段', async ({page}) => {
 const writes:any[]=[];
 const binding={kind:'schedule_cancelled',title:'课程取消',templateId:'',enabled:false,ready:false,requiredFields:{},availableFields:{course_name:'原课程名称',student_name:'学生姓名',lesson_time:'原上课时间',cancelled_at:'取消时间',lesson_count:'取消课次数量'},triggerReady:true};
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**',async route=>{
  const req=route.request();const path=new URL(req.url()).pathname.replace('/api','');let data:any=[];
  if(path==='/auth/me')data=user;
  if(path==='/official-account/automatic-notices')data=[binding];
  if(path==='/official-account/templates')data=[{id:'cancel-test',title:'取消模板测试',fields:[{key:'thing8',label:'课程'},{key:'time10',label:'原上课时间'}]}];
  if(req.method()==='PUT'){writes.push(req.postDataJSON());data=[binding];}
  await route.fulfill({json:{code:0,message:'ok',data}});
 });
 await page.goto('/notices');await page.getByRole('button',{name:/配\s*置/}).click();
 const dialog=page.getByRole('dialog',{name:'课程取消配置'});
 await dialog.locator('.ant-select-selector').first().click();await page.getByText('取消模板测试',{exact:true}).click();
 await dialog.getByRole('combobox',{name:'映射 time10'}).click();
 await expect(page.locator('.ant-select-item-option-content').getByText('原上课时间',{exact:true})).toBeVisible();
 await expect(page.locator('.ant-select-item-option-content').getByText('原课程名称',{exact:true})).toHaveCount(0);
 await page.locator('.ant-select-item-option-content').getByText('原上课时间',{exact:true}).click();
 await dialog.getByRole('combobox',{name:'映射 thing8'}).click();await page.locator('.ant-select-item-option-content').getByText('原课程名称',{exact:true}).click();
 await dialog.getByRole('button',{name:/确\s*定/}).click();await expect.poll(()=>writes.length).toBe(1);
 expect(writes[0].fieldMappings).toEqual({time10:'lesson_time',thing8:'course_name'});expect(writes[0].enabled).toBe(false);
});

for (const kind of ['homework_published', 'review_completed']) {
 test(`${kind} 模板映射保存且不会开启推送`, async ({page}) => {
  const writes:any[]=[];const review=kind==='review_completed';
  const binding={kind,title:review?'批改完成':'作业发布',enabled:false,ready:false,requiredFields:{},availableFields:{course_name:'课程名称',homework_title:'作业名称',student_name:'学生姓名',...(review?{reviewed_at:'完成时间',score:'最终得分'}:{published_at:'发布时间',deadline_at:'截止时间'})},triggerReady:true};
  await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
  await page.route('**/api/**',async route=>{
   const req=route.request();const path=new URL(req.url()).pathname.replace('/api','');let data:any=[];
   if(path==='/auth/me')data=user;
   if(path==='/official-account/automatic-notices')data=[binding];
   if(path==='/official-account/templates')data=[{id:'homework-test',title:'业务模板测试',fields:[{key:'thing2',label:'作业'},{key:'time4',label:'业务时间'}]}];
   if(req.method()==='PUT'){writes.push(req.postDataJSON());data=[binding];}
   await route.fulfill({json:{code:0,message:'ok',data}});
  });
  await page.goto('/notices');await page.getByRole('button',{name:/配\s*置/}).click();const dialog=page.getByRole('dialog');
  await dialog.locator('.ant-select-selector').first().click();await page.getByText('业务模板测试',{exact:true}).click();
  await dialog.getByRole('combobox',{name:'映射 thing2'}).click();await page.locator('.ant-select-item-option-content').getByText('作业名称',{exact:true}).click();
  await dialog.getByRole('combobox',{name:'映射 time4'}).click();await expect(page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option-content').getByText('作业名称',{exact:true})).toHaveCount(0);await page.locator('.ant-select-item-option-content').getByText(review?'完成时间':'发布时间',{exact:true}).click();
  await dialog.getByRole('button',{name:/确\s*定/}).click();await expect.poll(()=>writes.length).toBe(1);expect(writes[0].fieldMappings).toEqual({thing2:'homework_title',time4:review?'reviewed_at':'published_at'});expect(writes[0].enabled).toBe(false);
 });
}

test('课前提醒拒绝预约结果模板且可配置实际字段', async ({page}) => {
 const writes:any[]=[];const binding={kind:'schedule_reminder',title:'课前提醒',templateId:'',enabled:false,ready:false,requiredFields:{},availableFields:{course_name:'课程名称',student_name:'学生姓名',lesson_time:'上课时间'},triggerReady:true};
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**',async route=>{
  const req=route.request();const path=new URL(req.url()).pathname.replace('/api','');let data:any=[];
  if(path==='/auth/me')data=user;
  if(path==='/official-account/automatic-notices')data=[binding];
  if(path==='/official-account/templates')data=[{id:'reservation',title:'课程预约结果通知',fields:[{key:'thing8'},{key:'time10'}]},{id:'reminder',title:'上课提醒',fields:[{key:'thing8',label:'课程'},{key:'time10',label:'上课时间'}]}];
  if(req.method()==='PUT'){writes.push(req.postDataJSON());data=[binding];}
  await route.fulfill({json:{code:0,message:'ok',data}});
 });
 await page.goto('/notices');await page.getByRole('button',{name:/配\s*置/}).click();const dialog=page.getByRole('dialog');await dialog.locator('.ant-select-selector').first().click();
 await expect(page.locator('.ant-select-item-option-disabled').filter({hasText:'课程预约结果通知'})).toBeVisible();await page.getByText('上课提醒',{exact:true}).click();
 await dialog.getByRole('combobox',{name:'映射 thing8'}).click();await page.locator('.ant-select-item-option-content').getByText('课程名称',{exact:true}).click();
 await dialog.getByRole('combobox',{name:'映射 time10'}).click();await page.locator('.ant-select-item-option-content').getByText('上课时间',{exact:true}).click();
 await dialog.getByRole('button',{name:/确\s*定/}).click();await expect.poll(()=>writes.length).toBe(1);expect(writes[0].fieldMappings).toEqual({thing8:'course_name',time10:'lesson_time'});expect(writes[0].enabled).toBe(false);
});

test('异常原因配置保留指定家长范围，保存不自动开启',async({page})=>{
 const writes:any[]=[];
 const binding={kind:'review_exception',title:'作业批改异常',templateId:'exception',enabled:false,ready:false,requiredFields:{thing7:'作业名称',thing5:'班级',const2:'异常原因'},approvedReasons:[],guardianIds:['g1'],studentIds:['s1']};
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**',async route=>{
  const req=route.request();const path=new URL(req.url()).pathname.replace('/api','');let data:any=[];
  if(path==='/auth/me')data=user;
  if(path==='/students')data=[{id:'s1',name:'本地学生',grade:'五年级'}];
  if(path==='/official-account/automatic-notices')data=[binding];
  if(path==='/official-account/templates')data=[{id:'exception',title:'批改异常通知',fields:[{key:'thing7',label:'作业名称'},{key:'thing5',label:'班级'},{key:'const2',label:'异常原因'}]}];
  if(req.method()==='PUT'){writes.push(req.postDataJSON());data=[{...binding,...writes.at(-1)}];}
  await route.fulfill({json:{code:0,message:'ok',data}});
 });
 await page.goto('/notices');await page.getByRole('button',{name:/配\s*置/}).click();
 const modal=page.getByRole('dialog',{name:'作业批改异常配置'});
 await modal.getByRole('combobox',{name:'微信审核异常原因'}).fill('作业缺页');await modal.getByRole('combobox',{name:'微信审核异常原因'}).press('Enter');
 await modal.getByRole('button',{name:/确\s*定/}).click();await expect.poll(()=>writes.length).toBe(1);
 expect(writes[0].approvedReasons).toEqual(['作业缺页']);expect(writes[0].guardianIds).toEqual(['g1']);expect(writes[0].studentIds).toEqual(['s1']);expect(writes[0].enabled).toBe(false);
});

for (const approvedReasons of [[], ['已审核选项']]) {
 test(`批改异常固定选项：${approvedReasons.length ? '只允许选择审核值' : '缺少配置禁止发送但可存草稿'}`, async ({page}) => {
  const writes:any[]=[];
  await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
  await page.route('**/api/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname.replace('/api','');let data:any=[];
   if(path==='/auth/me')data=user;
   if(path==='/official-account/templates')data=[{id:'exception',title:'作业批改异常提醒',fields:[{key:'thing7',label:'作业名称',maxLength:20},{key:'const2',label:'异常原因',maxLength:20}]}];
   if(path==='/official-account/automatic-notices')data=[{kind:'review_exception',title:'批改异常提醒',templateId:'exception',approvedReasons,enabled:false,ready:false,requiredFields:{}}];
   if(path==='/official-account/recipient')data={guardianId:'g1',name:'测试家长',studentNames:['学生甲'],reachable:true};
   if(path==='/official-account/campaigns/preview')data={studentCount:1,guardianCount:1,reachableCount:1,unreachableCount:0};
   if(path==='/official-account/campaigns'&&req.method()==='POST'){writes.push(req.postDataJSON());data={id:'c1',targetCount:1,status:req.postDataJSON().draft?'草稿':'发送中'};}
   await route.fulfill({json:{code:0,message:'ok',data}});
  });
  await page.goto('/notices');await page.getByRole('tab',{name:'消息推送',exact:true}).click();
  await page.getByPlaceholder('输入已登录小程序的家长手机号').fill('18518673993');await page.getByRole('button',{name:/查\s*询\s*账\s*号/}).click();
  await expect(page.getByText('已匹配 测试家长',{exact:false})).toBeVisible();await page.getByPlaceholder('请输入作业名称').fill('明确标注测试');
  const send=page.locator('.official-actionbar').getByRole('button',{name:/确\s*认\s*发\s*送/});
  await expect(send).toBeDisabled();
  const reason=page.getByRole('combobox',{name:'异常原因',exact:true});
  if(!approvedReasons.length){
   await expect(reason).toBeDisabled();await expect(page.getByText('异常原因是微信固定选项。',{exact:false})).toBeVisible();
   await page.getByRole('button',{name:/保\s*存\s*草\s*稿/}).click();await expect.poll(()=>writes.length).toBe(1);expect(writes[0].draft).toBe(true);
   await page.setViewportSize({width:390,height:844});await page.screenshot({path:'/tmp/starline-exception-reason-missing.png',fullPage:true});
  }else{
   await reason.click();await page.locator('.ant-select-item-option-content').getByText('已审核选项',{exact:true}).click();await expect(send).toBeEnabled();await send.click();await page.getByRole('dialog').getByRole('button',{name:/确\s*认\s*发\s*送/}).click();await expect.poll(()=>writes.length).toBe(1);expect(writes[0].values.const2).toBe('已审核选项');
  }
 });
}
