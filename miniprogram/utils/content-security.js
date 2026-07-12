const { request } = require("./request");

const captureHandlers = new Set();
let captureProtectionReported = false;

function activateContentSecurity(options = {}) {
  const context = normalizeContext(options);
  enableCaptureProtection(context);
  const handler = () => {
    safeToast("学习内容已加专属水印，请勿外传");
    reportSecurityEvent({ ...context, eventType: "screenshot", detail: "用户触发截屏事件" });
  };
  captureHandlers.add(handler);
  if (wx.onUserCaptureScreen) {
    wx.onUserCaptureScreen(handler);
  }
  reportSecurityEvent({ ...context, eventType: "content_view", detail: "进入安全学习页面" });
  return () => {
    captureHandlers.delete(handler);
    if (wx.offUserCaptureScreen) {
      wx.offUserCaptureScreen(handler);
    }
    if (captureHandlers.size === 0) {
      disableCaptureProtection(context);
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
