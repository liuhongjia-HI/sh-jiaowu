const { request } = require("../../utils/request");
const { subjectEmoji, subjectsMatchName } = require("../../utils/subject");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "Unlocked courses will appear here.",
    keyword: "",
    activeFilter: "all",
    filters: [
      { key: "all", label: "All", className: "active" },
      { key: "learning", label: "In Progress", className: "" },
      { key: "saved", label: "Saved", className: "" },
      { key: "done", label: "Completed", className: "" }
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
      title: "My Starline learning",
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
        error: error.message || "Failed to load",
        emptyMessage: error.message || "Unlocked courses will appear here.",
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
      filters: this.data.filters.map((item) => ({ ...item, className: item.key === activeFilter ? "active" : "" }))
    }, () => this.applyFilters());
  },
  applyFilters() {
    const keyword = this.data.keyword.trim().toLowerCase();
    const activeFilter = this.data.activeFilter;
    const visibleCourses = this.data.courses.filter((course) => {
      const matchKeyword = !keyword || [course.name, course.displayName, course.subject, course.grade, course.displayMeta, course.accessLabel].join(" ").toLowerCase().includes(keyword);
      const completed = isCompletedCourse(course);
      const matchFilter = activeFilter === "all"
        || (activeFilter === "learning" && !completed)
        || (activeFilter === "saved" && course.favorited)
        || (activeFilter === "done" && completed);
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
      wx.showToast({ title: dataset.message || "Unlock to access all content", icon: "none" });
      return;
    }
    if (!id) {
      wx.showToast({ title: "Content is being prepared", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${id}` });
  }
});

function hasStudentToken() {
  return Boolean(wx.getStorageSync && wx.getStorageSync("starline_token"));
}

function isCompletedCourse(course) {
  return course.status === "已完成" || Number(course.progress) >= 100;
}

// decorateCourses 使用接口返回的真实进度，仅补充图标等展示字段。
function decorateCourses(courses, favorites) {
  const favoriteCourseNames = (favorites || []).map((item) => item.course).filter(Boolean);
  return [...courses].sort((left, right) => courseAvailableAt(right) - courseAvailableAt(left)).map((course, index) => {
    const progress = Number(course.progress) || 0;
    const isNew = Boolean(course.isNew);
    const isOpened = Boolean(course.isOpened);
    const isPreview = course.accessState === "preview";
    const isPreparing = course.accessState === "pending";
    const displayName = course.displayName || course.name;
    const accessLabel = course.accessLabel || (isNew ? "New" : "");
    return {
      ...course,
      progress,
      favorited: favoriteCourseNames.includes(course.name),
      badgeText: accessLabel,
      cardClass: isNew ? "new-course" : progress >= 100 ? "reward" : isOpened ? "opened-course" : "",
      newCourseText: isNew && course.availableAt ? `New · ${formatCourseTime(course.availableAt)}` : "",
      coverIcon: subjectEmoji(course.subject || course.displayName, index),
      displayName,
      displayMeta: course.grade || "Learning content",
      entryCourseId: course.entryCourseId || course.id,
      accessLabel,
      isPreview,
      isPreparing,
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
  const opened = decorateCourses((courses || []).map((course) => ({
    ...course,
    isOpened: true,
    displayName: course.displayName || catalogDisplayName(course, subjects)
  })), favorites);
  const openedSubjects = opened.map((course) => String(course.subject || course.displayName || "").trim()).filter(Boolean);
  const catalog = decorateCourses((subjects || []).filter((subject) => {
    const key = String(subject.subject || subject.displayName || "").trim();
    return !openedSubjects.some((item) => subjectsMatchName(item, key));
  }), favorites);
  return opened.concat(catalog);
}

function catalogDisplayName(course, subjects) {
  const subject = String(course.subject || "").trim();
  if (!subject) return "";
  const grade = String(course.grade || "").trim();
  const matched = (subjects || []).find((item) => {
    if (!subjectsMatchName(item.subject, subject)) return false;
    const itemGrade = String(item.grade || "").trim();
    return !grade || !itemGrade || itemGrade === grade;
  });
  return String((matched && matched.displayName) || "").trim();
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
    return "Course unlocked. Content will appear here once published.";
  }
  return "No courses unlocked yet. Please contact your teacher.";
}
