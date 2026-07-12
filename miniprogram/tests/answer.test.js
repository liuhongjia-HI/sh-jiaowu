const assert = require("node:assert/strict");
const test = require("node:test");

function loadAnswerPage(requestImpl, wxMock) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const securityPath = require.resolve("../utils/content-security");
  const pagePath = require.resolve("../pages/answer/index.js");
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

test("answer page builds submission payload from selected option and text answer", async () => {
  const calls = [];
  const wxMock = {
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    },
    showToast(args) {
      calls.push(["showToast", args]);
    },
    navigateTo(args) {
      calls.push(["navigateTo", args.url]);
    },
    getStorageSync() {
      return "";
    }
  };
  const page = loadAnswerPage((url, options = {}) => {
    calls.push(["request", url, options]);
    return Promise.resolve({ submissionId: "sub-answer-001" });
  }, wxMock);
  page.setData({
    homeworkId: "hw-answer-001",
    questions: [
      {
        id: "q-single",
        type: "single",
        choice: "",
        choices: [],
        text: "",
        options: [
          { value: "A", className: "" },
          { value: "B", className: "" }
        ]
      },
      { id: "q-text", type: "text", choice: "", choices: [], text: "", options: [] }
    ]
  });

  page.chooseOption({ currentTarget: { dataset: { qindex: 0, value: "A" } } });
  page.changeAnswer({ currentTarget: { dataset: { qindex: 1 } }, detail: { value: "我学会了先找关键词。" } });
  page.submit();
  await flushPromises();

  const requestCall = calls.find((item) => item[0] === "request");
  assert.equal(requestCall[1], "/student/submissions");
  assert.deepEqual(requestCall[2].data, {
    homeworkId: "hw-answer-001",
    answers: [
      { questionId: "q-single", choice: "A", choices: [], text: "" },
      { questionId: "q-text", choice: "", choices: [], text: "我学会了先找关键词。" }
    ]
  });
  assert.equal(page.data.saving, true);
  assert.deepEqual(calls.find((item) => item[0] === "removeStorageSync"), ["removeStorageSync", "starline_homework_draft_hw-answer-001"]);
  assert.deepEqual(calls.find((item) => item[0] === "navigateTo"), ["navigateTo", "/pages/result/index?id=sub-answer-001"]);
});

test("answer page blocks submission when any question is unanswered", () => {
  const calls = [];
  const page = loadAnswerPage((url, options = {}) => {
    calls.push(["request", url, options]);
    return Promise.resolve({});
  }, {
    showToast(args) {
      calls.push(["showToast", args]);
    },
    getStorageSync() {
      return "";
    }
  });
  page.setData({
    homeworkId: "hw-answer-002",
    questions: [
      { id: "q-single", type: "single", choice: "A", choices: [], text: "", options: [] },
      { id: "q-text", type: "text", choice: "", choices: [], text: "   ", options: [] }
    ]
  });

  page.submit();

  assert.equal(calls.some((item) => item[0] === "request"), false);
  assert.deepEqual(calls.find((item) => item[0] === "showToast"), [
    "showToast",
    { title: "还有题目没有完成哦", icon: "none" }
  ]);
});

test("answer page applies dynamic watermark and reports capture event", async () => {
  const calls = [];
  let captureHandler = null;
  const wxMock = {
    onUserCaptureScreen(handler) {
      captureHandler = handler;
      calls.push(["onUserCaptureScreen"]);
    },
    offUserCaptureScreen(handler) {
      calls.push(["offUserCaptureScreen", handler === captureHandler]);
    },
    setVisualEffectOnCapture(options) {
      calls.push(["setVisualEffectOnCapture", options.visualEffect]);
      options.success && options.success();
    },
    showToast(args) {
      calls.push(["showToast", args.title]);
    },
    getStorageSync() {
      return "";
    }
  };
  const page = loadAnswerPage((url, options = {}) => {
    calls.push(["request", url, options.data || {}]);
    if (url === "/student/homework/hw-secure") {
      return Promise.resolve({
        title: "安全题库",
        course: "英语",
        deadline: "2026-12-31",
        watermarkText: "小明 · 尾号9069 · 2026-07-11 10:00 · IDstu001",
        securityNotice: "请勿截屏录屏或外传。",
        questions: [{ id: "q1", type: "single", stem: "题干", options: ["A"], answer: "A" }]
      });
    }
    if (url === "/student/favorites") {
      return Promise.resolve([]);
    }
    return Promise.resolve({});
  }, wxMock);

  page.onLoad({ id: "hw-secure" });
  await flushPromises();
  captureHandler && captureHandler();
  page.onUnload();
  await flushPromises();

  assert.equal(page.data.watermarkText, "小明 · 尾号9069 · 2026-07-11 10:00 · IDstu001");
  assert.equal(page.data.securityNotice, "请勿截屏录屏或外传。");
  assert.equal(calls.some((item) => item[0] === "request" && item[1] === "/student/security/events"), true);
  assert.equal(calls.some((item) => item[0] === "showToast" && item[1] === "学习内容已加专属水印，请勿外传"), true);
  assert.equal(calls.some((item) => item[0] === "setVisualEffectOnCapture" && item[1] === "hidden"), true);
  assert.equal(calls.some((item) => item[0] === "setVisualEffectOnCapture" && item[1] === "none"), true);
});
