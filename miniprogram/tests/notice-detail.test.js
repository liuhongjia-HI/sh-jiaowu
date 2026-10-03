const assert = require('node:assert/strict');
const test = require('node:test');
function load(request) {
 const saved = { starline_token: 'child-a', starline_student_id: 'a' };
 const requests = [];
 const requestPath = require.resolve('../utils/request');
 const badgePath = require.resolve('../utils/notice-badge');
 const pagePath = require.resolve('../pages/notice-detail/index');
 delete require.cache[pagePath];
 require.cache[requestPath] = { id: requestPath, filename: requestPath, loaded: true, exports: { request: (url, opts) => { requests.push(url); return request(url, opts); } } };
 require.cache[badgePath] = { id: badgePath, filename: badgePath, loaded: true, exports: { refreshNoticeBadge() {} } };
 global.wx = { setStorageSync: (key, value) => saved[key] = value, navigateTo() {} };
 let def; global.Page = (value) => def = value; require(pagePath);
 const page = { data: structuredClone(def.data), setData(patch) { Object.assign(this.data, patch); } };
 for (const [key, value] of Object.entries(def)) if (typeof value === 'function') page[key] = value.bind(page);
 page.noticeId = 'a'.repeat(64);
 return { page, saved, requests };
}
const detail = { canSwitch: true, event: { studentId: 'b', studentName: '孩子B', title: '调课成功', lessons: [{ after: { id: 'lesson', lessonDate: '2030-10-02', startTime: '10:00', endTime: '11:00' } }] }, currentLessons: [{ id: 'lesson', lessonDate: '2030-10-02', startTime: '12:00', endTime: '13:00', status: '已取消' }] };
test('message switches to the server-authorized child and reloads details', async () => {
 let reads = 0;
 const { page, saved, requests } = load((url) => {
  if (url.includes('/switch')) return Promise.resolve({ token: 'child-b', user: { studentId: 'b' } });
  if (url.includes('/read')) return Promise.resolve();
  return Promise.resolve({ ...detail, canSwitch: ++reads === 1 });
 });
 await page.loadDetail();
 assert.equal(saved.starline_token, 'child-b'); assert.equal(saved.starline_student_id, 'b');
 assert.equal(page.data.studentName, '孩子B'); assert.equal(reads, 2);
 assert.equal(page.data.lessons[0].status, 'Cancelled'); assert.equal(page.data.lessons[0].changed, true);
 assert.ok(requests.includes('/student/accounts/b/switch'));
});
test('failed switch clears stale child data and preserves the old token', async () => {
 const { page, saved } = load(url => url.includes('/switch') ? Promise.reject(new Error('关系已失效')) : Promise.resolve(detail));
 page.data.studentName = '孩子A'; page.data.detail = { stale: true };
 await page.loadDetail();
 assert.equal(page.data.detail, null); assert.equal(page.data.studentName, ''); assert.equal(saved.starline_token, 'child-a'); assert.equal(page.data.error, '关系已失效');
});
test('malformed ID never requests a child or changes identity', async () => {
 const { page, requests } = load(() => Promise.reject(new Error('unexpected'))); page.noticeId = '../other';
 await page.loadDetail(); assert.equal(requests.length, 0); assert.equal(page.data.detail, null);
});
test('unloaded page ignores a delayed detail result', async () => {
 let resolve; const { page } = load(() => new Promise(done => resolve = done));
 const pending = page.loadDetail(); page.onUnload(); resolve(detail); await pending;
 assert.equal(page.data.detail, null); assert.equal(page.data.studentName, '');
});

test('homework publication and final review open the specific learning record', () => {
 const {page} = load(() => Promise.resolve());
 const targets = []; wx.navigateTo = ({url}) => targets.push(url);
 page.data.detail = {event:{kind:'homework_published',relatedId:'hw B'}}; page.openRelated();
 page.data.detail = {event:{kind:'review_completed',relatedId:'sub B'}}; page.openRelated();
 assert.deepEqual(targets, ['/pages/answer/index?id=hw%20B','/pages/result/index?id=sub%20B']);
});

test('material message opens only current authorized files and reads its own station notice', async () => {
 const payload = { canSwitch: false, noticeId: 'notice-upload-owned', event: { studentId: 'a', studentName: '孩子A', title: '资料已更新', summary: '共1份资料', kind: 'materials_published' }, currentMaterials: [{ id: 'material one', title: '当前资料' }] };
 const { page, requests } = load(url => Promise.resolve(url.includes('/read') ? undefined : payload));
 const targets = []; wx.navigateTo = ({url}) => targets.push(url);
 await page.loadDetail();
 assert.deepEqual(page.data.materials, payload.currentMaterials);
 assert.ok(requests.includes('/student/notices/notice-upload-owned/read'));
 page.openMaterial({currentTarget:{dataset:{id:'material one'}}});
 page.openMaterial({currentTarget:{dataset:{id:'withdrawn'}}});
 assert.deepEqual(targets, ['/pages/material-preview/index?id=material%20one']);
});

test('failed material detail reload clears previously authorized files', async () => {
 const { page } = load(() => Promise.reject(new Error('资料权限已失效')));
 page.data.materials = [{id:'stale'}];
 await page.loadDetail();
 assert.deepEqual(page.data.materials, []);
 assert.equal(page.data.error, '资料权限已失效');
});
