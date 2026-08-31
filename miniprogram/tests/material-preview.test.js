const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

test("material preview keeps only the preview card as the full-courseware entry", () => {
  const template = fs.readFileSync(path.join(__dirname, "../pages/material-preview/index.wxml"), "utf8");

  assert.doesNotMatch(template, />打开完整课件<\/button>/);
  assert.doesNotMatch(template, /class="preview-action"/);
  assert.doesNotMatch(template, /class="watermark-layer"/);
  assert.match(template, /class="button challenge-button" bindtap="goAnswer"/);
});

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

test("material preview downloads only the first page as the clickable cover", async () => {
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
  assert.equal(page.data.previewImagePath, "https://gate.example.com/api/student/materials/mat-1/preview/pages/1#local");
  assert.equal(page.data.pagesLoading, false);
  assert.deepEqual(downloadedUrls, [
    "https://gate.example.com/api/student/materials/mat-1/preview/pages/1"
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
  assert.equal(page.data.previewImagePath, "");
});

test("material preview lets the student retry when the first-page cover fails", async () => {
  let call = 0;
  const wxMock = baseWxMock({
    downloadFile(opts) {
      call += 1;
      if (call === 1) {
        opts.fail(new Error("network drop"));
        return;
      }
      opts.success({ statusCode: 200, tempFilePath: "page-1#local" });
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

  assert.equal(page.data.previewMode, "cover-error");
  assert.equal(page.data.pagesLoading, false);
  assert.equal(page.data.previewImagePath, "");

  page.retryPreview();
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.previewMode, "image");
  assert.equal(page.data.previewImagePath, "page-1#local");
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
      opts.success && opts.success();
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

test("openSecurePreview ignores repeated taps while the document is opening", async () => {
  const downloadedUrls = [];
  let finishDownload;
  const wxMock = baseWxMock({
    downloadFile(opts) {
      downloadedUrls.push(opts.url);
      finishDownload = () => opts.success({ statusCode: 200, tempFilePath: "secure-preview.pdf#local" });
    },
    openDocument(opts) {
      opts.success && opts.success();
    }
  });
  const page = loadMaterialPreviewPage(() => Promise.reject(new Error("unused")), wxMock);
  page.setData({ material: { previewUrl: "/api/student/materials/mat-1/preview" } });

  page.openSecurePreview();
  page.openSecurePreview();
  assert.equal(page.data.openingPreview, true);
  assert.deepEqual(downloadedUrls, ["https://gate.example.com/api/student/materials/mat-1/preview"]);

  finishDownload();
  await flushPromises();
  await flushPromises();
  assert.equal(page.data.openingPreview, false);
});

test("openSecurePreview opens the watermarked PDF even when a page cover is available", async () => {
  const downloadedUrls = [];
  let openedPath = "";
  const page = loadMaterialPreviewPage(() => Promise.reject(new Error("unused")), baseWxMock({
    downloadFile(opts) {
      downloadedUrls.push(opts.url);
      opts.success({ statusCode: 200, tempFilePath: "secure-preview.pdf#local" });
    },
    openDocument(opts) {
      openedPath = opts.filePath;
      opts.success && opts.success();
    }
  }));
  page.materialId = "mat-1";
  page.setData({
    material: { previewUrl: "/api/student/materials/mat-1/preview", title: "第一课" },
    previewMode: "image",
    previewImagePath: "page-1#local",
    pageCount: 3
  });

  page.openSecurePreview();
  await flushPromises();
  await flushPromises();

  assert.equal("readerOpen" in page.data, false);
  assert.equal(openedPath, "secure-preview.pdf#local");
  assert.deepEqual(downloadedUrls, [
    "https://gate.example.com/api/student/materials/mat-1/preview"
  ]);
});

test("material preview consumes structured processing status without treating it as an error", async () => {
  const requestImpl = (path) => {
    if (path === "/student/materials/mat-1") {
      return Promise.resolve({ id: "mat-1", title: "第一课", previewUrl: "/api/student/materials/mat-1/preview" });
    }
    if (path === "/student/materials/mat-1/preview/pages") {
      return Promise.resolve({ previewStatus: "processing", imageMode: false, pageCount: 0, message: "课件正在生成，请稍后再试" });
    }
    if (path === "/student/favorites") return Promise.resolve([]);
    return Promise.reject(new Error("unexpected path " + path));
  };
  const page = loadMaterialPreviewPage(requestImpl, baseWxMock());

  page.onLoad({ id: "mat-1" });
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.previewMode, "processing");
  assert.equal(page.data.previewMessage, "课件正在生成，请稍后再试");
  page.onUnload();
});

test("material detail failure leaves the loading state with an actionable message", async () => {
  const page = loadMaterialPreviewPage((path) => {
    if (path === "/student/materials/mat-1") return Promise.reject(new Error("network unavailable"));
    if (path === "/student/favorites") return Promise.resolve([]);
    return Promise.reject(new Error("unexpected path " + path));
  }, baseWxMock());

  page.onLoad({ id: "mat-1" });
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.previewMode, "unavailable");
  assert.equal(page.data.previewMessage, "资料加载失败，请重新进入");
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

test("small challenge prioritizes a task from the material's lesson", async () => {
  const navigatedUrls = [];
  const page = loadMaterialPreviewPage((path) => {
    if (path === "/student/tasks") {
      return Promise.resolve([
        { id: "homework-other", courseId: "course-1", lessonId: "lesson-2" },
        { id: "homework-matched", courseId: "course-1", lessonId: "lesson-1" }
      ]);
    }
    return Promise.reject(new Error("unexpected path " + path));
  }, baseWxMock({
    navigateTo({ url }) { navigatedUrls.push(url); }
  }));
  page.setData({ material: { courseId: "course-1", lessonId: "lesson-1" } });

  page.goAnswer();
  await flushPromises();

  assert.deepEqual(navigatedUrls, ["/pages/answer/index?id=homework-matched"]);
});

test("small challenge does not use a task that is missing the material's lesson", async () => {
  const navigatedUrls = [];
  const toastTitles = [];
  const page = loadMaterialPreviewPage((path) => {
    if (path === "/student/tasks") {
      return Promise.resolve([{ id: "homework-course", courseId: "course-1" }]);
    }
    return Promise.reject(new Error("unexpected path " + path));
  }, baseWxMock({
    navigateTo({ url }) { navigatedUrls.push(url); },
    showToast({ title }) { toastTitles.push(title); }
  }));
  page.setData({ material: { courseId: "course-1", lessonId: "lesson-1" } });

  page.goAnswer();
  await flushPromises();

  assert.deepEqual(navigatedUrls, ["/pages/tasks/index"]);
  assert.deepEqual(toastTitles, ["本课节暂无小挑战"]);
});

test("small challenge opens the task list when the lesson has no matching task", async () => {
  const navigatedUrls = [];
  const toastTitles = [];
  const page = loadMaterialPreviewPage((path) => {
    if (path === "/student/tasks") {
      return Promise.resolve([{ id: "homework-other", courseId: "course-1", lessonId: "lesson-2" }]);
    }
    return Promise.reject(new Error("unexpected path " + path));
  }, baseWxMock({
    navigateTo({ url }) { navigatedUrls.push(url); },
    showToast({ title }) { toastTitles.push(title); }
  }));
  page.setData({ material: { courseId: "course-1", lessonId: "lesson-1" } });

  page.goAnswer();
  await flushPromises();

  assert.deepEqual(navigatedUrls, ["/pages/tasks/index"]);
  assert.deepEqual(toastTitles, ["本课节暂无小挑战"]);
});
