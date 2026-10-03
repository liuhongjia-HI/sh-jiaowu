import { test, expect } from '@playwright/test';

test('unread entry marks only successfully opened files and reflects new versions', async ({ page }) => {
 const user={userId:'teacher',name:'测试老师',roles:['teacher'],teacherLibrary:{spaceIds:['e1'],scopes:[],canDownload:true,canManageCourses:false},canUploadHandout:false,canUploadQuestion:false,canReview:false};
 const course={id:'c1',name:'五年级英语',subject:'English',grade:'五年级',learningSpaceId:'e1',status:'启用',curriculum:[{id:'u1',name:'第一课',type:'unit',sortOrder:1}]};
 const materials=['m1','m2'].map((id,i)=>({id,title:i?'第二份资料':'第一份资料',courseId:'c1',course:course.name,lessonId:'u1',fileName:`${id}.pdf`,previewStatus:'可预览',previewUrl:`/api/files/${id}/preview`,status:'启用',curriculum:{unit:'第一课'},updatedAt:`2026-10-0${i+1} 09:00:00`}));
 let unread=['m1','m2'];const views:string[]=[];
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','unread-fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**',async route=>{
  const path=new URL(route.request().url()).pathname.replace('/api','');let data:any=[];
  if(path==='/auth/me')data=user;
  if(path==='/teacher/library')data={courses:[course],spaces:[{id:'e1',name:'五年级英语',subject:'English',grade:'五年级',status:'启用'}],materials,recentMaterialIds:views,unreadMaterialIds:unread,canDownload:true,policy:user.teacherLibrary};
  if(path==='/files/m1/preview')return route.fulfill({contentType:'application/pdf',body:Buffer.from('%PDF-1.4 test fixture')});
  if(path==='/files/m2/preview')return route.fulfill({status:400,json:{code:400,message:'预览文件不可用'}});
  if(path==='/teacher/materials/m1/view'){views.push('m1');unread=unread.filter(id=>id!=='m1');data={};}
  await route.fulfill({json:{code:0,data}});
 });
 await page.goto('/teacher-library');
 await page.getByRole('button',{name:'新增与未读资料',exact:true}).click();
 await expect(page).toHaveURL(/unread=1/);
 const table=page.locator('.ant-table');await expect(table.getByText('未读',{exact:true})).toHaveCount(2);
 await table.getByRole('button',{name:'第一份资料',exact:true}).click();
 await expect(page.getByTitle('讲义预览：第一份资料')).toBeVisible();
 await expect.poll(()=>views.length).toBe(1);
 await page.getByRole('button',{name:'返回讲义列表',exact:true}).click();
 await expect(table.getByRole('button',{name:'第一份资料',exact:true})).toHaveCount(0);
 await expect(table.getByRole('button',{name:'第二份资料',exact:true})).toBeVisible();
 await table.getByRole('button',{name:'第二份资料',exact:true}).click();
 await expect(page.getByText('预览文件不可用',{exact:true})).toBeVisible();
 expect(views).toEqual(['m1']);
 await page.getByRole('button',{name:'返回讲义列表',exact:true}).click();
 unread=['m1','m2'];
 await page.getByRole('button',{name:/刷新资料/}).click();
 await expect(table.getByRole('button',{name:'第一份资料',exact:true})).toBeVisible();
 await expect(table.getByText('未读',{exact:true})).toHaveCount(2);
});
