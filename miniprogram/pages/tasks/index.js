const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "New exercises will appear here when published.",
    activeFilter: "全部",
    filters: [
      { label: "全部", displayLabel: "All", className: "active" },
      { label: "待完成", displayLabel: "Pending", className: "" },
      { label: "批改中", displayLabel: "In Review", className: "" },
      { label: "已完成", displayLabel: "Completed", className: "" }
    ],
    tasks: [],
    visibleTasks: []
  },
  onLoad() {
    this.loadTasks();
  },
  onShareAppMessage() {
    return {
      title: "Starline Exercises",
      path: "/pages/tasks/index"
    };
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
        error: error.message || "Failed to load",
        emptyMessage: error.message || "New exercises will appear here when published.",
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
      statusLabel: ({ "待完成": "Pending", "批改中": "In Review", "已完成": "Completed" })[studentStatus] || studentStatus,
      rewardText: done ? (task.score >= 90 ? "High Score" : "Completed") : "Rewards Available",
      estimateText: done ? `Score: ${task.score || 0}` : `${task.questionNum || 0} questions · About 8 min`,
      cardClass: done ? "" : "reward"
    };
  });
}
