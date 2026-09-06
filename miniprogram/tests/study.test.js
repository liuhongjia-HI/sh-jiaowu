const assert = require("node:assert/strict");
const test = require("node:test");
const fs = require("node:fs");
const path = require("node:path");

function loadStudyPage(requestImpl) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const pagePath = require.resolve("../pages/study/index.js");
  delete require.cache[requestPath];
  delete require.cache[pagePath];
  require.cache[requestPath] = {
    id: requestPath,
    filename: requestPath,
    loaded: true,
    exports: { request: requestImpl }
  };
  global.wx = {
    showToast() {},
    navigateTo() {}
  };
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

test("课程卡片整体可点击，学习入口只负责展示状态", () => {
  const template = fs.readFileSync(path.join(__dirname, "../pages/study/index.wxml"), "utf8");

  assert.match(template, /wx:if="{{item\.canOpen}}" class="course-card-hit-area" bindtap="goDetail" data-id="{{item\.entryCourseId}}"/);
  assert.match(template, /wx:else class="course-card-hit-area">\s*<template is="course-card"/);
  assert.match(template, /wx:if="{{item\.canOpen}}" class="course-action">/);
  assert.match(template, /wx:else class="course-action course-action-disabled">{{item\.accessLabel \|\| '内容准备中'}}<\/view>/);
  assert.match(template, /item\.accessLabel === '首节可体验' \? '体验' : '进入学习'/);
  assert.doesNotMatch(template, /体验第一节/);
});

test("内容准备中的课程分支不绑定课程点击事件", () => {
  const template = fs.readFileSync(path.join(__dirname, "../pages/study/index.wxml"), "utf8");

  assert.match(template, /<view wx:else class="course-card-hit-area">\s*<template is="course-card"/);
  assert.doesNotMatch(template, /wx:else class="course-card-hit-area"[^>]*bindtap=/);
});

test("study page refreshes opened courses when tab is shown again", async () => {
  const calls = [];
  const page = loadStudyPage((path) => {
    calls.push(path);
    if (path === "/student/favorites") {
      return Promise.resolve([]);
    }
    return Promise.resolve({
      student: { id: "stu-001", openedPackages: ["四年级地理"] },
      courses: [{ id: "course-g04-geo-s1-q1", name: "四年级地理S1Q1课程", subject: "地理", grade: "四年级" }],
      materials: []
    });
  });

  page.data.loading = false;
  page.onShow();
  await flushPromises();

  assert.deepEqual(calls, ["/student/study", "/student/favorites"]);
  assert.equal(page.data.visibleCourses.length, 1);
  assert.equal(page.data.visibleCourses[0].name, "四年级地理S1Q1课程");
});

test("未登录点击学习导航只提示一次，取消登录后可以停留在学习页", () => {
  const navigations = [];
  const page = loadStudyPage(() => Promise.resolve({}));
  global.wx.getStorageSync = () => "";
  global.wx.navigateTo = (options) => navigations.push(options.url);

  page.onLoad();
  page.onShow();

  assert.deepEqual(navigations, ["/pages/login/index"]);
  assert.equal(page.data.authRequired, true);
  assert.equal(page.data.loginPrompted, true);
});

test("study page puts a newly opened course first and keeps its new marker", async () => {
  const page = loadStudyPage((path) => {
    if (path === "/student/favorites") return Promise.resolve([]);
    return Promise.resolve({
      student: { id: "stu-001", openedPackages: ["五年级课程"] },
      courses: [
        { id: "course-old", name: "已开通课程", subject: "英语", grade: "五年级", availableAt: "2026-08-30 09:00:00" },
        { id: "course-new", name: "刚开通课程", subject: "数学", grade: "五年级", availableAt: "2026-08-30 10:00:00", isNew: true }
      ],
      materials: []
    });
  });

  page.loadStudy();
  await flushPromises();

  assert.equal(page.data.visibleCourses[0].id, "course-new");
  assert.equal(page.data.visibleCourses[0].isNew, true);
  assert.equal(page.data.visibleCourses[0].cardClass, "new-course");
});

test("study page shows the grade subject catalog and blocks unopened subjects", async () => {
  const toasts = [];
  const navigations = [];
  const page = loadStudyPage((path) => {
    if (path === "/student/favorites") return Promise.resolve([]);
    return Promise.resolve({
      student: { id: "stu-001", grade: "五年级", openedPackages: [] },
      subjects: [
        { id: "g5-geography", displayName: "地理", subject: "地理", grade: "五年级", accessState: "preview", accessLabel: "首节可体验", canOpen: true, entryCourseId: "course-g05-geography-s1-q1", materialNum: 1 },
        { id: "g5-chinese", displayName: "语文", subject: "语文", grade: "五年级", accessState: "locked", accessLabel: "暂未开通", canOpen: false }
      ],
      courses: [], materials: []
    });
  });
  global.wx.showToast = (value) => toasts.push(value);
  global.wx.navigateTo = (value) => navigations.push(value);

  page.loadStudy();
  await flushPromises();
  assert.equal(page.data.visibleCourses.length, 2);
  assert.equal(page.data.visibleCourses[0].displayName, "地理");
  assert.equal(page.data.visibleCourses[0].accessLabel, "首节可体验");
  page.goDetail({ currentTarget: { dataset: { id: "", canOpen: false } } });
  assert.equal(toasts[0].title, "开通后即可学习全部内容");
  page.goDetail({ currentTarget: { dataset: { id: "course-g05-geography-s1-q1", canOpen: true } } });
  assert.equal(navigations[0].url, "/pages/study-detail/index?id=course-g05-geography-s1-q1");
});

test("点击可学习课程卡片时，即使事件未携带权限字段也能进入课程", async () => {
  const navigations = [];
  const page = loadStudyPage((path) => {
    if (path === "/student/favorites") return Promise.resolve([]);
    return Promise.resolve({
      subjects: [
        { id: "g5-math", displayName: "数学", subject: "数学", grade: "五年级", accessState: "full", canOpen: true, entryCourseId: "course-math-first" }
      ],
      courses: [], materials: []
    });
  });
  global.wx.navigateTo = (value) => navigations.push(value);

  page.loadStudy();
  await flushPromises();
  page.goDetail({ currentTarget: { dataset: { id: "course-math-first" } } });

  assert.deepEqual(navigations, [{ url: "/pages/study-detail/index?id=course-math-first" }]);
});

test("内容准备中的课程即使存在课程编号也不可进入", async () => {
  const page = loadStudyPage((path) => {
    if (path === "/student/favorites") return Promise.resolve([]);
    return Promise.resolve({
      student: { id: "stu-001", grade: "五年级", openedPackages: ["五年级语文"] },
      subjects: [
        { id: "g5-chinese", entryCourseId: "course-chinese", displayName: "语文", subject: "语文", grade: "五年级", accessState: "pending", accessLabel: "内容准备中", canOpen: false }
      ],
      courses: [], materials: []
    });
  });

  page.loadStudy();
  await flushPromises();

  assert.equal(page.data.visibleCourses[0].canOpen, false);
});
