const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

function loadNoticesPage(requestImpl) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const pagePath = require.resolve("../pages/notices/index.js");
  delete require.cache[requestPath];
  delete require.cache[pagePath];
  require.cache[requestPath] = {
    id: requestPath,
    filename: requestPath,
    loaded: true,
    exports: { request: requestImpl }
  };
  global.wx = { switchTab() {} };
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

test("notice tabs switch active state and filter messages by category", async () => {
  const page = loadNoticesPage(() => Promise.resolve([
    { id: "course-1", type: "课", title: "课程调整提醒", relatedType: "schedule" },
    { id: "homework-1", type: "练", title: "英语阅读挑战已发布", relatedType: "homework" },
    { id: "system-1", type: "通知", title: "服务提醒" }
  ]));

  page.onLoad();
  await flushPromises();

  assert.equal(page.data.visibleNotices.length, 3);
  page.changeFilter({ currentTarget: { dataset: { filter: "作业" } } });
  assert.equal(page.data.activeFilter, "作业");
  assert.deepEqual(page.data.visibleNotices.map((item) => item.id), ["homework-1"]);
  assert.equal(page.data.filters.find((item) => item.label === "作业").className, "active");

  page.changeFilter({ currentTarget: { dataset: { filter: "课程" } } });
  assert.deepEqual(page.data.visibleNotices.map((item) => item.id), ["course-1"]);

  page.changeFilter({ currentTarget: { dataset: { filter: "系统" } } });
  assert.deepEqual(page.data.visibleNotices.map((item) => item.id), ["system-1"]);
});

test("notice tab keeps an empty filtered result when that category has no messages", async () => {
  const page = loadNoticesPage(() => Promise.resolve([
    { id: "course-1", type: "课", title: "课程调整提醒" }
  ]));

  page.onLoad();
  await flushPromises();
  page.changeFilter({ currentTarget: { dataset: { filter: "作业" } } });

  assert.equal(page.data.notices.length, 1);
  assert.deepEqual(page.data.visibleNotices, []);
});

test("notice cards identify the current student by name and grade", async () => {
  const page = loadNoticesPage((path) => {
    if (path === "/student/accounts") {
      return Promise.resolve([{ name: "小星", grade: "五年级", active: true }]);
    }
    return Promise.resolve([
      { id: "homework-1", type: "练", title: "英语阅读挑战已发布", relatedType: "homework" }
    ]);
  });

  page.onLoad();
  await flushPromises();

  assert.equal(page.data.visibleNotices[0].studentDisplay, "小星（五年级）");
});

test("notice cards navigate directly to their related learning detail", async () => {
  const navigations = [];
  const tabNavigations = [];
  const page = loadNoticesPage(() => Promise.resolve([
    { id: "homework-1", relatedType: "homework", relatedId: "hw-001" },
    { id: "review-1", relatedType: "review", relatedId: "sub-001" },
    { id: "course-1", relatedType: "course", relatedId: "course-001" },
    { id: "material-1", relatedType: "material", relatedId: "material-001" },
    { id: "schedule-1", relatedType: "schedule", relatedId: "schedule-001" }
  ]));
  global.wx.navigateTo = (value) => navigations.push(value);
  global.wx.switchTab = (value) => tabNavigations.push(value);

  page.onLoad();
  await flushPromises();

  page.goNotice({ currentTarget: { dataset: { id: "homework-1" } } });
  page.goNotice({ currentTarget: { dataset: { id: "review-1" } } });
  page.goNotice({ currentTarget: { dataset: { id: "course-1" } } });
  page.goNotice({ currentTarget: { dataset: { id: "material-1" } } });
  page.goNotice({ currentTarget: { dataset: { id: "schedule-1" } } });

  assert.deepEqual(navigations.map((item) => item.url), [
    "/pages/answer/index?id=hw-001",
    "/pages/result/index?id=sub-001",
    "/pages/study-detail/index?id=course-001",
    "/pages/material-preview/index?id=material-001"
  ]);
  assert.deepEqual(tabNavigations.map((item) => item.url), []);
  assert.equal(navigations[4].url, "/pages/schedule/index");
});

test("notice header lists every linked student by name and grade", async () => {
  const page = loadNoticesPage((path) => {
    if (path === "/student/accounts") {
      return Promise.resolve([
        { studentId: "student-1", name: "小星", grade: "五年级", active: true },
        { studentId: "student-2", name: "小月", grade: "三年级", active: false }
      ]);
    }
    return Promise.resolve([]);
  });

  page.onLoad();
  await flushPromises();

  assert.equal(page.data.linkedStudentText, "小星（五年级）、小月（三年级）");
});

test("notice summary wraps so the complete message remains visible", () => {
  const styles = fs.readFileSync(path.join(__dirname, "../pages/notices/index.wxss"), "utf8");
  const summaryRules = [...styles.matchAll(/\.notice-summary\s*\{([^}]*)\}/g)].map((match) => match[1]).join("\n");

  assert.match(summaryRules, /white-space:\s*normal/);
  assert.match(summaryRules, /overflow-wrap:\s*anywhere/);
});

test("linked student names can wrap without hiding the switch action", () => {
  const styles = fs.readFileSync(path.join(__dirname, "../pages/notices/index.wxss"), "utf8");
  const contextMainRules = [...styles.matchAll(/\.notice-student-context-main\s*\{([^}]*)\}/g)].map((match) => match[1]).join("\n");
  const nameRules = [...styles.matchAll(/\.notice-student-name\s*\{([^}]*)\}/g)].map((match) => match[1]).join("\n");

  assert.match(contextMainRules, /min-width:\s*0/);
  assert.match(nameRules, /overflow-wrap:\s*anywhere/);
});
