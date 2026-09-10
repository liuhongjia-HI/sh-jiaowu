const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

function loadStudyDetailPage(requestImpl, wxMock = {}) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const pagePath = require.resolve("../pages/study-detail/index.js");
  delete require.cache[requestPath];
  delete require.cache[pagePath];
  require.cache[requestPath] = {
    id: requestPath,
    filename: requestPath,
    loaded: true,
    exports: { request: requestImpl }
  };
  global.wx = wxMock;
  global.Page = (definition) => pages.push(definition);
  require(pagePath);
  const definition = pages[0];
  const page = {
    data: JSON.parse(JSON.stringify(definition.data)),
    setData(patch, callback) {
      Object.assign(this.data, patch);
      callback && callback();
    }
  };
  Object.keys(definition).forEach((key) => {
    if (key !== "data") {
      page[key] = typeof definition[key] === "function" ? definition[key].bind(page) : definition[key];
    }
  });
  return page;
}

function flushPromises() {
  return new Promise((resolve) => setImmediate(resolve));
}

test("study detail shares the current course id and title", () => {
  const page = loadStudyDetailPage(() => Promise.resolve({}), {
    showToast() {}
  });
  page.courseId = "course-g05-english-s1-q1";
  page.setData({ course: { name: "五年级英语S1Q1课程" } });

  assert.deepEqual(page.onShareAppMessage(), {
    title: "五年级英语S1Q1课程",
    path: "/pages/study-detail/index?id=course-g05-english-s1-q1"
  });
});

test("study detail top-right affordance is a native share button", () => {
  const wxml = fs.readFileSync(path.join(__dirname, "../pages/study-detail/index.wxml"), "utf8");

  assert.match(wxml, /<button class="detail-share" open-type="share"/);
});

test("study detail flattens all Lesson leaves in curriculum order", async () => {
  const page = loadStudyDetailPage(() => Promise.resolve({
    course: { name: "五年级英语S1Q1课程", curriculum: [
      { id: "unit-2", type: "unit", name: "Unit 2", sortOrder: 2 },
      { id: "unit-1", type: "unit", name: "Unit 1", sortOrder: 1 },
      { id: "chapter-1", parentId: "unit-1", type: "chapter", name: "Chapter 1", sortOrder: 1 },
      { id: "chapter-2", parentId: "unit-2", type: "chapter", name: "Chapter 2", sortOrder: 1 },
      { id: "lesson-3", parentId: "chapter-2", type: "lesson", name: "第三节", sortOrder: 1 },
      { id: "lesson-2", parentId: "chapter-1", type: "lesson", name: "第二节", sortOrder: 2 },
      { id: "lesson-1", parentId: "chapter-1", type: "lesson", name: "第一节", sortOrder: 1 }
    ] },
    materials: [{ id: "mat-hd", lessonId: "lesson-1", tagCode: "HD" }],
    homework: [{ id: "hw-exam", lessonId: "lesson-1", tagCode: "Exam" }],
    stations: [
      { lessonId: "lesson-1", status: "学习中", materialId: "mat-hd", tagCode: "HD" }
    ]
  }), { showToast() {} });
  page.courseId = "course-1";

  page.loadDetail();
  await flushPromises();
  assert.equal(page.data.lessonCount, 3);
  assert.deepEqual(page.data.catalogLessons.map((item) => item.displayName), ["1.1.1 · Chapter 1 · 第一节", "1.1.2 · Chapter 1 · 第二节", "2.1.1 · Chapter 2 · 第三节"]);
  assert.deepEqual(page.data.catalogLessons[0], {
    id: "lesson-1", parentId: "chapter-1", type: "lesson", name: "第一节", sortOrder: 1,
    displayName: "1.1.1 · Chapter 1 · 第一节",
    icon: "📖", status: "学习中", statusClass: "is-active", desc: "2 项学习内容", materialId: "mat-hd", homeworkId: "hw-exam"
  });
});

test("study detail flattens Chapter and Unit leaves when a curriculum has fewer levels", async () => {
  const page = loadStudyDetailPage(() => Promise.resolve({
    course: { curriculum: [
      { id: "unit-2", type: "unit", name: "第二单元", sortOrder: 2 },
      { id: "unit-1", type: "unit", name: "第一单元", sortOrder: 1 },
      { id: "chapter-1", parentId: "unit-1", type: "chapter", name: "第一章", sortOrder: 1 }
    ] },
    stations: [
      { lessonId: "chapter-1", status: "学习中" },
      { lessonId: "unit-2", status: "待挑战" }
    ]
  }), { showToast() {} });
  page.courseId = "course-2";

  page.loadDetail();
  await flushPromises();
  assert.deepEqual(page.data.catalogLessons.map((item) => item.displayName), ["1.1 · 第一单元 · 第一章", "2 · 第二单元"]);
  assert.deepEqual(page.data.catalogLessons.map((item) => item.id), ["chapter-1", "unit-2"]);
});

