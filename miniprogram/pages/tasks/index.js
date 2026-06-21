const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "新挑战发布后，会第一时间出现在这里。",
    activeFilter: "全部",
    filters: [
      { label: "全部", className: "active" },
      { label: "待完成", className: "" },
      { label: "批改中", className: "" },
      { label: "已完成", className: "" }
    ],
    tasks: [],
    visibleTasks: []
  },
  onLoad() {
    this.loadTasks();
  },
  onShow() {
    if (!this.data.loading) {
      this.loadTasks();
    }
  },
  loadTasks() {
    this.setData({ loading: true, error: "" });
    request("/student/tasks")
      .then((tasks) => {
        this.setData({ tasks: decorateTasks(tasks || []), loading: false }, () => this.applyFilters());
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "新挑战发布后，会第一时间出现在这里。",
        loading: false
      }));
  },
  changeFilter(event) {
    const activeFilter = event.currentTarget.dataset.filter;
    this.setData({
      activeFilter,
      filters: this.data.filters.map((item) => ({ ...item, className: item.label === activeFilter ? "active" : "" }))
    }, () => this.applyFilters());
  },
  applyFilters() {
    const filter = this.data.activeFilter;
    const visibleTasks = filter === "全部" ? this.data.tasks : this.data.tasks.filter((task) => task.studentStatus === filter);
    this.setData({ visibleTasks });
  },
  goAnswer(event) {
    const id = event.currentTarget.dataset.id || "";
    const task = this.data.tasks.find((item) => item.id === id);
    if (task && task.studentStatus === "已完成" && task.submissionId) {
      wx.navigateTo({ url: `/pages/result/index?id=${task.submissionId}` });
      return;
    }
    wx.navigateTo({ url: `/pages/answer/index?id=${id}` });
  }
});

// decorateTasks 仅基于接口返回的真实 studentStatus 补充展示字段。
function decorateTasks(tasks) {
  return tasks.map((task) => {
    const studentStatus = task.studentStatus || "待完成";
    const done = studentStatus === "已完成";
    return {
      ...task,
      studentStatus,
      rewardText: done ? (task.score >= 90 ? "高分" : "已完成") : "有奖励",
      estimateText: done ? `得分 ${task.score || 0}` : `${task.questionNum || 0} 道题 · 预计 8 分钟`,
      cardClass: done ? "" : "reward"
    };
  });
}
