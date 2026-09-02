const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "老师开通学习套餐后，你会在这里看到学习星球。",
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
    materials: [],
    hasOpenedPackage: false
  },
  onLoad() {
    this.loadStudy();
  },
  onShareAppMessage() {
    return {
      title: "我的 Starline 学习课程",
      path: "/pages/study/index"
    };
  },
  onShow() {
    if (!this.data.loading) {
      this.loadStudy();
    }
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
        this.setData({
          courses: decorateCourses(subjects.length ? subjects : courses, favorites || []),
          subjects,
          materials,
          hasOpenedPackage,
          emptyMessage: studyEmptyMessage(hasOpenedPackage),
          loading: false
        }, () => this.applyFilters());
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "老师开通学习套餐后，你会在这里看到学习星球。",
        hasOpenedPackage: false,
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
      const matchKeyword = !keyword || [course.name, course.subject, course.grade].join(" ").toLowerCase().includes(keyword);
      const matchFilter = activeFilter === "全部" || (activeFilter === "学习中" && course.status !== "已完成") || (activeFilter === "已收藏" && course.favorited) || (activeFilter === "已完成" && course.status === "已完成");
      return matchKeyword && matchFilter;
    });
    this.setData({ visibleCourses });
  },
  goDetail(event) {
    const id = event.currentTarget.dataset.id || "";
    const canOpen = event.currentTarget.dataset.canOpen;
    if (!canOpen) {
      wx.showToast({ title: event.currentTarget.dataset.message || "开通后即可学习全部内容", icon: "none" });
      return;
    }
    if (!id) {
      wx.showToast({ title: "课程内容正在准备", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${id}` });
  }
});

// decorateCourses 使用接口返回的真实进度，仅补充图标等展示字段。
function decorateCourses(courses, favorites) {
  const favoriteCourseNames = (favorites || []).map((item) => item.course).filter(Boolean);
  return [...courses].sort((left, right) => courseAvailableAt(right) - courseAvailableAt(left)).map((course, index) => {
    const progress = Number(course.progress) || 0;
    const isNew = Boolean(course.isNew);
    return {
      ...course,
      progress,
      favorited: favoriteCourseNames.includes(course.name),
      badgeText: course.accessLabel || (progress >= 80 ? "阅读小达人" : progress > 0 ? "继续加油" : "新课程"),
      cardClass: isNew ? "new-course" : progress >= 100 ? "reward" : "",
      newCourseText: isNew && course.availableAt ? `新开通 · ${formatCourseTime(course.availableAt)}` : "",
      coverIcon: subjectEmoji(course.subject || course.displayName, index),
      entryCourseId: course.entryCourseId || course.id,
      canOpen: course.accessState !== "locked" && course.accessState !== "pending" && (typeof course.canOpen === "boolean" ? course.canOpen : Boolean(course.id)),
      isLocked: course.accessState === "locked" || course.accessState === "pending",
      imageUrl: course.imageUrl || ""
    };
  });
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
    return "学习套餐已开通，老师发布课程后会显示在这里。你也可以先回首页查看课程讲义和小挑战。";
  }
  return "你的身份已绑定，暂时还没有开通学习套餐，请联系老师或教务确认。";
}
