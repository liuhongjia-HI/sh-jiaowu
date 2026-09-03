const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    emptyMessage: "收藏讲义或练习后，可以在这里找到。",
    favorites: []
  },
  onLoad() {
    this.loadFavorites();
  },
  onShareAppMessage() {
    return {
      title: "我的 Starline 学习收藏",
      path: "/pages/favorites/index"
    };
  },
  onShow() {
    if (!this.data.loading) {
      this.loadFavorites();
    }
  },
  loadFavorites() {
    this.setData({ loading: true });
    request("/student/favorites")
      .then((favorites) => this.setData({ favorites: favorites || [], loading: false }))
      .catch((error) => this.setData({
        emptyMessage: error.message || "加载失败",
        loading: false
      }));
  },
  openFavorite(event) {
    const { type, target } = event.currentTarget.dataset;
    if (type === "material") {
      wx.navigateTo({ url: `/pages/material-preview/index?id=${target}` });
      return;
    }
    if (type === "homework") {
      wx.navigateTo({ url: `/pages/answer/index?id=${target}` });
    }
  },
  removeFavorite(event) {
    const id = event.currentTarget.dataset.id;
    request(`/student/favorites/${id}`, { method: "DELETE" })
      .then(() => {
        wx.showToast({ title: "已取消收藏", icon: "none" });
        this.setData({ favorites: this.data.favorites.filter((item) => item.id !== id) });
      })
      .catch(() => {});
  }
});
