const assert = require("node:assert/strict");
const test = require("node:test");

function loadContentSecurity(wxMock, requestImpl) {
  const requestPath = require.resolve("../utils/request");
  const securityPath = require.resolve("../utils/content-security");
  delete require.cache[requestPath];
  delete require.cache[securityPath];
  require.cache[requestPath] = {
    id: requestPath,
    filename: requestPath,
    loaded: true,
    exports: { request: requestImpl || (() => Promise.resolve()) }
  };
  global.wx = wxMock;
  global.getCurrentPages = () => [{ route: "pages/material-preview/index" }];
  return require(securityPath);
}

function baseWxMock(overrides = {}) {
  return {
    showToast() {},
    setVisualEffectOnCapture(opts) {
      opts.success && opts.success();
    },
    onUserCaptureScreen() {},
    offUserCaptureScreen() {},
    ...overrides
  };
}

test("activateContentSecurity registers a screen recording listener when the platform supports it", () => {
  let registeredHandler = null;
  const wxMock = baseWxMock({
    onScreenRecordingStateChanged(handler) {
      registeredHandler = handler;
    },
    offScreenRecordingStateChanged() {}
  });
  const { activateContentSecurity } = loadContentSecurity(wxMock);

  activateContentSecurity({ targetType: "material", targetId: "mat-1" });

  assert.equal(typeof registeredHandler, "function");
});

test("activateContentSecurity notifies onRecordingChange when recording starts and stops", () => {
  let registeredHandler = null;
  const wxMock = baseWxMock({
    onScreenRecordingStateChanged(handler) {
      registeredHandler = handler;
    },
    offScreenRecordingStateChanged() {}
  });
  const { activateContentSecurity } = loadContentSecurity(wxMock);
  const changes = [];

  activateContentSecurity({
    targetType: "material",
    targetId: "mat-1",
    onRecordingChange: (isRecording) => changes.push(isRecording)
  });

  registeredHandler({ state: "start" });
  registeredHandler({ state: "end" });

  assert.deepEqual(changes, [true, false]);
});

test("activateContentSecurity forces onRecordingChange(false) on cleanup so a stale page never stays hidden", () => {
  let registeredHandler = null;
  let unregisteredHandler = null;
  const wxMock = baseWxMock({
    onScreenRecordingStateChanged(handler) {
      registeredHandler = handler;
    },
    offScreenRecordingStateChanged(handler) {
      unregisteredHandler = handler;
    }
  });
  const { activateContentSecurity } = loadContentSecurity(wxMock);
  const changes = [];

  const stop = activateContentSecurity({
    targetType: "material",
    targetId: "mat-1",
    onRecordingChange: (isRecording) => changes.push(isRecording)
  });
  registeredHandler({ state: "start" });
  stop();

  assert.equal(unregisteredHandler, registeredHandler);
  assert.deepEqual(changes, [true, false]);
});

test("activateContentSecurity degrades silently when the platform has no recording API", () => {
  const wxMock = baseWxMock();
  const { activateContentSecurity } = loadContentSecurity(wxMock);

  assert.doesNotThrow(() => {
    const stop = activateContentSecurity({ targetType: "material", targetId: "mat-1" });
    stop();
  });
});

test("iPad screenshot navigates back while iPhone keeps the page open", () => {
  let captureHandler = null;
  const calls = [];
  const wxMock = baseWxMock({
    getDeviceInfo() {
      return { platform: "ios", model: "iPad13,18" };
    },
    onUserCaptureScreen(handler) {
      captureHandler = handler;
    },
    navigateBack(options) {
      calls.push("navigateBack");
      options && options.success && options.success();
    },
    showToast(args) {
      calls.push(args.title);
    }
  });
  const { activateContentSecurity } = loadContentSecurity(wxMock);
  const stop = activateContentSecurity({ targetType: "material", targetId: "mat-1" });
  captureHandler();
  stop();

  assert.equal(calls.includes("navigateBack"), true);
  assert.equal(calls.includes("检测到截图，页面即将返回"), true);

  let iphoneCapture = null;
  const iphoneCalls = [];
  const iphoneWx = baseWxMock({
    getDeviceInfo() {
      return { platform: "ios", model: "iPhone15,2" };
    },
    onUserCaptureScreen(handler) { iphoneCapture = handler; },
    navigateBack() { iphoneCalls.push("navigateBack"); }
  });
  const security = loadContentSecurity(iphoneWx);
  const stopIphone = security.activateContentSecurity({ targetType: "material", targetId: "mat-1" });
  iphoneCapture();
  stopIphone();
  assert.equal(iphoneCalls.includes("navigateBack"), false);
});