test("study detail renders one flat leaf list without Unit or Chapter groups", () => {
  const wxml = fs.readFileSync(path.join(__dirname, "../pages/study-detail/index.wxml"), "utf8");

  assert.doesNotMatch(wxml, /下载全部/);
  assert.doesNotMatch(wxml, /收起/);
  assert.match(wxml, /wx:for="\{\{catalogLessons\}\}"/);
  assert.doesNotMatch(wxml, /catalog-unit|catalog-chapter/);
  assert.doesNotMatch(wxml, /wx:for="\{\{materials\}\}"/);
});

test("study detail renders every lecture returned by the API", async () => {
  const materials = [
    { id: "mat-1", title: "第一份讲义" },
    { id: "mat-2", title: "第二份讲义" },
    { id: "mat-3", title: "第三份讲义" }
  ];
  const page = loadStudyDetailPage(() => Promise.resolve({ materials }), { showToast() {} });
  page.courseId = "course-1";

  page.loadDetail();
  await flushPromises();

  assert.deepEqual(page.data.materials.map((item) => item.id), ["mat-1", "mat-2", "mat-3"]);
});

test("homework-only lesson opens the tagged lesson content page", async () => {
  const navigations = [];
  const page = loadStudyDetailPage(() => Promise.resolve({}), { navigateTo: ({ url }) => navigations.push(url) });
  page.courseId = "course-1";

  page.tapLesson({ currentTarget: { dataset: { status: "待挑战", lessonId: "lesson-1", homeworkId: "homework-1" } } });

  assert.deepEqual(navigations, ["/pages/material-preview/index?courseId=course-1&lessonId=lesson-1"]);
});

test("未开通课节保留锁且不跳转，授权刷新后可进入带课节上下文的预览页", async () => {
  const navigations = [];
  let full = false;
  const page = loadStudyDetailPage(() => Promise.resolve({
    course: { curriculum: [
      { id: "unit", type: "unit", name: "Unit", sortOrder: 1 },
      { id: "chapter", parentId: "unit", type: "chapter", name: "Chapter", sortOrder: 1 },
      { id: "lesson-1", parentId: "chapter", type: "lesson", name: "第一节", sortOrder: 1 },
      { id: "lesson-2", parentId: "chapter", type: "lesson", name: "第二节", sortOrder: 2 }
    ] },
    materials: [{ id: "first", lessonId: "lesson-1", tagCode: "HD" }].concat(full ? [{ id: "later", lessonId: "lesson-2", tagCode: "HD" }] : []),
    stations: [
      { lessonId: "lesson-1", status: "学习中", materialId: "first", tagCode: "HD" },
      full ? { lessonId: "lesson-2", status: "待挑战", materialId: "later", tagCode: "HD" }
        : { lessonId: "lesson-2", status: "未开通", icon: "🔒" }
    ]
  }), { navigateTo: (value) => navigations.push(value.url) });
  page.courseId = "course";
  page.loadDetail();
  await flushPromises();
  assert.equal(page.data.catalogLessons[1].status, "未开通");
  page.tapLesson({ currentTarget: { dataset: { status: "未开通", lessonId: "lesson-2", materialId: "later" } } });
  assert.equal(navigations.length, 0);
  page.tapLesson({ currentTarget: { dataset: { status: "学习中", lessonId: "lesson-1", materialId: "first" } } });
  assert.equal(navigations[0], "/pages/material-preview/index?id=first&courseId=course&lessonId=lesson-1");
  full = true;
  page.onShow();
  await flushPromises();
  assert.equal(page.data.catalogLessons[1].materialId, "later");
  page.tapLesson({ currentTarget: { dataset: { status: "待挑战", lessonId: "lesson-2", materialId: "later" } } });
  assert.equal(navigations[1], "/pages/material-preview/index?id=later&courseId=course&lessonId=lesson-2");
  full = false;
  page.onShow();
  await flushPromises();
  assert.equal(page.data.catalogLessons[1].icon, "🔒");
});
