function showPhoneAuthFailed(message) {
  wx.showModal({
    title: "暂时无法完成授权",
    content: message || "没有获取到手机号授权结果。请稍后重试，或联系老师确认课程和账号信息。",
    showCancel: false,
    confirmText: "知道了"
  });
}

function isCancel(detail = {}) {
  return detail.errMsg && detail.errMsg.indexOf("ok") === -1;
}

module.exports = {
  showPhoneAuthFailed,
  isCancel
};
