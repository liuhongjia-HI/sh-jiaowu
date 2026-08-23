const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "有新小挑战、批改结果或资料更新时，会提醒你。",
    activeFilter: "全部",
    filters: [
      { label: "全部", className: "active" },
      { label: "课程", className: "" },
      { label: "作业", className: "" },
      { label: "系统", className: "" }
    ],
    notices: [],
    visibleNotices: []
  },
  onLoad() {
    this.setData({ loading: true, error: "" });
    request("/student/notices")
      .then((notices) => this.setData({ notices: decorateNotices(notices || []), loading: false }, () => this.applyFilters()))
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "有新小挑战、批改结果或资料更新时，会提醒你。",
        loading: false
      }));
  },
  onShareAppMessage() {
    return {
      title: "Starline 学习消息提醒",
      path: "/pages/notices/index"
    };
  },
  goStudy() {
    wx.switchTab({ url: "/pages/study/index" });
  },
  changeFilter(event) {
    const activeFilter = event.currentTarget.dataset.filter;
    this.setData({
      activeFilter,
      filters: this.data.filters.map((item) => ({ ...item, className: item.label === activeFilter ? "active" : "" }))
    }, () => this.applyFilters());
  },
  applyFilters() {
    const activeFilter = this.data.activeFilter;
    const visibleNotices = activeFilter === "全部"
      ? this.data.notices
      : this.data.notices.filter((notice) => notice.category === activeFilter);
    this.setData({ visibleNotices });
  }
});

function decorateNotices(notices) {
  return notices.map((notice) => ({
    ...notice,
    icon: notice.type || "新",
    iconClass: notice.type === "评" ? "review" : "default",
    category: noticeCategory(notice)
  }));
}

function noticeCategory(notice) {
  const relatedType = String(notice.relatedType || "").toLowerCase();
  const type = String(notice.type || "");
  const text = [notice.title, notice.summary].filter(Boolean).join(" ");
  if (relatedType === "schedule" || relatedType === "course") {
    return "课程";
  }
  if (["homework", "review", "submission"].includes(relatedType)) {
    return "作业";
  }
  if (type === "课") return "课程";
  if (["练", "评", "作业"].includes(type)) return "作业";
  if (/课程|上课|调课|排课/.test(text)) return "课程";
  if (/作业|挑战|练习|批改|反馈/.test(text)) return "作业";
  return "系统";
}
