const {
  request,
  LOGIN_RETURN_KEY = "starline_after_login",
  completeLoginRedirect = () => {}
} = require("../../utils/request");
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
  onUnload() {
    completeLoginRedirect();
  },
  onShareAppMessage() {
    return {
      title: "加入 Starline 学习",
      path: "/pages/home/index"
    };
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
            resumeAfterLogin();
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
        // 多子女：手机号命中多个学生档案时后端不报错，而是返回候选列表，
        // 这里弹出选择框，选中后带着 selectedStudentId 重新提交同一份登录请求。
        // binding 必须在调起选择框之前就复位——resubmit 会再走一次 doLogin，
        // 如果复位挂在链条末尾的公共 .then 里，会在 resubmit 的请求还没回来
        // 时就把它的 binding:true 覆盖掉。
        if (result && result.needsSelection) {
          this.setData({ binding: false });
          this.promptStudentSelection(result.candidates || [], payload, path);
          return;
        }
        wx.setStorageSync("starline_token", result.token);
        wx.showToast({ title: "绑定成功", icon: "success" });
        resumeAfterLogin();
        this.setData({ binding: false });
      })
      .catch((error) => {
        this.showLoginError(error);
        this.setData({ binding: false });
      });
  },
  promptStudentSelection(candidates, payload, path) {
    if (!candidates.length) {
      wx.showToast({ title: "未找到可绑定的学生，请联系老师", icon: "none" });
      return;
    }
    wx.showActionSheet({
      itemList: candidates.map((item) => `${item.name} · ${item.grade}`),
      success: (res) => {
        const picked = candidates[res.tapIndex];
        if (!picked) {
          return;
        }
        this.doLogin({ ...payload, selectedStudentId: picked.studentId }, path);
      }
      // fail：家长取消选择，不做任何事，允许重新点击登录按钮。
    });
  }
});

function resumeAfterLogin() {
  completeLoginRedirect();
  const destination = wx.getStorageSync ? wx.getStorageSync(LOGIN_RETURN_KEY) : "";
  if (destination && wx.removeStorageSync) {
    wx.removeStorageSync(LOGIN_RETURN_KEY);
  }
  const pagePath = String(destination || "").split("?")[0];
  if (!pagePath.startsWith("/pages/") || pagePath === "/pages/login/index") {
    wx.switchTab({ url: "/pages/home/index" });
    return;
  }
  const tabPages = ["/pages/home/index", "/pages/study/index", "/pages/notices/index", "/pages/me/index"];
  if (tabPages.includes(pagePath)) {
    wx.switchTab({ url: pagePath });
    return;
  }
  if (wx.redirectTo) {
    wx.redirectTo({ url: destination });
    return;
  }
  wx.switchTab({ url: "/pages/home/index" });
}
