import { test, expect } from '@playwright/test';

test('real local API: configuration, approval, reschedule and eligible retry', async ({ page, request }) => {
 const api='http://127.0.0.1:18992';
 async function json(path:string,method='GET',token='',data?:unknown) {
  const response=await request.fetch(`${api}/api${path}`,{method,headers:token?{Authorization:`Bearer ${token}`}:{},data});
  expect(response.ok(),`${method} ${path}`).toBeTruthy();
  const body=await response.json();expect(body.code).toBe(0);return body.data;
 }
 const ops=await json('/auth/admin-password-login','POST','',{phone:'13800000003',password:'123456'});
 const teacher=await json('/auth/admin-password-login','POST','',{phone:'13800000004',password:'123456'});
 await page.addInitScript(auth=>{localStorage.setItem('starline_admin_token',auth.token);localStorage.setItem('starline_admin_user',JSON.stringify(auth.user));},ops);
 await page.goto('/notices');
 async function enable(title:string,template:string) {
  await page.getByRole('row').filter({has:page.getByText(title,{exact:true})}).getByRole('button',{name:/配\s*置/}).click();
  const dialog=page.getByRole('dialog',{name:`${title}配置`});
  await dialog.locator('.ant-select-selector').first().click();
  await page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').getByText(template,{exact:true}).click();
  await dialog.getByRole('switch').click();await dialog.getByRole('button',{name:/确\s*定/}).click();
  await page.getByRole('button',{name:/开\s*启/}).click();
  await expect(dialog).toBeHidden();
 }
 await enable('排课确认','排课时间已确认通知');
 await enable('调课成功','调课成功通知');
 await enable('课前提醒','课程预约结果通知');
 const meta=await (await request.get(`${api}/__notice/meta`)).json();
 const created=await json('/schedule-classes','POST',teacher.token,{courseId:'course-g05-english-s1-q1',teacherId:'user-teacher',classType:'1V1',durationMinutes:60,startTime:'16:00',endTime:'17:00',startDate:meta.date,studentIds:['stu-001'],ignoreWarnings:true});
 expect(created.auditStatus).toBe('待审核');
 expect(await json('/official-account/automatic-notice-tasks','GET',ops.token)).toHaveLength(0);
 await page.goto('/scheduling');
 await page.getByRole('row').filter({hasText:'16:00'}).getByRole('button',{name:/通\s*过/}).click();
 await expect.poll(async()=> (await json('/official-account/automatic-notice-tasks','GET',ops.token)).length).toBe(1);
 for(const minutes of [0,1,6,21]) expect((await request.post(`${api}/__notice/process?minutes=${minutes}`)).ok()).toBeTruthy();
 await page.goto('/notices');
 await expect(page.getByRole('button',{name:'补发',exact:true})).toHaveCount(1);
 await expect(page.getByText('本地模拟微信限流',{exact:true})).toBeVisible();
 expect((await request.post(`${api}/__notice/success`)).ok()).toBeTruthy();
 await page.getByRole('button',{name:'补发',exact:true}).click();
 await expect.poll(async()=> (await json('/official-account/automatic-notice-tasks','GET',ops.token)).find((task:any)=>task.kind==='schedule_confirmed').status).toBe('待发送');
 expect((await request.post(`${api}/__notice/process`)).ok()).toBeTruthy();
 await page.getByRole('button',{name:/刷\s*新/}).click();
 await expect(page.getByText('微信已受理',{exact:true})).toBeVisible();
 await page.goto('/scheduling');
 await page.getByRole('button',{name:'下一期',exact:true}).click();await page.getByRole('button',{name:'下一期',exact:true}).click();
 await page.locator('.calendar-scroll').evaluate(el=>el.scrollTop=600);
 await page.locator('.calendar-column .is-class').first().click();
 const detail=page.getByRole('dialog',{name:'课程详情'});
 await detail.locator('#startTime').fill('17:00');await detail.locator('#endTime').fill('18:00');
 await detail.getByRole('button',{name:'保存调课',exact:true}).click();await expect(detail).toBeHidden();
 await expect.poll(async()=> (await json('/official-account/automatic-notice-tasks','GET',ops.token)).some((task:any)=>task.kind==='schedule_changed')).toBe(true);
 expect((await request.post(`${api}/__notice/process`)).ok()).toBeTruthy();
 const tasks=await json('/official-account/automatic-notice-tasks','GET',ops.token);
 const changed=tasks.find((task:any)=>task.kind==='schedule_changed');
 expect(changed.status).toBe('微信已受理');expect(changed.values.time2).toContain('16:00');expect(changed.values.time4).toContain('17:00');
 await page.goto('/notices');await expect(page.getByRole('button',{name:'补发',exact:true})).toHaveCount(0);
 await expect(page.getByText('微信已受理',{exact:true})).toHaveCount(2);
 await expect(page.locator('.ant-spin-spinning')).toHaveCount(0);
 await page.screenshot({path:'/tmp/starline-notice-real-api-flow.png',fullPage:true});
 // Cancellation must invalidate the new reminder, without re-sending history.
 await page.goto('/scheduling');
 await page.getByRole('button',{name:'下一期',exact:true}).click();await page.getByRole('button',{name:'下一期',exact:true}).click();
 await page.locator('.calendar-scroll').evaluate(el=>el.scrollTop=600);
 await page.locator('.calendar-column .is-class').first().click();
 await page.getByRole('dialog',{name:'课程详情'}).getByRole('button',{name:'取消课程',exact:true}).click();
 const cancel=page.getByRole('dialog',{name:'取消课程',exact:true});
 await cancel.getByRole('button',{name:'取消课程',exact:true}).click();await expect(cancel).toBeHidden();
 expect((await request.post(`${api}/__notice/process`)).ok()).toBeTruthy();
 const cancelled=await json('/official-account/automatic-notice-tasks','GET',ops.token);
 const reminders=cancelled.filter((task:any)=>task.kind==='schedule_reminder');
 expect(reminders).toHaveLength(2);expect(reminders.every((task:any)=>task.status==='已失效')).toBe(true);
 expect(cancelled.filter((task:any)=>task.status==='微信已受理')).toHaveLength(2);
 const student=await json('/auth/demo-student-login','POST','',{phone:'18500009069',password:'123456'});
 const historical=await json(`/student/business-notices/${changed.eventId}`,'GET',student.token);
 expect(historical.currentLessons[0].status).toBe('已取消');

});
