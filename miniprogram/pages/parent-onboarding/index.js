const ONBOARDING_SEEN_KEY = "starline_onboarding_seen";

Page({
  data: {
    benefits: [
      { icon: "📚", tone: "orange", title: "Courses", summary: "View courses and learning progress" },
      { icon: "✍️", tone: "blue", title: "Exercises", summary: "Complete exercises and view results" },
      { icon: "💬", tone: "green", title: "Teacher Feedback", summary: "Review performance and advice" }
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
