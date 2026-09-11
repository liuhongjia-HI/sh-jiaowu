const ONBOARDING_SEEN_KEY = "starline_onboarding_seen";

Page({
  data: {
    benefits: [
      { icon: "学", tone: "orange", title: "课程学习", summary: "查看课程和学习进度" },
      { icon: "练", tone: "blue", title: "课后练习", summary: "完成练习并查看结果" },
      { icon: "评", tone: "green", title: "老师反馈", summary: "了解表现和改进建议" }
    ]
  },
  onShow() {
    this.markSeen();
  },
  goAddStudent() {
    this.openLogin("add");
  },
  goBindStudent() {
    this.openLogin("bind");
  },
  skipForNow() {
    this.leaveToHome();
  },
  openLogin(mode) {
    this.markSeen();
    wx.navigateTo({ url: `/pages/login/index?mode=${mode}` });
  },
  markSeen() {
    if (wx.setStorageSync) {
      wx.setStorageSync(ONBOARDING_SEEN_KEY, "1");
    }
  },
  leaveToHome() {
    this.markSeen();
    wx.switchTab({
      url: "/pages/home/index",
      fail: () => {
        if (typeof wx.navigateBack === "function") {
          wx.navigateBack({
            fail: () => {
              if (typeof wx.reLaunch === "function") {
                wx.reLaunch({ url: "/pages/home/index" });
              }
            }
          });
          return;
        }
        if (typeof wx.reLaunch === "function") {
          wx.reLaunch({ url: "/pages/home/index" });
        }
      }
    });
  }
});
