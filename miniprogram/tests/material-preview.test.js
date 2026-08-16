const assert = require("node:assert/strict");
const test = require("node:test");

function loadMaterialPreviewPage(requestImpl, wxMock) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const securityPath = require.resolve("../utils/content-security");
  const pagePath = require.resolve("../pages/material-preview/index.js");
  delete require.cache[requestPath];
  delete require.cache[securityPath];
  delete require.cache[pagePath];
  require.cache[requestPath] = {
    id: requestPath,
    filename: requestPath,
    loaded: true,
    exports: { request: requestImpl }
  };
  global.wx = wxMock;
  global.getCurrentPages = () => [{ route: "pages/material-preview/index" }];
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

function baseWxMock(overrides = {}) {
  return {
    showToast() {},
    showLoading() {},
    hideLoading() {},
    setVisualEffectOnCapture(opts) {
      opts.success && opts.success();
    },
    onUserCaptureScreen() {},
    offUserCaptureScreen() {},
    getStorageSync: () => "token-abc",
    ...overrides
  };
}

test("material preview switches to paged image mode and downloads pages in order", async () => {
  const downloadedUrls = [];
  const wxMock = baseWxMock({
    downloadFile(opts) {
      downloadedUrls.push(opts.url);
      opts.success({ statusCode: 200, tempFilePath: `${opts.url}#local` });
    }
  });
  const requestImpl = (path) => {
    if (path === "/student/materials/mat-1") {
      return Promise.resolve({ id: "mat-1", title: "第一课", watermarkText: "张三 · 1234" });
    }
    if (path === "/student/materials/mat-1/preview/pages") {
      return Promise.resolve({ imageMode: true, pageCount: 2 });
    }
    if (path === "/student/favorites") {
      return Promise.resolve([]);
    }
    return Promise.reject(new Error("unexpected path " + path));
  };
  const page = loadMaterialPreviewPage(requestImpl, wxMock);

  page.onLoad({ id: "mat-1" });
  await flushPromises();
  await flushPromises();
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.previewMode, "image");
  assert.equal(page.data.pageCount, 2);
  assert.deepEqual(page.data.pageImages.map((item) => item.index), [1, 2]);
  assert.equal(page.data.pagesLoading, false);
  assert.deepEqual(downloadedUrls, [
    "https://gate.example.com/api/student/materials/mat-1/preview/pages/1",
    "https://gate.example.com/api/student/materials/mat-1/preview/pages/2"
  ]);
});

test("material preview falls back to pdf mode when the server has no image mode", async () => {
  const wxMock = baseWxMock();
  const requestImpl = (path) => {
    if (path === "/student/materials/mat-1") {
      return Promise.resolve({ id: "mat-1", title: "第一课" });
    }
    if (path === "/student/materials/mat-1/preview/pages") {
      return Promise.resolve({ imageMode: false, pageCount: 0 });
    }
    if (path === "/student/favorites") {
      return Promise.resolve([]);
    }
    return Promise.reject(new Error("unexpected path " + path));
  };
  const page = loadMaterialPreviewPage(requestImpl, wxMock);

  page.onLoad({ id: "mat-1" });
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.previewMode, "pdf");
  assert.equal(page.data.pageImages.length, 0);
});

test("material preview keeps loaded pages and marks failed pages for retry", async () => {
  let call = 0;
  const wxMock = baseWxMock({
    downloadFile(opts) {
      call += 1;
      if (call === 1) {
        opts.success({ statusCode: 200, tempFilePath: "page-1#local" });
        return;
      }
      opts.fail(new Error("network drop"));
    }
  });
  const requestImpl = (path) => {
    if (path === "/student/materials/mat-1") {
      return Promise.resolve({ id: "mat-1", title: "第一课" });
    }
    if (path === "/student/materials/mat-1/preview/pages") {
      return Promise.resolve({ imageMode: true, pageCount: 3 });
    }
    if (path === "/student/favorites") {
      return Promise.resolve([]);
    }
    return Promise.reject(new Error("unexpected path " + path));
  };
  const page = loadMaterialPreviewPage(requestImpl, wxMock);

  page.onLoad({ id: "mat-1" });
  await flushPromises();
  await flushPromises();
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.previewMode, "image");
  assert.equal(page.data.pagesLoading, false);
  assert.deepEqual(page.data.pageImages.map((item) => item.status), ["ready", "error", "error"]);
  assert.equal(page.data.pageImages[0].path, "page-1#local");
});

