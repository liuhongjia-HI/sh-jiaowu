const { request } = require("../../utils/request");

Page({
  data: {
    material: {},
    pageTitle: "资料预览",
    paperTitle: "",
    readText: "",
    favorited: false,
    favoriteId: ""
  },
  onLoad(options) {
    const id = options.id || "";
    if (!id) {
      this.setData({ pageTitle: "资料信息缺失" });
      return;
    }
    this.materialId = id;
    request(`/student/materials/${id}`).then((material) => {
      this.setData({
        material,
        pageTitle: material.title,
        paperTitle: material.chapter || material.title,
        readText: `${material.viewCount || 0} 人学过`
      });
    });
    this.refreshFavorite(id);
  },
  refreshFavorite(materialId) {
    request("/student/favorites").then((favorites) => {
      const matched = (favorites || []).find(
        (item) => item.targetType === "material" && item.targetId === materialId
      );
      this.setData({ favorited: !!matched, favoriteId: matched ? matched.id : "" });
    }).catch(() => {});
  },
  toggleFavorite() {
    if (!this.materialId) {
      return;
    }
    if (this.data.favorited && this.data.favoriteId) {
      request(`/student/favorites/${this.data.favoriteId}`, { method: "DELETE" })
        .then(() => {
          wx.showToast({ title: "已取消收藏", icon: "none" });
          this.setData({ favorited: false, favoriteId: "" });
        })
        .catch(() => {});
      return;
    }
    request("/student/favorites", {
      method: "POST",
      data: { targetType: "material", targetId: this.materialId }
    })
      .then((favorite) => {
        wx.showToast({ title: "已收藏", icon: "success" });
        this.setData({ favorited: true, favoriteId: favorite.id });
      })
      .catch(() => {});
  },
  goAnswer() {
    // 通过资料所属课程进入课程详情，选择对应的小挑战
    if (this.data.material.courseId) {
      wx.navigateTo({ url: `/pages/study-detail/index?id=${this.data.material.courseId}` });
      return;
    }
    wx.navigateBack({ delta: 1, fail() {} });
  }
});
