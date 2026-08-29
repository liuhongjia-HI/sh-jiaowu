const { request } = require("../../utils/request");

const PAGE_BATCH_SIZE = 3;

Page({
  data: {
    title: "课件阅读",
    mode: "loading",
    message: "正在打开课件…",
    pages: [],
    pageCount: 0,
    nextPage: 1,
    hasMore: false,
    loadingMore: false
  },
  onLoad(options) {
    const id = options.id || "";
    if (!id) {
      this.setData({ mode: "error", message: "课件信息缺失，请返回后重试" });
      return;
    }
    this.materialId = id;
    let title = "课件阅读";
    try {
      title = decodeURIComponent(options.title || "") || title;
    } catch (_) {}
    this.setData({ title });
    if (wx.setNavigationBarTitle) wx.setNavigationBarTitle({ title });
    this.loadMetadata();
  },
  loadMetadata() {
    if (!this.materialId) return Promise.resolve();
    this.setData({
      mode: "loading",
      message: "正在打开课件…",
      pages: [],
      pageCount: 0,
      nextPage: 1,
      hasMore: false,
      loadingMore: false
    });
    return request(`/student/materials/${this.materialId}/preview/pages`)
      .then((info) => {
        if (!info || !info.imageMode || !info.pageCount) {
          this.setData({
            mode: "error",
            message: (info && info.message) || "课件页面正在生成，请稍后重试"
          });
          return;
        }
        this.setData({
          mode: "ready",
          message: "",
          pageCount: info.pageCount,
          nextPage: 1,
          hasMore: true
        });
        return this.loadNextBatch();
      })
      .catch((error) => {
        this.setData({ mode: "error", message: error.message || "课件打开失败，请稍后重试" });
      });
  },
  loadNextBatch() {
    if (this.data.loadingMore || !this.data.hasMore || !this.materialId) return Promise.resolve();
    const start = this.data.nextPage;
    const end = Math.min(this.data.pageCount, start + PAGE_BATCH_SIZE - 1);
    const pageNumbers = [];
    for (let page = start; page <= end; page += 1) pageNumbers.push(page);
    this.setData({ loadingMore: true, message: "" });
    return Promise.all(pageNumbers.map((page) => (
      downloadPage(this.materialId, page).then((path) => ({ page, path }))
    ))).then((loadedPages) => {
      const nextPage = end + 1;
      this.setData({
        pages: this.data.pages.concat(loadedPages),
        nextPage,
        hasMore: nextPage <= this.data.pageCount,
        loadingMore: false
      });
    }).catch((error) => {
      this.setData({
        loadingMore: false,
        message: error.message || "部分课件页面加载失败，请重试"
      });
    });
  },
  onReachBottom() {
    this.loadNextBatch();
  },
  retry() {
    if (this.data.mode === "error") {
      this.loadMetadata();
      return;
    }
    this.loadNextBatch();
  },
  previewPage(event) {
    if (!wx.previewImage) return;
    const index = Number(event.currentTarget.dataset.index || 0);
    const urls = this.data.pages.map((item) => item.path);
    if (!urls.length) return;
    wx.previewImage({ current: urls[index] || urls[0], urls });
  }
});

function downloadPage(materialId, page) {
  const app = getApp();
  return new Promise((resolve, reject) => {
    wx.downloadFile({
      url: `${app.globalData.apiBaseUrl}/student/materials/${materialId}/preview/pages/${page}`,
      header: {
        Authorization: wx.getStorageSync("starline_token") ? `Bearer ${wx.getStorageSync("starline_token")}` : ""
      },
      success(response) {
        if (response.statusCode !== 200) {
          reject(new Error(`第 ${page} 页加载失败`));
          return;
        }
        resolve(response.tempFilePath);
      },
      fail(error) {
        reject(new Error((error && error.errMsg) || `第 ${page} 页加载失败`));
      }
    });
  });
}
