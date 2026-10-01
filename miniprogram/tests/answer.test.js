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
  assert.match(requestCall[2].data.requestId, /^[A-Za-z0-9_-]{1,64}$/);
  assert.deepEqual(requestCall[2].data, {
    requestId: requestCall[2].data.requestId,
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
    { title: "Please answer all questions", icon: "none" }
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
  assert.equal(calls.some((item) => item[0] === "showToast" && item[1] === "This content is watermarked. Please do not share it."), true);
  assert.equal(calls.some((item) => item[0] === "setVisualEffectOnCapture" && item[1] === "hidden"), true);
  assert.equal(calls.some((item) => item[0] === "setVisualEffectOnCapture" && item[1] === "none"), true);
});

test("answer page downloads the secured homework file when the API exposes permission", async () => {
  const calls = [];
  global.getApp = () => ({ globalData: { apiBaseUrl: "https://gate.example.com/api" } });
  const page = loadAnswerPage(() => Promise.resolve({}), {
    getStorageSync() { return "token-abc"; },
    showLoading(args) { calls.push(["showLoading", args.title]); },
    hideLoading() { calls.push(["hideLoading"]); },
    showToast(args) { calls.push(["showToast", args.title]); },
    showModal(args) { calls.push(["showModal", args.title, args.content]); },
    downloadFile(options) {
      calls.push(["downloadFile", options.url, options.header.Authorization]);
      options.success({ statusCode: 200, tempFilePath: "/tmp/homework.pdf" });
    },
    saveFile(options) {
      calls.push(["saveFile", options.tempFilePath]);
      options.success({ savedFilePath: "/saved/homework.pdf" });
    }
  });
  page.setData({ downloadUrl: "/api/student/homework/hw-1/download" });

  page.downloadHomework();
  await flushPromises();
  await flushPromises();

  assert.deepEqual(calls.find((item) => item[0] === "downloadFile"), [
    "downloadFile",
    "https://gate.example.com/api/student/homework/hw-1/download",
    "Bearer token-abc"
  ]);
  assert.deepEqual(calls.find((item) => item[0] === "saveFile"), ["saveFile", "/tmp/homework.pdf"]);
  assert.equal(calls.some((item) => item[0] === "showToast" && item[1] === "Exercise saved"), true);
});

test("judge options display English while preserving the submitted answer value", async () => {
  let payload;
  const page = loadAnswerPage((url, options = {}) => {
    if (url === "/student/homework/hw-judge") {
      return Promise.resolve({ questions: [{ id: "q-judge", type: "judge", options: [] }] });
    }
    if (url === "/student/favorites") return Promise.resolve([]);
    if (url === "/student/submissions") {
      payload = options.data;
      return Promise.resolve({ submissionId: "sub-judge" });
    }
    return Promise.resolve({});
  }, { getStorageSync() { return ""; }, removeStorageSync() {}, showToast() {}, navigateTo() {} });
  page.onLoad({ id: "hw-judge" });
  await flushPromises();
  assert.deepEqual(page.data.questions[0].options.map(option => option.label), ["A. True", "B. False"]);
  page.chooseOption({ currentTarget: { dataset: { qindex: 0, value: "正确" } } });
  page.submit();
  await flushPromises();
  assert.equal(payload.answers[0].choice, "正确");
  page.onUnload();
});

test("uncertain submission keeps its request ID across page reload; success clears it", async () => {
  const storage = { starline_student_id: 'child-b' };
  const wxMock = { getStorageSync: key => storage[key], setStorageSync: (key,value) => storage[key]=value, removeStorageSync: key => delete storage[key], showToast() {}, navigateTo() {} };
  const requests=[];
  const data={homeworkId:'hw-retry',questions:[{id:'q',type:'single',choice:'A',choices:[],text:''}]};
  const first=loadAnswerPage((url,opts)=>{requests.push(opts.data);return Promise.reject(new Error('timeout'));},wxMock);
  first.setData(data);first.submit();await flushPromises();
  assert.equal(first.data.saving,false);
  assert.equal(storage['starline_submission_request_child-b_hw-retry'].id, requests[0].requestId);
  const second=loadAnswerPage((url,opts)=>{requests.push(opts.data);return Promise.resolve({submissionId:'saved'});},wxMock);
  second.setData(data);second.submit();await flushPromises();
  assert.equal(requests[0].requestId,requests[1].requestId);
  assert.equal(storage['starline_submission_request_child-b_hw-retry'],undefined);
});
