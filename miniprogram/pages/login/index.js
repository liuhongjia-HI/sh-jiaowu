const { request } = require("../../utils/request");
const {
  showPhoneAuthFailed,
  isCancel
} = require("../../utils/phone-auth");

Page({
  data: {
    binding: false,
    gradeOptions: ["一年级", "二年级", "三年级", "四年级", "五年级", "六年级", "七年级", "八年级", "九年级", "十年级", "十一年级", "十二年级"],
    gradeIndex: -1,
    form: {
      studentName: "",
      schoolName: "",
      grade: ""
    }
  },
  onLoad() {
    this.silentLogin();
  },
  onInput(event) {
    const field = event.currentTarget.dataset.field;
    this.setData({ [`form.${field}`]: event.detail.value });
  },
  onGradeChange(event) {
    const index = Number(event.detail.value);
    const grade = this.data.gradeOptions[index] || "";
    this.setData({
      gradeIndex: index,
      "form.grade": grade
    });
  },
  validateProfile() {
    const form = this.data.form;
    if (!(form.studentName || "").trim()) {
      wx.showToast({ title: "请输入学生姓名", icon: "none" });
      return false;
    }
    if (!(form.schoolName || "").trim()) {
      wx.showToast({ title: "请输入学校", icon: "none" });
      return false;
    }
    if (!(form.grade || "").trim()) {
      wx.showToast({ title: "请输入年级", icon: "none" });
      return false;
    }
    return true;
  },
  showLoginError(error, fallback = "登录失败") {
    if (error && error.userNotified) {
      return;
    }
    const message = error && error.message ? error.message : fallback;
    const title = message.indexOf("微信账号未绑定") !== -1 ? "请先用手机号一键登录" : message;
    wx.showToast({ title, icon: "none" });
  },
  silentLogin() {
    if (this.data.binding) {
      return;
    }
    wx.login({
      success: (res) => {
        const code = res.code;
        if (!code) {
          return;
        }
        request("/auth/wechat-login", { method: "POST", data: { code } })
          .then((result) => {
            wx.setStorageSync("starline_token", result.token);
            wx.switchTab({ url: "/pages/home/index" });
          })
          .catch((error) => {
            const message = error && error.message ? error.message : "";
            if (message.indexOf("微信账号未绑定") === -1) {
              wx.removeStorageSync("starline_token");
            }
          });
      }
    });
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
    if (!this.validateProfile()) {
      return;
    }
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
        if (!code) {
          wx.showToast({ title: "微信登录失败，请重试", icon: "none" });
          return;
        }
        // detail.code 为手机号凭据，后端调用 getuserphonenumber 解析后绑定。
        const form = this.data.form;
        this.doLogin({
          code,
          phoneCode: detail.code,
          studentName: (form.studentName || "").trim(),
          schoolName: (form.schoolName || "").trim(),
          grade: (form.grade || "").trim()
        });
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
        wx.showToast({ title: "绑定成功", icon: "success" });
        wx.switchTab({ url: "/pages/home/index" });
      })
      .catch((error) => this.showLoginError(error))
      .then(() => this.setData({ binding: false }));
  }
});
