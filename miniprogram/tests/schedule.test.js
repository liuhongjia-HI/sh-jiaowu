const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

function loadSchedulePage(requestImpl, wxMock = {}) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const pagePath = require.resolve("../pages/schedule/index.js");
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

test("schedule page loads confirmed classes from student schedule API", async () => {
  const calls = [];
  const page = loadSchedulePage((url) => {
    calls.push(url);
    if (url === "/student/availability") {
      return Promise.resolve([{ dayOfWeek: 6, startTime: "09:00", endTime: "11:00" }]);
    }
    if (url === "/student/schedule") {
      return Promise.resolve([
        {
          id: "schedule-closed-loop",
          name: "英语 1V3 小班",
          courseName: "五年级英语S1Q1课程",
          teacherName: "英语老师",
          roomName: "验收教室A",
          dayOfWeek: 3,
          startTime: "19:00",
          endTime: "20:30",
          startDate: "2026-06-01",
          endDate: "2026-08-31",
          status: "已确认"
        }
      ]);
    }
    return Promise.reject(new Error(`unexpected request ${url}`));
  });

  page.onLoad();
  await flushPromises();

  assert.deepEqual(calls, ["/student/availability", "/student/schedule"]);
  assert.equal(page.data.loading, false);
  assert.equal(page.data.availability[0].weekLabel, "周六");
  assert.equal(page.data.nextClass.id, "schedule-closed-loop");
  assert.equal(page.data.nextClass.timeText, "周三 19:00-20:30");
  assert.equal(page.data.nextClass.periodText, "2026-06-01 至 2026-08-31");
  assert.equal(page.data.nextClass.statusText, "已确认");
  assert.equal(page.data.classes.length, 1);
  assert.equal(page.data.classes[0].name, "英语 1V3 小班");
  assert.equal(page.data.classes[0].weekLabel, "周三");
  assert.equal(page.data.classes[0].startTime, "19:00");
  assert.equal(page.data.classes[0].endTime, "20:30");
  assert.equal(page.data.classes[0].teacherName, "英语老师");
});

test("schedule page keeps confirmed schedule empty state data when there are no classes", async () => {
  const page = loadSchedulePage((url) => {
    if (url === "/student/availability") {
      return Promise.resolve([]);
    }
    if (url === "/student/schedule") {
      return Promise.resolve([]);
    }
    return Promise.reject(new Error(`unexpected request ${url}`));
  });

  page.onLoad();
  await flushPromises();

  assert.equal(page.data.loading, false);
  assert.equal(page.data.nextClass, null);
  assert.deepEqual(page.data.classes, []);
  assert.deepEqual(page.data.availability, []);
});

test("schedule page template does not expose offline classroom fields", () => {
  const template = fs.readFileSync(path.join(__dirname, "../pages/schedule/index.wxml"), "utf8");

  assert.equal(template.includes("roomName"), false);
  assert.equal(template.includes("教室"), false);
  assert.equal(template.includes("地点"), false);
  assert.equal(template.includes("进教室"), false);
});
