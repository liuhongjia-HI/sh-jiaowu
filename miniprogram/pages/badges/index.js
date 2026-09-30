const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    emptyMessage: "Keep learning to earn badges.",
    badges: [],
    obtainedCount: 0
  },
  onLoad() {
    this.loadBadges();
  },
  onShareAppMessage() {
    return {
      title: "My Starline Badges",
      path: "/pages/badges/index"
    };
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
        emptyMessage: error.message || "Failed to load",
        loading: false
      }));
  }
});
