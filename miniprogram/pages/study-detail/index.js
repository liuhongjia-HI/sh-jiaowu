const { request } = require("../../utils/request");

Page({
  data: {
    course: {},
    materials: [],
    homework: [],
    stations: [],
    progress: 0,
    teacherText: "",
    materialCountText: "0 份资料",
    homeworkText: "可得徽章"
  },
  onLoad(options) {
    this.courseId = options.id || "";
    if (!this.courseId) {
      this.setData({ teacherText: "课程信息缺失" });
      return;
    }
    this.loadDetail();
  },
  onShow() {
    // 从答题页提交返回时刷新站点状态
    if (this.courseId && this.loaded) {
      this.loadDetail();
    }
  },
  loadDetail() {
    request(`/student/study/${this.courseId}`).then((data) => {
      const course = data.course || {};
      const materials = data.materials || [];
      const homework = data.homework || [];
      this.loaded = true;
      this.setData({
        course,
        materials,
        homework,
        stations: data.stations || [],
        progress: data.progress || 0,
        teacherText:
          (materials[0] && materials[0].ownerTeacherName) ||
          (homework[0] && homework[0].ownerTeacherName) ||
          `${course.subject || ""}老师`,
        materialCountText: `${materials.length} 份资料`,
        homeworkText: homework.length ? `${homework.length} 个挑战` : "可得徽章"
      });
    });
  },
  goPreview() {
    const material = this.data.materials[0];
    if (!material) {
      wx.showToast({ title: "暂无学习资料", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/material-preview/index?id=${material.id}` });
  },
  goAnswer() {
    const homework = this.data.homework[0];
    if (!homework) {
      wx.showToast({ title: "暂无小挑战", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/answer/index?id=${homework.id}` });
  },
  tapStation(event) {
    const { status, materialId, homeworkId } = event.currentTarget.dataset;
    if (materialId) {
      wx.navigateTo({ url: `/pages/material-preview/index?id=${materialId}` });
      return;
    }
    if (homeworkId && status !== "已完成") {
      wx.navigateTo({ url: `/pages/answer/index?id=${homeworkId}` });
    }
  }
});
