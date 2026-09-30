const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    emptyMessage: "Find your saved materials and exercises here.",
    favorites: []
  },
  onLoad() {
    this.loadFavorites();
  },
  onShareAppMessage() {
    return {
      title: "My Starline Favorites",
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
        emptyMessage: error.message || "Failed to load",
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
        wx.showToast({ title: "Removed from favorites", icon: "none" });
        this.setData({ favorites: this.data.favorites.filter((item) => item.id !== id) });
      })
      .catch(() => {});
  }
});
