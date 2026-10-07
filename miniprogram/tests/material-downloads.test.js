const assert = require('node:assert/strict');
const test = require('node:test');
function pageFixture(request, wxMock = {}) {
  const requestPath = require.resolve('../utils/request'); const pagePath = require.resolve('../pages/material-downloads/index');
  delete require.cache[pagePath]; require.cache[requestPath] = { id: requestPath, filename: requestPath, loaded: true, exports: { request } };
  let definition; global.Page = value => { definition = value; };
  global.wx = { showToast() {}, showModal() {}, ...wxMock };
  global.getApp = () => ({ globalData: { webBaseUrl: 'https://school.example' } });
  require(pagePath);
  const page = { data: JSON.parse(JSON.stringify(definition.data)), setData(value) { Object.assign(this.data, value); } };
  Object.entries(definition).forEach(([key, value]) => { if (key !== 'data') page[key] = typeof value === 'function' ? value.bind(page) : value; });
  return page;
}
const quote = { studentName: 'Student A', courses: ['G5 English'], materials: [{ id: 'a', fileName: 'A.pdf', size: 1024, unit: 'Unit 1', chapter: 'Chapter 1', lesson: 'Lesson 1' }, { id: 'b', fileName: 'B.pdf', size: 2048, unit: 'Unit 2', chapter: 'Chapter 1', lesson: 'Lesson 2' }] };
const job = { id: 'download-job', studentName: 'Student A', scope: { subject: 'English' }, status: '可下载', count: 2, size: 3072, materials: quote.materials };
test('course selection defaults to all allowed files and submits only checked files', async () => {
 const writes = []; const page = pageFixture((path, options) => {
  if (path === '/student/home') return Promise.resolve({ student: { id: 'student-a', name: 'Student A' } });
  if (path.endsWith('/selection')) return Promise.resolve(quote);
  if (options?.method === 'POST') { writes.push(options.data); return Promise.resolve(job); }
  return Promise.resolve([job]);
 });
 page.onLoad({ courseId: 'course-a' }); await page.load();
 assert.equal(page.data.selectedCount, 2); assert.equal(page.data.studentName, 'Student A');
 page.toggleGroup({ currentTarget: { dataset: { index: 1 } } });
 assert.equal(page.data.selectedCount, 1); await page.generate();
 assert.deepEqual(writes, [{ courseIds: ['course-a'], materialIds: ['a'] }]);
 page.toggleChapter({ currentTarget: { dataset: { groupIndex: 1, chapterIndex: 0 } } }); assert.equal(page.data.selectedCount, 2);
 page.toggleAll(); assert.equal(page.data.selectedCount, 0); page.toggleAll(); assert.equal(page.data.selectedCount, 2); page.toggleAll(); assert.equal(page.data.selectedCount, 0);
 await page.generate(); assert.equal(writes.length, 1); page.onUnload();
});
test('computer link carries only job ID and QR confirmation needs an explicit tap', async () => {
 let clipboard = '', target = '', confirms = 0;
 const challenge = 'a'.repeat(48);
 const page = pageFixture((path, options) => {
  if (options?.method === 'POST') confirms++;
  return Promise.resolve(job);
 }, { setClipboardData({ data }) { clipboard = data; }, scanCode({ success }) { success({ result: `starline-download:${challenge}` }); }, navigateTo({ url }) { target = url; } });
 page.onLoad({}); page.data.jobs = [{ ...job, ready: true }]; page.copyLink({ currentTarget: { dataset: { id: job.id } } });
 assert.equal(clipboard, 'https://school.example/student-download?job=download-job');
 assert.equal(clipboard.includes('token'), false); page.scanPickup(); assert.ok(target.endsWith(`challenge=${challenge}`));
 page.onLoad({ challenge }); await page.load(); assert.equal(confirms, 0); assert.ok(page.data.pickup);
 await page.confirmPickup(); assert.equal(confirms, 1); assert.equal(page.data.confirmed, true);
 await page.confirmPickup(); assert.equal(confirms, 1); page.onUnload();
});
test('a failed QR lookup clears old confirmation data and cannot authorize', async () => {
 let count = 0;
 const page = pageFixture(() => ++count === 1 ? Promise.resolve(job) : Promise.reject(new Error('Expired')));
 page.onLoad({ challenge: 'a'.repeat(48) }); await page.load(); assert.ok(page.data.pickup);
 await page.load(); assert.equal(page.data.pickup, null); assert.equal(page.data.error, 'Expired');
 await page.confirmPickup(); assert.equal(count, 2); page.onUnload();
});
test('pending task polling stops when page is hidden', async () => {
 const page = pageFixture(path => Promise.resolve(path === '/student/home' ? { student: { id: 'a', name: 'A' } } : [{ ...job, status: '准备中' }]));
 page.onLoad({}); await page.load(); assert.ok(page.pollTimer); page.onHide(); assert.equal(page.showing, false); page.onUnload();
});
test('switching the active student clears selections and loads the new student permission list', async () => {
 let studentId = 'student-a', selectionReads = 0;
 const page = pageFixture(path => {
  if (path === '/student/home') return Promise.resolve({ student: { id: studentId, name: studentId } });
  if (path.endsWith('/selection')) { selectionReads++; return Promise.resolve({ studentName: studentId, courses: ['Course'], materials: studentId === 'student-a' ? quote.materials : [quote.materials[1]] }); }
  return Promise.resolve([]);
 });
 page.onLoad({ courseId: 'course-a' }); await page.load(); assert.equal(page.data.selectedCount, 2);
 studentId = 'student-b'; await page.load(); assert.equal(selectionReads, 2); assert.equal(page.data.selectedCount, 1);
 assert.equal(page.data.studentName, 'student-b'); assert.deepEqual([...page.selectedIDs], ['b']); page.onUnload();
});
test('file list expansion survives task polling', async () => {
 const page = pageFixture(() => Promise.resolve([job]));
 page.onLoad({}); page.data.jobs = [job]; page.showFiles({ currentTarget: { dataset: { id: job.id } } });
 await page.refreshJobs(); assert.equal(page.data.jobs[0].expanded, true); page.onUnload();
});
