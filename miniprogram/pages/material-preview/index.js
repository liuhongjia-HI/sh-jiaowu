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
    securityNotice: "这份资料仅供你本人学习，已添加专属水印。请不要分享、截图或录屏。",
    favorited: false,
    favoriteId: "",
    // previewMode: unknown 加载中 / image 上传后预生成分页图片 / pdf 整份安全预览
    previewMode: "unknown",
    pageCount: 0,
    pageImages: [],
    loadedPageCount: 0,
    pagesLoading: false,
    previewMessage: "",
    recordingWarning: false
  },
  onLoad(options) {
    const id = options.id || "";
    if (!id) {
      this.setData({ pageTitle: "资料信息缺失" });
      return;
    }
    this.materialId = id;
    this.pageLoadToken = 0;
    this.previewRetryCount = 0;
    this.stopContentSecurity = activateContentSecurity({
      targetType: "material",
      targetId: id,
      pagePath: "pages/material-preview/index",
      onRecordingChange: (isRecording) => this.setData({ recordingWarning: isRecording })
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
        securityNotice: material.securityNotice || "这份资料仅供你本人学习，已添加专属水印。请不要分享、截图或录屏。"
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
      title: this.data.pageTitle && this.data.pageTitle !== "资料预览" ? `Starline 课程讲义：${this.data.pageTitle}` : "Starline 课程讲义",
      path: this.materialId ? `/pages/material-preview/index?id=${encodeURIComponent(this.materialId)}` : "/pages/study/index"
    };
  },
  onUnload() {
    this.pageLoadToken += 1;
    if (this.previewRetryTimer) {
      clearTimeout(this.previewRetryTimer);
      this.previewRetryTimer = null;
    }
    if (this.stopContentSecurity) {
      this.stopContentSecurity();
      this.stopContentSecurity = null;
    }
  },
  downloadMaterial() {
    const downloadUrl = this.data.material && this.data.material.downloadUrl;
    if (!downloadUrl) {
      wx.showToast({ title: "当前不在课件开放期", icon: "none" });
      return;
    }
    wx.showLoading({ title: "正在下载" });
    downloadWithAuth(stripApiPrefix(downloadUrl)).then((tempFilePath) => new Promise((resolve, reject) => {
      wx.saveFile({ tempFilePath, success: resolve, fail: reject });
    })).then(() => {
      wx.showToast({ title: "已保存课件", icon: "success" });
    }).catch((error) => {
      showFileError("课件下载失败", error);
    }).finally(() => wx.hideLoading());
  },
  // 分页图片在上传后由服务端预生成，学生专属水印由当前页面覆盖显示。
  // 图片模式不可用时保留整份 PDF 按钮，单页失败则留在原位重试，不清空已加载页面。
  loadPagedPreview(id) {
    request(`/student/materials/${id}/preview/pages`).then((info) => {
      if (!info || !info.imageMode || !info.pageCount) {
        this.setData({ previewMode: "pdf" });
        return;
      }
      const token = ++this.pageLoadToken;
      const pageImages = Array.from({ length: info.pageCount }).map((_, index) => ({
        index: index + 1,
        path: "",
        status: "pending",
        error: ""
      }));
      this.setData({
        previewMode: "image",
        previewMessage: "",
        pageCount: info.pageCount,
        pageImages,
        loadedPageCount: 0,
        pagesLoading: true
      });
      this.loadNextPage(id, token, 1, info.pageCount);
    }).catch((error) => {
      const message = error.message || "课件暂时无法打开";
      this.setData({ previewMode: "unavailable", previewMessage: message, pagesLoading: false });
      if (message.includes("正在生成") && this.previewRetryCount < 3) {
        this.previewRetryCount += 1;
        this.previewRetryTimer = setTimeout(() => this.loadPagedPreview(id), 3000);
      }
    });
  },
  retryPreview() {
    if (!this.materialId) return;
    this.previewRetryCount = 0;
    this.setData({ previewMode: "unknown", previewMessage: "" });
    this.loadPagedPreview(this.materialId);
  },
  loadNextPage(id, token, page, total) {
    if (token !== this.pageLoadToken) {
      return;
    }
    if (page > total) {
      this.setData({ pagesLoading: false });
      return;
    }
    this.updatePage(page, { status: "loading", error: "" });
    downloadWithAuth(`/student/materials/${id}/preview/pages/${page}`)
      .then((tempFilePath) => {
        if (token !== this.pageLoadToken) {
          return;
        }
        this.updatePage(page, { path: tempFilePath, status: "ready", error: "" });
        this.setData({ loadedPageCount: this.data.loadedPageCount + 1 });
        this.loadNextPage(id, token, page + 1, total);
      })
      .catch((error) => {
        if (token !== this.pageLoadToken) {
          return;
        }
        this.updatePage(page, { status: "error", error: error.message || "本页加载失败" });
        this.loadNextPage(id, token, page + 1, total);
      });
  },
  updatePage(page, patch) {
    const pageImages = this.data.pageImages.map((item) => item.index === page ? { ...item, ...patch } : item);
    this.setData({ pageImages });
  },
  retryPage(event) {
    const page = Number(event.currentTarget.dataset.page);
    if (!page || !this.materialId) {
      return;
    }
    this.updatePage(page, { status: "loading", error: "" });
    downloadWithAuth(`/student/materials/${this.materialId}/preview/pages/${page}`)
      .then((tempFilePath) => {
        this.updatePage(page, { path: tempFilePath, status: "ready", error: "" });
        this.setData({ loadedPageCount: this.data.loadedPageCount + 1 });
      })
      .catch((error) => this.updatePage(page, { status: "error", error: error.message || "本页加载失败" }));
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
      wx.showToast({ title: "完整课件还在准备，请稍后再试", icon: "none" });
      return;
    }
    wx.showLoading({ title: "正在打开课件" });
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
      .catch((error) => {
        showFileError("课件无法打开", error);
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
          readDownloadErrorMessage(res).then((message) => reject(new Error(message || `课件请求失败（${res.statusCode}）`)));
          return;
        }
        resolve(res.tempFilePath);
      },
      fail(err) {
        reject(new Error((err && err.errMsg) || "网络下载失败"));
      }
    });
  });
}

function readDownloadErrorMessage(response) {
  if (response && response.data && typeof response.data === "object") {
    return Promise.resolve(response.data.message || "");
  }
  if (!response || !response.tempFilePath || !wx.getFileSystemManager) {
    return Promise.resolve("");
  }
  return new Promise((resolve) => {
    wx.getFileSystemManager().readFile({
      filePath: response.tempFilePath,
      encoding: "utf8",
      success(result) {
        try {
          const body = JSON.parse(result.data || "{}");
          resolve(body.message || "");
        } catch (_) {
          resolve("");
        }
      },
      fail() { resolve(""); }
    });
  });
}

function showFileError(title, error) {
  const content = (error && error.message) || "请稍后重试";
  if (wx.showModal) {
    wx.showModal({ title, content, showCancel: false, confirmText: "知道了" });
    return;
  }
  wx.showToast({ title: content, icon: "none" });
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
