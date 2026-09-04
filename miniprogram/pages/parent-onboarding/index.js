Page({
  data: {
    benefits: [
      { icon: "学", tone: "orange", title: "课程学习", summary: "查看课程和学习进度" },
      { icon: "练", tone: "blue", title: "课后练习", summary: "完成练习并查看结果" },
      { icon: "评", tone: "green", title: "老师反馈", summary: "了解表现和改进建议" }
    ]
  },
  goAddStudent() {
    this.openLogin("add");
  },
  goBindStudent() {
    this.openLogin("bind");
  },
  openLogin(mode) {
    wx.navigateTo({ url: `/pages/login/index?mode=${mode}` });
  }
});
