const { request } = require("../../utils/request");
const { activateContentSecurity } = require("../../utils/content-security");

const TAG_DEFINITIONS = [
  { code: "ALL", label: "All", shortLabel: "All" },
  { code: "HD", label: "Notes", shortLabel: "Notes" },
  { code: "Blank", label: "Blank", shortLabel: "Blank" },
  { code: "HW", label: "Homework", shortLabel: "Homework" },
  { code: "Exam", label: "Exam", shortLabel: "Exam" },
  { code: "Special", label: "Special", shortLabel: "Special" }
];

Page({
  data: {
    material: {},
    activeHomework: {},
    contentMode: "material",
    tags: TAG_DEFINITIONS.map((item) => ({ ...item, count: 0 })),
    activeTag: "ALL",
    activeTagLabel: "All",
    contentTagLabel: "Notes",
    tagItems: [],
    lessonTitle: "",
    pageTitle: "资料预览",
    displayTitle: "资料预览",
    materialCode: "",
    paperTitle: "",
    securityNotice: "仅供本人学习，请勿分享、截图或录屏。",
    watermarkText: "水印加载中",
    watermarkTexts: ["水印加载中", "水印加载中", "水印加载中", "水印加载中", "水印加载中", "水印加载中", "水印加载中", "水印加载中", "水印加载中", "水印加载中"],
    favorited: false,
    favoriteId: "",
    // previewMode: unknown 加载中 / processing 生成中 / image 首图预览 / pdf 无缩略图降级 / cover-error 首图失败 / unavailable 不可用
    previewMode: "unknown",
    pageCount: 0,
    previewImagePath: "",
    pagesLoading: false,
    previewMessage: "",
    openingPreview: false,
    downloading: false,
    recordingWarning: false
  },
  onLoad(options) {
    const id = options.id || "";
    this.courseId = options.courseId || "";
    this.lessonId = options.lessonId || "";
    this.pageLoadToken = 0;
    this.previewRetryCount = 0;
    if (!id && (!this.courseId || !this.lessonId)) {
      this.setData({ pageTitle: "课节信息缺失", displayTitle: "课节信息缺失", contentMode: "empty" });
      return;
    }
    if (!id) {
      this.setData({ pageTitle: "课节内容", displayTitle: "课节内容", contentMode: "empty" });
      this.loadLessonContents();
      return;
    }
    this.loadMaterial(id, true);
  },
  loadMaterial(id, loadLessonContents) {
    this.materialId = id;
    this.resetContentSecurity(id, "material");
    this.setData({
      contentMode: "material",
      activeHomework: {},
      previewMode: "unknown",
      previewMessage: "",
      previewImagePath: "",
      pagesLoading: false,
      favorited: false,
      favoriteId: ""
    });
    request(`/student/materials/${id}`).then((material) => {
      this.courseId = this.courseId || material.courseId || "";
      this.lessonId = this.lessonId || material.lessonId || "";
      const lessonTitle = (material.curriculum && material.curriculum.lesson) || this.data.lessonTitle || material.title;
      const header = buildDisplayHeader(material, lessonTitle);
      this.setData({
        material,
        pageTitle: header.displayTitle,
        displayTitle: header.displayTitle,
        materialCode: header.materialCode,
        paperTitle: header.displayTitle,
        lessonTitle: header.displayTitle,
        contentTagLabel: tagLabel(normalizeTagCode(material.tagCode) || "HD"),
        watermarkText: material.watermarkText || "水印加载中",
        watermarkTexts: buildWatermarks(material.watermarkText || "水印加载中"),
        securityNotice: material.securityNotice || "仅供本人学习，请勿分享、截图或录屏。"
      });
      this.loadPagedPreview(id);
      if (loadLessonContents) this.loadLessonContents();
    }).catch(() => {
      this.setData({
        pageTitle: "资料加载失败",
        displayTitle: "资料加载失败",
        securityNotice: "资料加载失败，请重新进入。",
        previewMode: "unavailable",
        previewMessage: "资料加载失败，请重新进入"
      });
    });
    this.refreshFavorite(id);
  },
  resetContentSecurity(id, targetType) {
    if (this.stopContentSecurity) this.stopContentSecurity();
    this.stopContentSecurity = activateContentSecurity({
      targetType: targetType || "material",
      targetId: id,
      pagePath: "pages/material-preview/index",
      onRecordingChange: (isRecording) => this.setData({ recordingWarning: isRecording })
    });
  },
  loadLessonContents() {
    if (!this.courseId || !this.lessonId) return;
    request(`/student/study/${this.courseId}`).then((detail) => {
      const courseMaterials = detail.materials || [];
      const courseHomework = detail.homework || [];
      // 从课程目录进入时，只展示当前课节的讲义和练习，避免串到其他章节。
      const contents = [
        ...courseMaterials.map((item) => ({ ...item, contentType: "material", tagCode: normalizeTagCode(item.tagCode) || "HD", displayName: prettyContentTitle(item.title) })),
        ...courseHomework.map((item) => ({ ...item, contentType: "homework", tagCode: normalizeTagCode(item.tagCode) || "HW", displayName: prettyContentTitle(item.title) }))
      ].filter((item) => item.lessonId === this.lessonId);
      const lesson = ((detail.course && detail.course.curriculum) || []).find((node) => node.id === this.lessonId);
      this.lessonContents = contents;
      const tags = TAG_DEFINITIONS.map((tag) => ({ ...tag, count: countForTag(contents, tag) }));
      const resolvedLessonTitle = prettyContentTitle((lesson && lesson.name) || this.data.lessonTitle);
      this.setData({
        tags,
        activeTag: "ALL",
        activeTagLabel: "All",
        lessonTitle: resolvedLessonTitle || this.data.lessonTitle,
        displayTitle: resolvedLessonTitle || this.data.displayTitle,
        pageTitle: resolvedLessonTitle || this.data.pageTitle
      });
      this.showTagContents("ALL", true);
    }).catch(() => {});
  },
  selectTag(event) {
    this.showTagContents(event.currentTarget.dataset.code || "ALL", false, true);
  },
  selectTagItem(event) {
    const item = (this.lessonContents || []).find((content) => content.id === event.currentTarget.dataset.id);
    if (item) this.showContent(item);
  },
  showTagContents(code, preserveCurrent, listOnly) {
    const tagCode = normalizeFilterTag(code);
    const items = itemsForTag(this.lessonContents || [], tagCode);
    this.setData({ activeTag: tagCode, activeTagLabel: tagLabel(tagCode), tagItems: items });
    if (!items.length) {
      this.pageLoadToken += 1;
      if (this.stopContentSecurity) {
        this.stopContentSecurity();
        this.stopContentSecurity = null;
      }
      this.setData({ contentMode: "empty", activeHomework: {}, materialCode: "" });
      return;
    }
    if (listOnly) {
      this.pageLoadToken += 1;
      if (this.stopContentSecurity) {
        this.stopContentSecurity();
        this.stopContentSecurity = null;
      }
      this.setData({ contentMode: "list", activeHomework: {}, materialCode: "" });
      return;
    }
    const current = preserveCurrent && items.find((item) => item.contentType === "material" && item.id === this.materialId);
    this.showContent(current || items[0]);
  },
  showContent(item) {
    if (item.contentType === "homework") {
      this.pageLoadToken += 1;
      this.resetContentSecurity(item.id, "homework");
      this.setData({ contentMode: "homework", activeHomework: item, materialCode: "", contentTagLabel: tagLabel(item.tagCode || "HW") });
      return;
    }
    if (item.id === this.materialId && this.data.contentMode === "material") return;
    this.loadMaterial(item.id, false);
  },
  onShareAppMessage() {
    const contextQuery = this.courseId && this.lessonId ? `&courseId=${encodeURIComponent(this.courseId)}&lessonId=${encodeURIComponent(this.lessonId)}` : "";
    return {
      title: this.data.displayTitle && this.data.displayTitle !== "资料预览" ? `Starline 课节：${this.data.displayTitle}` : "Starline 课节内容",
      path: this.materialId
        ? `/pages/material-preview/index?id=${encodeURIComponent(this.materialId)}${contextQuery}`
        : (this.courseId && this.lessonId ? `/pages/material-preview/index?courseId=${encodeURIComponent(this.courseId)}&lessonId=${encodeURIComponent(this.lessonId)}` : "/pages/study/index")
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
  printMaterial() {
    if (this.data.downloading) return;
    const downloadUrl = this.data.material && this.data.material.downloadUrl;
    if (!downloadUrl) {
      wx.showToast({ title: "暂未开通下载打印权限", icon: "none" });
      return;
    }
    this.setData({ downloading: true });
    wx.showLoading({ title: "正在下载课件" });
    return downloadWithAuth(stripApiPrefix(downloadUrl))
      .then((tempFilePath) => openDocument(tempFilePath))
      .catch((error) => showFileError("课件无法打开", error))
      .finally(() => {
        this.setData({ downloading: false });
        wx.hideLoading();
      });
  },
  // 分页图片在上传后由服务端预生成；详情页只下载第一页作为预览，完整内容交给文档查看器。
  // 缩略图不可用时保留整份 PDF 入口，避免模拟内容冒充真实预览。
  loadPagedPreview(id) {
    request(`/student/materials/${id}/preview/pages`).then((info) => {
      const previewStatus = info && info.previewStatus;
      if (previewStatus === "processing") {
        this.setData({
          previewMode: "processing",
          previewMessage: info.message || "课件正在生成，请稍后再试",
          pagesLoading: false
        });
        this.schedulePreviewRetry(id);
        return;
      }
      if (previewStatus === "failed" || previewStatus === "unavailable") {
        this.setData({
          previewMode: "unavailable",
          previewMessage: info.message || "课件暂时无法打开",
          pagesLoading: false
        });
        return;
      }
      if (!info || !info.imageMode || !info.pageCount) {
        this.setData({
          previewMode: "pdf",
          previewMessage: (info && info.message) || "暂未生成缩略图，点击打开完整课件",
          pageCount: (info && info.pageCount) || 0,
          pagesLoading: false
        });
        return;
      }
      const token = ++this.pageLoadToken;
      this.setData({
        previewMode: "image",
        previewMessage: "",
        pageCount: info.pageCount,
        previewImagePath: "",
        pagesLoading: true
      });
      this.loadPreviewCover(id, token);
    }).catch((error) => {
      const message = error.message || "课件暂时无法打开";
      this.setData({ previewMode: "unavailable", previewMessage: message, pagesLoading: false });
      if (message.includes("正在生成") && this.previewRetryCount < 3) {
        this.schedulePreviewRetry(id);
      }
    });
  },
  schedulePreviewRetry(id) {
    if (this.previewRetryCount >= 3 || this.previewRetryTimer) return;
    this.previewRetryCount += 1;
    this.previewRetryTimer = setTimeout(() => {
      this.previewRetryTimer = null;
      this.loadPagedPreview(id);
    }, 3000);
  },
  retryPreview() {
    if (!this.materialId) return;
    if (this.previewRetryTimer) {
      clearTimeout(this.previewRetryTimer);
      this.previewRetryTimer = null;
    }
    this.pageLoadToken += 1;
    this.previewRetryCount = 0;
    this.setData({ previewMode: "unknown", previewMessage: "", previewImagePath: "", pagesLoading: false });
    this.loadPagedPreview(this.materialId);
  },
  loadPreviewCover(id, token) {
    downloadWithAuth(`/student/materials/${id}/preview/pages/1`)
      .then((tempFilePath) => {
        if (token !== this.pageLoadToken) return;
        this.setData({ previewImagePath: tempFilePath, pagesLoading: false });
      })
      .catch((error) => {
        if (token !== this.pageLoadToken) return;
        this.setData({
          previewMode: "cover-error",
          previewMessage: error.message || "课件缩略图加载失败",
          pagesLoading: false
        });
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
    if (this.data.activeHomework && this.data.activeHomework.id) {
      wx.navigateTo({ url: `/pages/answer/index?id=${this.data.activeHomework.id}` });
      return;
    }
    const material = this.data.material || {};
    const courseId = material.courseId || "";
    if (!courseId) {
      wx.navigateTo({ url: "/pages/tasks/index" });
      return;
    }

    request("/student/tasks").then((tasks) => {
      const courseTasks = (tasks || []).filter((task) => task.courseId === courseId);
      const lessonId = material.lessonId || "";
      const selected = courseTasks.find((task) => task.lessonId === lessonId);
      if (selected) {
        wx.navigateTo({ url: `/pages/answer/index?id=${selected.id}` });
        return;
      }
      wx.showToast({ title: "本课节暂无练习", icon: "none" });
      wx.navigateTo({ url: "/pages/tasks/index" });
    }).catch(() => {
      wx.showToast({ title: "练习加载失败，请稍后重试", icon: "none" });
      wx.navigateTo({ url: "/pages/tasks/index" });
    });
  },
  openSecurePreview() {
    if (this.data.openingPreview) return;
    const previewUrl = this.data.material.previewUrl;
    if (!previewUrl) {
      wx.showToast({ title: "完整课件还在准备，请稍后再试", icon: "none" });
      return;
    }
    this.setData({ openingPreview: true });
    wx.showLoading({ title: "正在打开课件" });
    downloadWithAuth(stripApiPrefix(previewUrl))
      .then((tempFilePath) => openDocument(tempFilePath, Boolean(this.data.material.downloadUrl)))
      .catch((error) => {
        showFileError("课件无法打开", error);
      })
      .finally(() => {
        this.setData({ openingPreview: false });
        wx.hideLoading();
      });
  }
});


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

function openDocument(filePath, showMenu = true) {
  return new Promise((resolve, reject) => {
    wx.openDocument({
      filePath,
      fileType: "pdf",
      // 打开右上角文档菜单，客户可从菜单转发到文件传输助手或选择支持打印的应用。
      // 小程序无法强制指定系统浏览器，showMenu 是微信侧可用的兼容入口。
      showMenu,
      success: resolve,
      fail(error) {
        reject(new Error((error && error.errMsg) || "资料打开失败，请稍后再试"));
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

function buildWatermarks(text) {
  return Array.from({ length: 10 }).map(() => text);
}

function normalizeTagCode(code) {
  const value = String(code || "").trim();
  if (value.toUpperCase() === "EXAM") return "Exam";
  if (value.toUpperCase() === "SPECIAL") return "Special";
  if (value.toUpperCase() === "BLANK") return "Blank";
  if (value.toUpperCase() === "HD") return "HD";
  if (value.toUpperCase() === "HW") return "HW";
  return "";
}

function normalizeFilterTag(code) {
  if (String(code || "").toUpperCase() === "ALL") return "ALL";
  return normalizeTagCode(code) || "ALL";
}

function tagLabel(code) {
  return (TAG_DEFINITIONS.find((tag) => tag.code === code) || {}).label || code;
}

function itemsForTag(contents, code) {
  if (code === "ALL") return (contents || []).slice();
  return (contents || []).filter((item) => item.tagCode === code);
}

function countForTag(contents, tag) {
  if (tag.code === "ALL") return (contents || []).length;
  return (contents || []).filter((item) => item.tagCode === tag.code).length;
}

function splitMaterialTitle(title) {
  const raw = String(title || "").trim();
  const matched = raw.match(/^((?:HD|HW|Blank|Exam|Special)[_-][A-Za-z0-9._-]+)\s+(.+)$/i);
  if (matched) return { code: matched[1], name: matched[2] };
  return { code: "", name: raw };
}

function prettyContentTitle(title) {
  const split = splitMaterialTitle(title);
  return split.name || String(title || "").trim();
}

function buildDisplayHeader(material, lessonTitle) {
  const split = splitMaterialTitle(material && material.title);
  const lesson = prettyContentTitle(lessonTitle);
  return {
    displayTitle: lesson || split.name || "课节内容",
    materialCode: split.code
  };
}
