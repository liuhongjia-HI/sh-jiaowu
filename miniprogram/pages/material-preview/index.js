const { request } = require("../../utils/request");
const { activateContentSecurity } = require("../../utils/content-security");

Page({
  data: {
    material: {},
    pageTitle: "资料预览",
    paperTitle: "",
    readText: "",
    watermarkText: "专属水印加载中",
    watermarkTexts: ["专属水印加载中", "专属水印加载中", "专属水印加载中", "专属水印加载中", "专属水印加载中", "专属水印加载中"],
    securityNotice: "学习内容仅限本人查看，请勿外传。",
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
    this.stopContentSecurity = activateContentSecurity({
      targetType: "material",
      targetId: id,
      pagePath: "pages/material-preview/index"
    });
    request(`/student/materials/${id}`).then((material) => {
      const watermarkText = material.watermarkText || "专属水印加载中";
      this.setData({
        material,
        pageTitle: material.title,
        paperTitle: material.chapter || material.title,
        readText: `${material.viewCount || 0} 人学过`,
        watermarkText,
        watermarkTexts: buildWatermarks(watermarkText),
        securityNotice: material.securityNotice || "学习内容仅限本人查看，请勿外传。"
      });
    }).catch(() => {
      this.setData({
        pageTitle: "资料加载失败",
        securityNotice: "资料加载失败，请重新进入。"
      });
    });
    this.refreshFavorite(id);
  },
  onUnload() {
    if (this.stopContentSecurity) {
      this.stopContentSecurity();
      this.stopContentSecurity = null;
    }
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
  },
  openSecurePreview() {
    const previewUrl = this.data.material.previewUrl;
    if (!previewUrl) {
      wx.showToast({ title: "资料正在生成安全预览，请稍后再试", icon: "none" });
      return;
    }
    const app = getApp();
    wx.showLoading({ title: "打开资料中" });
    wx.downloadFile({
      url: `${app.globalData.apiBaseUrl}${previewUrl}`,
      header: {
        Authorization: wx.getStorageSync("starline_token") ? `Bearer ${wx.getStorageSync("starline_token")}` : ""
      },
      success(res) {
        if (res.statusCode !== 200) {
          wx.showToast({ title: "资料正在生成安全预览，请稍后再试", icon: "none" });
          return;
        }
        wx.openDocument({
          filePath: res.tempFilePath,
          fileType: "pdf",
          fail() {
            wx.showToast({ title: "资料打开失败，请稍后再试", icon: "none" });
          }
        });
      },
      fail() {
        wx.showToast({ title: "资料打开失败，请稍后再试", icon: "none" });
      },
      complete() {
        wx.hideLoading();
      }
    });
  }
});

function buildWatermarks(text) {
  return Array.from({ length: 8 }).map(() => text);
}
