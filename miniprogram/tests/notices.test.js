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

test("notice summary wraps so the complete message remains visible", () => {
  const styles = fs.readFileSync(path.join(__dirname, "../pages/notices/index.wxss"), "utf8");
  const summaryRules = [...styles.matchAll(/\.notice-summary\s*\{([^}]*)\}/g)].map((match) => match[1]).join("\n");

  assert.match(summaryRules, /white-space:\s*normal/);
  assert.match(summaryRules, /overflow-wrap:\s*anywhere/);
});
