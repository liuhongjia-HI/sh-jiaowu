const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "登录后可以同步学习记录、小挑战结果和老师反馈。",
    me: null
  },
  onLoad() {
    this.loadMe();
  },
  onShow() {
    if (!this.data.loading && !this.data.me) {
      this.loadMe();
    }
  },
  loadMe() {
    this.setData({ loading: true, error: "" });
    request("/student/me")
      .then((me) => this.setData({ me, loading: false }))
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "登录后可以同步学习记录、小挑战结果和老师反馈。",
        loading: false
      }));
  },
  goLogin() {
    wx.navigateTo({ url: "/pages/login/index" });
  },
  goSchedule() {
    wx.navigateTo({ url: "/pages/schedule/index" });
  },
  goGrowth() {
    wx.navigateTo({ url: "/pages/growth/index" });
  },
  goBadges() {
    wx.navigateTo({ url: "/pages/badges/index" });
  },
  goFavorites() {
    wx.navigateTo({ url: "/pages/favorites/index" });
  }
});