test("openSecurePreview strips the redundant /api prefix from previewUrl before downloading", async () => {
  const downloadedUrls = [];
  let openedPath = "";
  const wxMock = baseWxMock({
    downloadFile(opts) {
      downloadedUrls.push(opts.url);
      opts.success({ statusCode: 200, tempFilePath: "secure-preview.pdf#local" });
    },
    openDocument(opts) {
      openedPath = opts.filePath;
    }
  });
  const requestImpl = (path) => {
    if (path === "/student/materials/mat-1") {
      return Promise.resolve({ id: "mat-1", title: "第一课", previewUrl: "/api/student/materials/mat-1/preview" });
    }
    if (path === "/student/materials/mat-1/preview/pages") {
      return Promise.resolve({ imageMode: false, pageCount: 0 });
    }
    if (path === "/student/favorites") {
      return Promise.resolve([]);
    }
    return Promise.reject(new Error("unexpected path " + path));
  };
  const page = loadMaterialPreviewPage(requestImpl, wxMock);

  page.onLoad({ id: "mat-1" });
  await flushPromises();
  await flushPromises();
  page.openSecurePreview();
  await flushPromises();

  assert.deepEqual(downloadedUrls, ["https://gate.example.com/api/student/materials/mat-1/preview"]);
  assert.equal(openedPath, "secure-preview.pdf#local");
});

test("openSecurePreview shows the backend reason when a historical preview file is missing", async () => {
  let modal = null;
  const wxMock = baseWxMock({
    downloadFile(opts) {
      opts.success({ statusCode: 400, tempFilePath: "error-response.json" });
    },
    getFileSystemManager() {
      return {
        readFile(opts) {
          opts.success({ data: JSON.stringify({ code: 400, message: "历史课件文件不可用，请联系老师重新上传" }) });
        }
      };
    },
    showModal(opts) { modal = opts; }
  });
  const requestImpl = (path) => {
    if (path === "/student/materials/mat-1") {
      return Promise.resolve({ id: "mat-1", title: "历史课件", previewUrl: "/api/student/materials/mat-1/preview" });
    }
    if (path === "/student/materials/mat-1/preview/pages") {
      return Promise.resolve({ imageMode: false, pageCount: 0 });
    }
    if (path === "/student/favorites") return Promise.resolve([]);
    return Promise.reject(new Error("unexpected path " + path));
  };
  const page = loadMaterialPreviewPage(requestImpl, wxMock);

  page.onLoad({ id: "mat-1" });
  await flushPromises();
  await flushPromises();
  page.openSecurePreview();
  await flushPromises();
  await flushPromises();

  assert.equal(modal.title, "课件无法打开");
  assert.equal(modal.content, "历史课件文件不可用，请联系老师重新上传");
});

test("material preview displays page metadata errors instead of a fake PDF placeholder", async () => {
  const requestImpl = (path) => {
    if (path === "/student/materials/mat-1") return Promise.resolve({ id: "mat-1", title: "历史课件" });
    if (path === "/student/materials/mat-1/preview/pages") return Promise.reject(new Error("历史课件分页文件不可用，请联系老师重新上传"));
    if (path === "/student/favorites") return Promise.resolve([]);
    return Promise.reject(new Error("unexpected path " + path));
  };
  const page = loadMaterialPreviewPage(requestImpl, baseWxMock());

  page.onLoad({ id: "mat-1" });
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.previewMode, "unavailable");
  assert.equal(page.data.previewMessage, "历史课件分页文件不可用，请联系老师重新上传");
});

test("recording change shows a warning without hiding the page content", async () => {
  let recordingHandler = null;
  const wxMock = baseWxMock({
    onScreenRecordingStateChanged(handler) {
      recordingHandler = handler;
    },
    offScreenRecordingStateChanged() {}
  });
  const requestImpl = (path) => {
    if (path === "/student/materials/mat-1") {
      return Promise.resolve({ id: "mat-1", title: "第一课" });
    }
    if (path === "/student/materials/mat-1/preview/pages") {
      return Promise.resolve({ imageMode: false, pageCount: 0 });
    }
    if (path === "/student/favorites") {
      return Promise.resolve([]);
    }
    return Promise.reject(new Error("unexpected path " + path));
  };
  const page = loadMaterialPreviewPage(requestImpl, wxMock);

  page.onLoad({ id: "mat-1" });
  await flushPromises();

  assert.equal(page.data.recordingWarning, false);
  recordingHandler({ state: "start" });
  assert.equal(page.data.recordingWarning, true);
  assert.notEqual(page.data.previewMode, "unknown");
  recordingHandler({ state: "end" });
  assert.equal(page.data.recordingWarning, false);
});
