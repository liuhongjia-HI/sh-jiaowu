const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "课程、练习和反馈有更新时，会提醒你。",
    activeFilter: "全部",
    filters: [
      { label: "全部", className: "active" },
      { label: "课程", className: "" },
      { label: "作业", className: "" },
      { label: "系统", className: "" }
    ],
    notices: [],
    visibleNotices: [],
    currentStudentName: "",
    currentStudentGrade: "",
    linkedStudentText: "",
    studentCount: 0
  },
  onLoad() {
    this._skipInitialShowRefresh = true;
    this.loadNotices();
  },
  onShow() {
    if (this._skipInitialShowRefresh) {
      this._skipInitialShowRefresh = false;
      return;
    }
    this.loadNotices();
  },
  loadNotices() {
    this.setData({ loading: true, error: "" });
    Promise.all([
      request("/student/notices"),
      request("/student/accounts", { silent: true }).catch(() => [])
    ])
      .then(([notices, accounts]) => {
        const active = (accounts || []).find((item) => item.active) || {};
        this.setData({
          notices: decorateNotices(notices || [], active),
          currentStudentName: active.name || "",
          currentStudentGrade: active.grade || "",
          linkedStudentText: (accounts || []).map(studentDisplay).filter(Boolean).join("、"),
          studentCount: (accounts || []).length,
          loading: false
        }, () => this.applyFilters());
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "课程、练习和反馈有更新时，会提醒你。",
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
  goMe() {
    wx.switchTab({ url: "/pages/me/index" });
  },
  goNotice(event) {
    const id = event.currentTarget.dataset.id || "";
    const notice = this.data.notices.find((item) => item.id === id);
    const path = notice && notice.destinationPath;
    if (!path) {
      wx.showToast({ title: "这条通知暂无可查看的详情", icon: "none" });
      return;
    }
    wx.navigateTo({ url: path });
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

function decorateNotices(notices, student) {
  return notices.map((notice) => ({
    ...notice,
    icon: notice.type || "新",
    iconClass: notice.type === "评" ? "review" : "default",
    category: noticeCategory(notice),
    scopeText: noticeScopeText(notice),
    studentDisplay: studentDisplay(student),
    destinationPath: noticeDestination(notice)
  }));
}

function noticeDestination(notice) {
  const type = String(notice.relatedType || "").toLowerCase();
  const id = String(notice.relatedId || "").trim();
  if (type === "schedule") return "/pages/schedule/index";
  if (!id) return "";
  if (type === "homework") return `/pages/answer/index?id=${encodeURIComponent(id)}`;
  if (["review", "submission", "feedback"].includes(type)) {
    return id.indexOf("sub-") === 0 ? `/pages/result/index?id=${encodeURIComponent(id)}` : "/pages/tasks/index";
  }
  if (type === "course") return `/pages/study-detail/index?id=${encodeURIComponent(id)}`;
  if (["material", "handout"].includes(type)) return `/pages/material-preview/index?id=${encodeURIComponent(id)}`;
  return "";
}

function studentDisplay(student) {
  const name = String(student && student.name || "").trim();
  const grade = String(student && student.grade || "").trim();
  if (!name) return "";
  return grade ? `${name}（${grade}）` : name;
}

function noticeScopeText(notice) {
  if (String(notice.relatedType || "").toLowerCase() === "student") return "仅发给指定学生";
  const target = String(notice.target || "").trim();
  if (!target || /全部|全体/.test(target)) return "面向全部学生";
  return `通知范围：${target}`;
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
