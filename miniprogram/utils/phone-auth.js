function showPhoneAuthFailed(message) {
  wx.showModal({
    title: "Authorization unavailable",
    content: message || "Phone authorization failed. Try again later or contact your teacher.",
    showCancel: false,
    confirmText: "OK"
  });
}

function isCancel(detail = {}) {
  return detail.errMsg && detail.errMsg.indexOf("ok") === -1;
}

module.exports = {
  showPhoneAuthFailed,
  isCancel
};
