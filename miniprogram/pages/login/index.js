const { request } = require("../../utils/request");

Page({
  data: {
    binding: false,
    demoMode: false,
    phone: "18500009069",
    password: "123456"
  },
  onLoad() {
    this.setData({ demoMode: !getApp().globalData.useRealWechatLogin });
  },
  onPhoneInput(event) {
    this.setData({ phone: event.detail.value });
  },
  onPasswordInput(event) {
    this.setData({ password: event.detail.value });
  },
  showLoginError(error, fallback = "登录失败") {
    const message = error && error.message ? error.message : fallback;
    const title = message.indexOf("微信账号未绑定") !== -1 ? "请先用手机号一键登录" : message;
    wx.showToast({ title, icon: "none" });
  },
  demoLogin() {
    const phone = (this.data.phone || "").trim();
    const password = this.data.password || "";
    if (!/^1\d{10}$/.test(phone)) {
      wx.showToast({ title: "请输入 11 位手机号", icon: "none" });
      return;
    }
    if (!password) {
      wx.showToast({ title: "请输入密码", icon: "none" });
      return;
    }
    this.doLogin({ phone, password }, "/auth/demo-student-login");
  },
  // 微信一键登录：始终调用 wx.login() 获取临时 code（最佳实践）。
  // 生产环境上送真实 code 由后端换取 openId；演示环境上送 demoLoginCode 直接登录。
  login() {
    const app = getApp();
    wx.login({
      success: (res) => {
        const code = app.globalData.useRealWechatLogin ? res.code : (app.globalData.demoLoginCode || "student");
        if (!code) {
          wx.showToast({ title: "微信登录失败", icon: "none" });
          return;
        }
        this.doLogin({ code });
      },
      fail: () => wx.showToast({ title: "微信登录失败", icon: "none" })
    });
  },
  // 手机号绑定：getPhoneNumber 授权后，把手机号随登录一起上送给后端完成绑定。
  bindPhone(event) {
    const detail = event.detail || {};
    if (detail.errMsg && detail.errMsg.indexOf("ok") === -1) {
      wx.showToast({ title: "已取消手机号授权", icon: "none" });
      return;
    }
    if (!detail.code) {
      wx.showToast({ title: "未获取到手机号授权，请用真机调试", icon: "none" });
      return;
    }
    const app = getApp();
    wx.login({
      success: (res) => {
        const code = app.globalData.useRealWechatLogin ? res.code : (app.globalData.demoLoginCode || "student");
        // 生产环境：detail.code 为手机号凭据，后端调用 getuserphonenumber 解析后绑定。
        // 演示环境：detail.phoneNumber 可能不可用，后端按手机号匹配既有账号完成绑定。
        this.doLogin({ code, phone: detail.phoneNumber || "", phoneCode: detail.code });
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
