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

test("study detail filters its ordered directory by the selected content tag", async () => {
  const page = loadStudyDetailPage(() => Promise.resolve({
    course: { name: "五年级英语S1Q1课程" },
    materials: [{ id: "mat-hd", tagCode: "HD" }],
    homework: [{ id: "hw-exam", tagCode: "Exam" }],
    stations: [
      { title: "第 1 站 讲义", materialId: "mat-hd", tagCode: "HD" },
      { title: "第 2 站 测试", homeworkId: "hw-exam", tagCode: "Exam" }
    ]
  }), { showToast() {} });
  page.courseId = "course-1";

  page.loadDetail();
  await flushPromises();
  assert.deepEqual(page.data.tags.map((tag) => tag.code), ["HD", "Blank", "HW", "Exam", "Special"]);
  assert.deepEqual(page.data.tags.map((tag) => tag.count), [1, 0, 0, 1, 0]);
  assert.equal(page.data.visibleStations.length, 2);

  page.selectTag({ currentTarget: { dataset: { code: "Exam" } } });
  assert.equal(page.data.activeTag, "Exam");
  assert.deepEqual(page.data.visibleStations.map((station) => station.homeworkId), ["hw-exam"]);
});

test("study detail shows all lectures without bulk controls", () => {
  const wxml = fs.readFileSync(path.join(__dirname, "../pages/study-detail/index.wxml"), "utf8");

  assert.doesNotMatch(wxml, /下载全部/);
  assert.doesNotMatch(wxml, /收起/);
  assert.match(wxml, /wx:for="\{\{materials\}\}"/);
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
