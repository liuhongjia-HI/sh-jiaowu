const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

function loadMaterialReaderPage(requestImpl, wxMock) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const pagePath = require.resolve("../pages/material-reader/index.js");
  delete require.cache[requestPath];
  delete require.cache[pagePath];
  require.cache[requestPath] = {
    id: requestPath,
    filename: requestPath,
    loaded: true,
    exports: { request: requestImpl }
  };
  global.wx = wxMock;
  global.getApp = () => ({ globalData: { apiBaseUrl: "https://gate.example.com/api" } });
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

test("material reader loads pages in small batches and continues at the bottom", async () => {
  const downloadedUrls = [];
  const wxMock = {
    getStorageSync: () => "token-abc",
    setNavigationBarTitle() {},
    downloadFile(opts) {
      downloadedUrls.push(opts.url);
      opts.success({ statusCode: 200, tempFilePath: `${opts.url}#local` });
    }
  };
  const requestImpl = (requestPath) => {
    if (requestPath === "/student/materials/mat-1/preview/pages") {
      return Promise.resolve({ previewStatus: "ready", imageMode: true, pageCount: 5 });
    }
    return Promise.reject(new Error("unexpected path " + requestPath));
  };
  const page = loadMaterialReaderPage(requestImpl, wxMock);

  page.onLoad({ id: "mat-1", title: "%E7%AC%AC%E4%B8%80%E8%AF%BE" });
  await flushPromises();
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.mode, "ready");
  assert.equal(page.data.pageCount, 5);
  assert.deepEqual(page.data.pages.map((item) => item.page), [1, 2, 3]);

  page.onReachBottom();
  await flushPromises();
  await flushPromises();

  assert.deepEqual(page.data.pages.map((item) => item.page), [1, 2, 3, 4, 5]);
  assert.equal(page.data.hasMore, false);
  assert.deepEqual(downloadedUrls, [1, 2, 3, 4, 5].map(
    (pageNumber) => `https://gate.example.com/api/student/materials/mat-1/preview/pages/${pageNumber}`
  ));
});

test("material reader template exposes loading, error, and reading states", () => {
  const template = fs.readFileSync(path.join(__dirname, "../pages/material-reader/index.wxml"), "utf8");
  assert.match(template, /mode === 'loading'/);
  assert.match(template, /mode === 'error'/);
  assert.match(template, /wx:for="{{pages}}"/);
  assert.match(template, /bindtap="retry"/);
});
