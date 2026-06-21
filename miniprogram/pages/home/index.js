const { request } = require("../../utils/request");
const {
  showPhoneAuthFailed,
  isCancel
} = require("../../utils/phone-auth");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "请先登录绑定，或联系老师开通学习套餐。",
    home: null,
    hasContent: false,
    phoneAuthOpening: false,
    bindingPhone: false,
    pendingTask: null,
    continueCourse: null,
    progressPercent: 0,
    pendingCount: 0,
    courseTitle: "待解锁学习星球",
    courseMeta: "",
    mapTag: "继续学习"
  },
  onLoad() {
    this.loadHome();
  },
  onShow() {
    if (!this.data.loading && !this.data.home) {
      this.loadHome();
    }
  },
  loadHome() {
    this.setData({ loading: true, error: "" });
    request("/student/home")
      .then((home) => {
        const continueCourse = home.continueCourse || {};
        const pendingTask = (home.pendingHomework || [])[0] || null;
        const hasContent = !!continueCourse.id || (home.pendingHomework || []).length > 0 || (home.materials || []).length > 0;
        const progressPercent = Number(home.continueProgress) || 0;
        this.setData({
          home,
          hasContent,
          continueCourse,
          pendingTask,
          progressPercent,
          pendingCount: (home.pendingHomework || []).length,
          courseTitle: continueCourse.name || "待解锁学习星球",
          courseMeta: `${continueCourse.grade || ""} · ${continueCourse.subject || ""} · ${continueCourse.chapterCount || 0} 个章节`,
          mapTag: pendingTask ? "新小挑战" : "继续学习",
          loading: false
        });
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "请先登录绑定，或联系老师开通学习套餐。",
        hasContent: false,
        loading: false
      }));
  },
  beginPhoneAuth() {
    if (this.data.phoneAuthOpening || this.data.bindingPhone) {
      wx.showToast({ title: "正在打开授权，请稍候", icon: "none" });
      return;
    }
    this.setData({ phoneAuthOpening: true });
  },
  bindPhone(event) {
    const detail = event.detail || {};
    if (isCancel(detail)) {
      this.setData({ phoneAuthOpening: false });
      wx.showToast({ title: "已取消手机号授权", icon: "none" });
      return;
    }
    if (!detail.code) {
      this.setData({ phoneAuthOpening: false });
      showPhoneAuthFailed();
      return;
    }
    if (this.data.bindingPhone) {
      return;
    }
    this.setData({ phoneAuthOpening: false, bindingPhone: true });
    wx.login({
      success: (res) => {
        const code = res.code;
        if (!code) {
          wx.showToast({ title: "微信登录失败", icon: "none" });
          this.setData({ bindingPhone: false });
          return;
        }
        request("/auth/wechat-login", {
          method: "POST",
          data: { code, phoneCode: detail.code },
          skipAuth: true
        })
          .then((result) => {
            wx.setStorageSync("starline_token", result.token);
            wx.showToast({ title: "匹配成功", icon: "success" });
            this.loadHome();
          })
          .catch((error) => wx.showToast({ title: error.message || "匹配失败", icon: "none" }))
          .then(() => this.setData({ bindingPhone: false }));
      },
      fail: () => {
        wx.showToast({ title: "微信登录失败", icon: "none" });
        this.setData({ phoneAuthOpening: false, bindingPhone: false });
      }
    });
  },
  goStudyDetail() {
    if (!this.data.continueCourse || !this.data.continueCourse.id) {
      wx.switchTab({ url: "/pages/study/index" });
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${this.data.continueCourse.id}` });
  },
  goAnswer() {
    if (!this.data.pendingTask) {
      wx.switchTab({ url: "/pages/tasks/index" });
      return;
    }
    wx.navigateTo({ url: `/pages/answer/index?id=${this.data.pendingTask.id}` });
  },
  stopCardTap() {},
  goTasks() {
    wx.switchTab({ url: "/pages/tasks/index" });
  },
  goLogin() {
    wx.navigateTo({ url: "/pages/login/index" });
  }
});
