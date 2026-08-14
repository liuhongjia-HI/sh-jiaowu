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
    favoriteId: "",
    // previewMode: unknown 加载中 / image 分页图片模式(服务端已烧录逐页水印) / pdf 退回整份安全预览
    previewMode: "unknown",
    pageCount: 0,
    pageImages: [],
    loadedPageCount: 0,
    pagesLoading: false,
    recordingBlocked: false
  },
  onLoad(options) {
    const id = options.id || "";
    if (!id) {
      this.setData({ pageTitle: "资料信息缺失" });
      return;
    }
    this.materialId = id;
    this.pageLoadToken = 0;
    this.stopContentSecurity = activateContentSecurity({
      targetType: "material",
      targetId: id,
      pagePath: "pages/material-preview/index",
      onRecordingChange: (isRecording) => this.setData({ recordingBlocked: isRecording })
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
      this.loadPagedPreview(id);
    }).catch(() => {
      this.setData({
        pageTitle: "资料加载失败",
        securityNotice: "资料加载失败，请重新进入。"
      });
    });
    this.refreshFavorite(id);
  },
  onShareAppMessage() {
    return {
      title: this.data.pageTitle && this.data.pageTitle !== "资料预览" ? `Starline 学习资料：${this.data.pageTitle}` : "Starline 学习资料",
      path: this.materialId ? `/pages/material-preview/index?id=${encodeURIComponent(this.materialId)}` : "/pages/study/index"
    };
  },
  onUnload() {
    this.pageLoadToken += 1;
    if (this.stopContentSecurity) {
      this.stopContentSecurity();
      this.stopContentSecurity = null;
    }
  },
  // loadPagedPreview 尝试用分页图片模式加载资料：每一页是服务端把当前学生水印烧录
  // 进像素后栅格化出来的 JPEG，水印无法通过复制文本或去掉图层剥离。
  // 服务器没装 Ghostscript（本地开发环境常见）时接口会返回 imageMode=false，
  // 这时保留原来的“安全预览”整份 PDF 按钮作为兜底，不影响正常使用。
  loadPagedPreview(id) {
    request(`/student/materials/${id}/preview/pages`).then((info) => {
      if (!info || !info.imageMode || !info.pageCount) {
        this.setData({ previewMode: "pdf" });
        return;
      }
      const token = ++this.pageLoadToken;
      this.setData({
        previewMode: "image",
        pageCount: info.pageCount,
        pageImages: [],
        loadedPageCount: 0,
        pagesLoading: true
      });
      this.loadNextPage(id, token, 1, info.pageCount);
    }).catch(() => {
      this.setData({ previewMode: "pdf" });
    });
  },
  loadNextPage(id, token, page, total) {
    if (token !== this.pageLoadToken) {
      return;
    }
    if (page > total) {
      this.setData({ pagesLoading: false });
      return;
    }
    downloadWithAuth(`/student/materials/${id}/preview/pages/${page}`)
      .then((tempFilePath) => {
        if (token !== this.pageLoadToken) {
          return;
        }
        const pageImages = this.data.pageImages.concat([{ index: page, path: tempFilePath }]);
        this.setData({ pageImages, loadedPageCount: page });
        this.loadNextPage(id, token, page + 1, total);
      })
      .catch(() => {
        if (token !== this.pageLoadToken) {
          return;
        }
        // 某一页加载失败就整体退回整份 PDF 安全预览，保证学生始终能看到内容，
        // 不会卡在“加载中”。
        this.setData({ previewMode: "pdf", pagesLoading: false, pageImages: [] });
      });
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
    wx.showLoading({ title: "打开资料中" });
    downloadWithAuth(stripApiPrefix(previewUrl))
      .then((tempFilePath) => {
        wx.openDocument({
          filePath: tempFilePath,
          fileType: "pdf",
          fail() {
            wx.showToast({ title: "资料打开失败，请稍后再试", icon: "none" });
          }
        });
      })
      .catch(() => {
        wx.showToast({ title: "资料打开失败，请稍后再试", icon: "none" });
      })
      .then(() => wx.hideLoading());
  }
});

function buildWatermarks(text) {
  return Array.from({ length: 8 }).map(() => text);
}

// downloadWithAuth 用 wx.downloadFile 带上登录态下载一份需要鉴权的文件（图片/PDF）。
// path 是不带 /api 前缀的接口路径，例如 "/student/materials/xxx/preview"，
// 与 utils/request.js 里 wx.request 的调用约定保持一致。
function downloadWithAuth(path) {
  const app = getApp();
  return new Promise((resolve, reject) => {
    wx.downloadFile({
      url: `${app.globalData.apiBaseUrl}${path}`,
      header: {
        Authorization: wx.getStorageSync("starline_token") ? `Bearer ${wx.getStorageSync("starline_token")}` : ""
      },
      success(res) {
        if (res.statusCode !== 200) {
          reject(new Error("下载失败"));
          return;
        }
        resolve(res.tempFilePath);
      },
      fail(err) {
        reject(err);
      }
    });
  });
}

// stripApiPrefix 去掉后端接口返回字段里多余的 "/api" 前缀。
// apiBaseUrl 本身已经以 /api 结尾（见 app.js），后端 Material.PreviewURL /
// DownloadURL 这类现成字段还各自带了一份 "/api/..."，直接拼接会变成
// "https://.../api/api/student/..."，在生产环境会 404。Web 管理端的
// openFile() 早就用同样的 replace(/^\/api/, '') 方式绕过了这个问题，
// 小程序这边此前一直没处理，是一个独立的既有 bug。
function stripApiPrefix(path) {
  return String(path || "").replace(/^\/api/, "");
}
