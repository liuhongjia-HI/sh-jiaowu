Page({
  onShareAppMessage() {
    return { title: "About Starline", path: "/pages/starline-intro/index" };
  },
  goStudy() {
    wx.switchTab({ url: "/pages/study/index" });
  }
});
