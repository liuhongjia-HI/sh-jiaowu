const { request } = require("../../utils/request");
const {
  showPhoneAuthFailed,
  isCancel
} = require("../../utils/phone-auth");

Page({
  data: {
    binding: false
  },
  showLoginError(error, fallback = "登录失败") {
    const message = error && error.message ? error.message : fallback;
    const title = message.indexOf("微信账号未绑定") !== -1 ? "请先用手机号一键登录" : message;
    wx.showToast({ title, icon: "none" });
  },
  // 微信一键登录：始终调用 wx.login() 获取临时 code，由后端换取 openId。
  login() {
    wx.login({
      success: (res) => {
        const code = res.code;
        if (!code) {
          wx.showToast({ title: "微信登录失败", icon: "none" });
          return;
        }
        this.doLogin({ code });
      },
      fail: () => {
        wx.showToast({ title: "微信登录失败", icon: "none" });
      }
    });
  },
  // 手机号绑定：getPhoneNumber 授权后，把手机号随登录一起上送给后端完成绑定。
  bindPhone(event) {
    const detail = event.detail || {};
    if (isCancel(detail)) {
      wx.showToast({ title: "已取消手机号授权", icon: "none" });
      return;
    }
    if (!detail.code) {
      showPhoneAuthFailed();
      return;
    }
    wx.login({
      success: (res) => {
        const code = res.code;
        // detail.code 为手机号凭据，后端调用 getuserphonenumber 解析后绑定。
        this.doLogin({ code, phoneCode: detail.code });
      },
      fail: () => wx.showToast({ title: "微信登录失败", icon: "none" })
    });
  },
  doLogin(payload, path = "/auth/wechat-login") {
    if (this.data.binding) {
      return;
    }
    this.setData({ binding: true });
    request(path, { method: "POST", data: payload })
      .then((result) => {
        wx.setStorageSync("starline_token", result.token);
        wx.showToast({ title: "登录成功", icon: "success" });
        wx.switchTab({ url: "/pages/home/index" });
      })
      .catch((error) => this.showLoginError(error))
      .then(() => this.setData({ binding: false }));
  }
});
