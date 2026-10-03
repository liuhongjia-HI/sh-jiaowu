import { test, expect } from '@playwright/test';

test('disabling teacher with outstanding lessons warns before saving and preserves grants', async ({ page }) => {
 const user={userId:'super',name:'管理员',roles:['super_admin']};
 let teacher={id:'t1',name:'英语老师',phone:'13911223344',learningSpaceIds:['e1'],learningSpaces:['五年级英文'],grades:['五年级'],subjects:['English'],accountStatus:'正常',bindStatus:'待绑定',activeClassCount:3,canUploadHandout:false,canUploadQuestion:false,canReview:false,remark:'',teacherLibrary:{spaceIds:['e1'],scopes:[],canDownload:true,canManageCourses:false,canViewDrafts:false}};
 const writes:any[]=[];
 await page.addInitScript(user=>{localStorage.setItem('starline_admin_token','teacher-stop-fixture');localStorage.setItem('starline_admin_user',JSON.stringify(user));},user);
 await page.route('**/api/**',async route=>{
  const request=route.request();const path=new URL(request.url()).pathname.replace('/api','');let data:any=[];
  if(path==='/auth/me')data=user;
  if(path==='/teachers')data=[teacher];
  if(path==='/subjects')data=[{id:'english',name:'English',status:'启用'}];
  if(path==='/learning-spaces')data=[{id:'e1',name:'五年级英文',grade:'五年级',subject:'English',semester:'S1',phase:'Q1',level:'S',status:'启用'}];
  if(path==='/teachers/t1'&&request.method()==='PUT'){const body=request.postDataJSON();writes.push(body);teacher={...teacher,...body};data=teacher;}
  await route.fulfill({json:{code:0,data}});
 });
 await page.goto('/teachers');
 await expect(page.getByText('待绑定',{exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'编辑',exact:true}).click();
 const editor=page.getByRole('dialog',{name:'编辑教师',exact:true});
 await expect(editor.getByText('待绑定',{exact:true})).toBeVisible();
 await editor.getByLabel('启用账号',{exact:true}).click();
 await expect(editor.getByText('名下还有 3 节未结束的课程',{exact:true})).toBeVisible();
 await editor.getByRole('button',{name:/保\s*存/}).click();
 const confirm=page.getByRole('dialog',{name:'这个老师名下还有排课',exact:true});
 await expect(confirm).toBeVisible();expect(writes).toHaveLength(0);
 await confirm.getByRole('button',{name:/取\s*消/}).click();
 expect(writes).toHaveLength(0);await expect(editor).toBeVisible();
 await editor.getByRole('button',{name:/保\s*存/}).click();
 await page.getByRole('dialog',{name:'这个老师名下还有排课',exact:true}).getByRole('button',{name:'仍然停用',exact:true}).click();
 await expect(editor).toBeHidden();
 expect(writes).toHaveLength(1);expect(writes[0].accountStatus).toBe('停用');expect(writes[0].learningSpaceIds).toEqual(['e1']);
 expect(writes[0].teacherLibrary.canDownload).toBe(true);expect(writes[0].teacherLibrary.canManageCourses).toBe(false);
 expect(writes[0].canUploadHandout).toBe(false);expect(writes[0].canReview).toBe(false);
});
