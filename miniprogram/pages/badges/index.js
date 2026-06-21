const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    emptyMessage: "继续学习，点亮属于你的成长徽章吧。",
    badges: [],
    obtainedCount: 0
  },
  onLoad() {
    this.loadBadges();
  },
  onShow() {
    if (!this.data.loading && this.data.badges.length === 0) {
      this.loadBadges();
    }
  },
  loadBadges() {
    this.setData({ loading: true });
    request("/student/badges")
      .then((badges) => {
        const list = badges || [];
        this.setData({
          badges: list,
          obtainedCount: list.filter((badge) => badge.obtained).length,
          loading: false
        });
      })
      .catch((error) => this.setData({
        emptyMessage: error.message || "加载失败",
        loading: false
      }));
  }
});
