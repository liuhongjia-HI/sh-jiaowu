const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "请先登录绑定，或联系老师开通学习套餐。",
    home: null,
    pendingTask: null,
    continueCourse: null,
    progressPercent: 0,
    pendingCount: 0,
    courseTitle: "待解锁学习星球",
    courseMeta: "",
    mapTag: "继续学习"
  },
  onLoad() {
    this.loadHome();
  },
  onShow() {
    if (!this.data.loading && !this.data.home) {
      this.loadHome();
    }
  },
  loadHome() {
    this.setData({ loading: true, error: "" });
    request("/student/home")
      .then((home) => {
        const continueCourse = home.continueCourse || {};
        const pendingTask = (home.pendingHomework || [])[0] || null;
        const progressPercent = Number(home.continueProgress) || 0;
        this.setData({
          home,
          continueCourse,
          pendingTask,
          progressPercent,
          pendingCount: (home.pendingHomework || []).length,
          courseTitle: continueCourse.name || "待解锁学习星球",
          courseMeta: `${continueCourse.grade || ""} · ${continueCourse.subject || ""} · ${continueCourse.chapterCount || 0} 个章节`,
          mapTag: pendingTask ? "新小挑战" : "继续学习",
          loading: false
        });
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "请先登录绑定，或联系老师开通学习套餐。",
        loading: false
      }));
  },
  goStudyDetail() {
    if (!this.data.continueCourse || !this.data.continueCourse.id) {
      wx.switchTab({ url: "/pages/study/index" });
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${this.data.continueCourse.id}` });
  },
  goAnswer() {
    if (!this.data.pendingTask) {
      wx.switchTab({ url: "/pages/tasks/index" });
      return;
    }
    wx.navigateTo({ url: `/pages/answer/index?id=${this.data.pendingTask.id}` });
  },
  stopCardTap() {},
  goTasks() {
    wx.switchTab({ url: "/pages/tasks/index" });
  },
  goLogin() {
    wx.navigateTo({ url: "/pages/login/index" });
  }
});
