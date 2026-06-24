const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    savingProfile: false,
    error: "",
    emptyMessage: "登录后可以同步学习记录、小挑战结果和老师反馈。",
    me: null,
    profileForm: {
      nickname: "",
      avatarUrl: ""
    }
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
      .then((me) => this.setData({
        me,
        profileForm: {
          nickname: me.nickname || "",
          avatarUrl: me.avatarUrl || ""
        },
        loading: false
      }))
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "登录后可以同步学习记录、小挑战结果和老师反馈。",
        loading: false
      }));
  },
  goLogin() {
    wx.navigateTo({ url: "/pages/login/index" });
  },
  authorizeProfile() {
    if (this.data.savingProfile) {
      return;
    }
    if (!wx.getUserProfile) {
      wx.showToast({ title: "当前微信版本不支持资料授权", icon: "none" });
      return;
    }
    wx.getUserProfile({
      desc: "用于完善学生头像和昵称",
      success: (res) => {
        const userInfo = res.userInfo || {};
        const nickname = (userInfo.nickName || "").trim();
        const avatarUrl = userInfo.avatarUrl || "";
        this.submitProfile(nickname, avatarUrl);
      },
      fail: () => wx.showToast({ title: "已取消头像昵称授权", icon: "none" })
    });
  },
  submitProfile(nickname, avatarUrl) {
    if (!avatarUrl) {
      wx.showToast({ title: "请授权微信头像", icon: "none" });
      return;
    }
    if (!nickname) {
      wx.showToast({ title: "请授权微信昵称", icon: "none" });
      return;
    }
    this.setData({ savingProfile: true });
    request("/student/profile", {
      method: "PUT",
      data: { nickname, avatarUrl }
    })
      .then((me) => {
        this.setData({
          me,
          profileForm: {
            nickname: me.nickname || "",
            avatarUrl: me.avatarUrl || ""
          }
        });
        wx.showToast({ title: "已保存", icon: "success" });
      })
      .catch((error) => wx.showToast({ title: error.message || "保存失败", icon: "none" }))
      .then(() => this.setData({ savingProfile: false }));
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
