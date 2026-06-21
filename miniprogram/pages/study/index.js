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
    visibleCourses: [],
    materials: []
  },
  onLoad() {
    this.loadStudy();
  },
  loadStudy() {
    this.setData({ loading: true, error: "" });
    Promise.all([request("/student/study"), request("/student/favorites").catch(() => [])])
      .then(([data, favorites]) => {
        const courses = Array.isArray(data) ? data : (data.courses || []);
        const materials = Array.isArray(data) ? [] : (data.materials || []);
        this.setData({ courses: decorateCourses(courses, favorites || []), materials, loading: false }, () => this.applyFilters());
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "老师开通学习套餐后，你会在这里看到学习星球。",
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
    if (!id) {
      wx.showToast({ title: "课程信息缺失", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${id}` });
  }
});

// decorateCourses 使用接口返回的真实进度，仅补充图标等展示字段。
function decorateCourses(courses, favorites) {
  const favoriteCourseNames = (favorites || []).map((item) => item.course).filter(Boolean);
  return courses.map((course, index) => {
    const progress = Number(course.progress) || 0;
    return {
      ...course,
      progress,
      favorited: favoriteCourseNames.includes(course.name),
      badgeText: progress >= 80 ? "阅读小达人" : progress > 0 ? "继续加油" : "新课程",
      cardClass: progress >= 100 ? "reward" : "",
      coverIcon: index % 2 === 0 ? "📖" : "💡"
    };
  });
}
