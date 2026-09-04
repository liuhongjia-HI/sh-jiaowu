const { request } = require("./request");

const captureHandlers = new Set();
let captureProtectionReported = false;
let recordingCapabilityReported = false;

// activateContentSecurity 在进入内容页时启用截屏/录屏相关的风控能力。
// options.onRecordingChange(isRecording) 是可选回调：录屏开始时页面可以据此
// 隐藏正文内容，录屏结束后恢复，降低整段内容被批量录制的价值。
// 录屏没有任何技术手段能100%拦截（另一台设备对屏幕拍摄永远拍得到），
// 这里做的是提高成本 + 留痕取证，不是承诺“防住”。
function activateContentSecurity(options = {}) {
  const context = normalizeContext(options);
  const onRecordingChange = typeof options.onRecordingChange === "function" ? options.onRecordingChange : null;
  enableCaptureProtection(context);
  const captureHandler = () => {
    safeToast("学习内容已加专属水印，请勿外传");
    reportSecurityEvent({ ...context, eventType: "screenshot", detail: "用户触发截屏事件" });
    if (isIPadIOS()) {
      safeToast("检测到截图，页面即将返回");
      navigateBackFromSecurePage();
    }
  };
  captureHandlers.add(captureHandler);
  if (wx.onUserCaptureScreen) {
    wx.onUserCaptureScreen(captureHandler);
  }
  const recordingHandler = buildRecordingHandler(context, onRecordingChange);
  if (wx.onScreenRecordingStateChanged) {
    wx.onScreenRecordingStateChanged(recordingHandler);
  } else {
    reportRecordingCapability(context, "unsupported");
  }
  reportSecurityEvent({ ...context, eventType: "content_view", detail: "进入安全学习页面" });
  return () => {
    captureHandlers.delete(captureHandler);
    if (wx.offUserCaptureScreen) {
      wx.offUserCaptureScreen(captureHandler);
    }
    if (wx.offScreenRecordingStateChanged) {
      wx.offScreenRecordingStateChanged(recordingHandler);
    }
    if (onRecordingChange) {
      onRecordingChange(false);
    }
    if (captureHandlers.size === 0) {
      disableCaptureProtection(context);
    }
  };
}

function isIPadIOS() {
  if (!wx.getDeviceInfo) return false;
  const info = wx.getDeviceInfo() || {};
  const platform = String(info.platform || "").toLowerCase();
  const model = String(info.model || "").toLowerCase();
  return platform === "ios" && model.includes("ipad");
}

function navigateBackFromSecurePage() {
  if (!wx.navigateBack) return;
  wx.navigateBack({
    delta: 1,
    fail() {
      if (wx.reLaunch) wx.reLaunch({ url: "/pages/study/index" });
    }
  });
}

function buildRecordingHandler(context, onRecordingChange) {
  return (res) => {
    const isRecording = res && res.state === "start";
    reportRecordingCapability(context, "enabled");
    if (onRecordingChange) {
      onRecordingChange(isRecording);
    }
    reportSecurityEvent({
      ...context,
      eventType: isRecording ? "screen_recording_start" : "screen_recording_end",
      detail: isRecording ? "检测到系统录屏已开始" : "系统录屏已结束"
    });
    if (isRecording) {
      safeToast("检测到录屏，内容已隐藏");
    }
  };
}

function reportSecurityEvent(options = {}) {
  const context = normalizeContext(options);
  return request("/student/security/events", {
    method: "POST",
    silent: true,
    data: {
      eventType: context.eventType,
      targetType: context.targetType,
      targetId: context.targetId,
      pagePath: context.pagePath,
      detail: context.detail
    }
  }).catch(() => {});
}

function enableCaptureProtection(context) {
  if (!wx.setVisualEffectOnCapture) {
    reportCaptureCapability(context, "unsupported");
    return;
  }
  wx.setVisualEffectOnCapture({
    visualEffect: "hidden",
    success() {
      reportCaptureCapability(context, "enabled");
    },
    fail() {
      reportCaptureCapability(context, "failed");
    }
  });
}

function disableCaptureProtection(context) {
  if (!wx.setVisualEffectOnCapture) {
    return;
  }
  wx.setVisualEffectOnCapture({
    visualEffect: "none",
    fail() {
      reportSecurityEvent({ ...context, eventType: "capture_protection_restore_failed", detail: "恢复截屏保护失败" });
    }
  });
}

function reportCaptureCapability(context, status) {
  if (captureProtectionReported) {
    return;
  }
  captureProtectionReported = true;
  reportSecurityEvent({ ...context, eventType: "capture_protection_" + status, detail: "平台捕获保护能力：" + status });
}

function reportRecordingCapability(context, status) {
  if (recordingCapabilityReported) {
    return;
  }
  recordingCapabilityReported = true;
  reportSecurityEvent({ ...context, eventType: "recording_detection_" + status, detail: "录屏检测能力：" + status });
}

function normalizeContext(options = {}) {
  return {
    eventType: String(options.eventType || ""),
    targetType: String(options.targetType || ""),
    targetId: String(options.targetId || ""),
    pagePath: String(options.pagePath || currentPagePath()),
    detail: String(options.detail || "")
  };
}

function currentPagePath() {
  if (typeof getCurrentPages !== "function") {
    return "";
  }
  const pages = getCurrentPages();
  const current = pages[pages.length - 1];
  return current ? current.route || "" : "";
}

function safeToast(title) {
  if (wx.showToast) {
    wx.showToast({ title, icon: "none" });
  }
}

module.exports = {
  activateContentSecurity,
  reportSecurityEvent
};
