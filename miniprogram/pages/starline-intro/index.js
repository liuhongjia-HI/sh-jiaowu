Page({
  onShareAppMessage() {
    return { title: "了解 Starline", path: "/pages/starline-intro/index" };
  },
  goStudy() {
    wx.switchTab({ url: "/pages/study/index" });
  }
});
