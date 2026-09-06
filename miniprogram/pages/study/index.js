const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "开通课程后，这里会显示学习内容。",
    keyword: "",
    activeFilter: "全部",
    filters: [
      { label: "全部", className: "active" },
      { label: "学习中", className: "" },
      { label: "已收藏", className: "" },
      { label: "已完成", className: "" }
    ],
    courses: [],
    subjects: [],
    visibleCourses: [],
    openedCourseCount: 0,
    materials: [],
    hasOpenedPackage: false,
    authRequired: false,
    loginPrompted: false
  },
  onLoad() {
    if (!hasStudentToken()) {
      this.promptLogin();
      return;
    }
    this.loadStudy();
  },
  onShareAppMessage() {
    return {
      title: "我的 Starline 学习内容",
      path: "/pages/study/index"
    };
  },
  onShow() {
    if (!hasStudentToken()) {
      if (!this.data.loginPrompted) {
        this.promptLogin();
      }
      return;
    }
    if (this.data.authRequired || this.data.loginPrompted) {
      this.setData({ authRequired: false, loginPrompted: false });
    }
    if (!this.data.loading) {
      this.loadStudy();
    }
  },
  promptLogin() {
    this.setData({
      loading: false,
      authRequired: true,
      loginPrompted: true,
      error: "",
      courses: [],
      subjects: [],
      visibleCourses: [],
      openedCourseCount: 0,
      materials: []
    });
  },
  goLogin() {
    wx.navigateTo({
      url: "/pages/login/index",
      fail() {
        if (wx.redirectTo) {
          wx.redirectTo({ url: "/pages/login/index" });
        }
      }
    });
  },
  loadStudy() {
    this.setData({ loading: true, error: "" });
    Promise.all([request("/student/study"), request("/student/favorites").catch(() => [])])
      .then(([data, favorites]) => {
        const courses = Array.isArray(data) ? data : (data.courses || []);
        const subjects = Array.isArray(data) ? [] : (data.subjects || []);
        const materials = Array.isArray(data) ? [] : (data.materials || []);
        const student = Array.isArray(data) ? {} : (data.student || {});
        const hasOpenedPackage = Array.isArray(student.openedPackages) && student.openedPackages.length > 0;
        const mergedCourses = mergeStudyCourses(subjects, courses, favorites || []);
        this.setData({
          courses: mergedCourses,
          openedCourseCount: mergedCourses.filter((course) => course.isOpened).length,
          subjects,
          materials,
          hasOpenedPackage,
          emptyMessage: studyEmptyMessage(hasOpenedPackage),
          loading: false
        }, () => this.applyFilters());
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "开通课程后，这里会显示学习内容。",
        hasOpenedPackage: false,
        openedCourseCount: 0,
        authRequired: !hasStudentToken(),
        loginPrompted: !hasStudentToken(),
        loading: false
      }));
  },
  changeKeyword(event) {
    this.setData({ keyword: event.detail.value }, () => this.applyFilters());
  },
  changeFilter(event) {
    const activeFilter = event.currentTarget.dataset.filter;
    this.setData({
      activeFilter,
      filters: this.data.filters.map((item) => ({ ...item, className: item.label === activeFilter ? "active" : "" }))
    }, () => this.applyFilters());
  },
  applyFilters() {
    const keyword = this.data.keyword.trim().toLowerCase();
    const activeFilter = this.data.activeFilter;
    const visibleCourses = this.data.courses.filter((course) => {
      const matchKeyword = !keyword || [course.name, course.displayName, course.subject, course.grade].join(" ").toLowerCase().includes(keyword);
      const matchFilter = activeFilter === "全部" || (activeFilter === "学习中" && course.status !== "已完成") || (activeFilter === "已收藏" && course.favorited) || (activeFilter === "已完成" && course.status === "已完成");
      return matchKeyword && matchFilter;
    });
    this.setData({ visibleCourses });
  },
  goDetail(event) {
    const dataset = event.currentTarget.dataset || {};
    const id = dataset.id || "";
    const course = this.data.courses.find((item) => item.entryCourseId === id || item.id === id);
    const canOpen = course ? course.canOpen : dataset.canOpen;
    if (!canOpen) {
      wx.showToast({ title: dataset.message || "开通后即可学习全部内容", icon: "none" });
      return;
    }
    if (!id) {
      wx.showToast({ title: "学习内容正在准备", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${id}` });
  }
});

function hasStudentToken() {
  return Boolean(wx.getStorageSync && wx.getStorageSync("starline_token"));
}

// decorateCourses 使用接口返回的真实进度，仅补充图标等展示字段。
function decorateCourses(courses, favorites) {
  const favoriteCourseNames = (favorites || []).map((item) => item.course).filter(Boolean);
  return [...courses].sort((left, right) => courseAvailableAt(right) - courseAvailableAt(left)).map((course, index) => {
    const progress = Number(course.progress) || 0;
    const isNew = Boolean(course.isNew);
    const isOpened = Boolean(course.isOpened);
    return {
      ...course,
      progress,
      favorited: favoriteCourseNames.includes(course.name),
      badgeText: course.accessLabel || (progress >= 80 ? "阅读小达人" : progress > 0 ? "继续加油" : "新内容"),
      cardClass: isNew ? "new-course" : progress >= 100 ? "reward" : isOpened ? "opened-course" : "",
      newCourseText: isNew && course.availableAt ? `新开通 · ${formatCourseTime(course.availableAt)}` : "",
      coverIcon: subjectEmoji(course.subject || course.displayName, index),
      entryCourseId: course.entryCourseId || course.id,
      accessLabel: course.accessLabel || (isOpened ? "已开通" : ((Number(course.materialNum) > 0 || Number(course.homeworkNum) > 0) ? "首节可体验" : "")),
      // 兼容旧接口未返回 accessState/canOpen 的情况：有首节内容就应允许进入体验。
      canOpen: course.accessState !== "pending" && (course.accessState === "preview" || (typeof course.canOpen === "boolean" ? course.canOpen : Boolean(course.id)) || Number(course.materialNum) > 0 || Number(course.homeworkNum) > 0),
      isLocked: course.accessState === "locked" && Number(course.materialNum) <= 0 && Number(course.homeworkNum) <= 0,
      imageUrl: course.imageUrl || "",
      isOpened
    };
  });
}

// 接口会同时返回已开通课程和年级学科目录。已开通课程保留真实进度并置顶，
// 目录中与其同学科的占位卡片不再重复展示。
function mergeStudyCourses(subjects, courses, favorites) {
  const opened = decorateCourses((courses || []).map((course) => ({ ...course, isOpened: true })), favorites);
  const openedSubjects = new Set(opened.map((course) => String(course.subject || course.displayName || "").trim()).filter(Boolean));
  const catalog = decorateCourses((subjects || []).filter((subject) => {
    const key = String(subject.subject || subject.displayName || "").trim();
    return !openedSubjects.has(key);
  }), favorites);
  return opened.concat(catalog);
}

function subjectEmoji(subject, index) {
  const icons = { 数学: "➗", 英文: "🔤", 语文: "📖", 科学: "🔬", 地理: "🌍", 物理: "⚙️", 化学: "🧪" };
  return icons[subject] || (index % 2 === 0 ? "📚" : "✨");
}

function courseAvailableAt(course) {
  const value = String(course.availableAt || course.openedAt || "").replace(" ", "T");
  const timestamp = Date.parse(value);
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

function formatCourseTime(value) {
  const text = String(value || "");
  return text.length >= 16 ? text.slice(5, 16).replace(" ", " ") : text;
}

function studyEmptyMessage(hasOpenedPackage) {
  if (hasOpenedPackage) {
    return "课程已开通，内容发布后会显示在这里。";
  }
  return "暂时还没有开通课程，请联系老师确认。";
}
