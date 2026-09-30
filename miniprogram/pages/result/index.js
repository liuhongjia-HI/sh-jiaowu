const { request } = require("../../utils/request");

Page({
  data: {
    taskTitle: "Results",
    resultTitle: "Well done! 🎉",
    teacherComment: "Your teacher is reviewing your work. Feedback will be available soon.",
    teacherCommentNodes: "Your teacher is reviewing your work. Feedback will be available soon.",
    rewardText: "",
    pendingReview: false,
    objectiveText: ""
  },
  onLoad(options) {
    const id = options.id || "";
    if (id.indexOf("sub-") !== 0) {
      // 无有效提交编号时保持待批改状态。
      return;
    }
    request(`/student/submissions/${id}`)
      .then((data) => {
        const pending = data.status === "待批改";
        this.setData({
          taskTitle: data.taskTitle || "Results",
          resultTitle: pending ? "Submitted. Awaiting teacher review." : `${data.score} pts, ${scoreTag(data.score)}`,
          teacherComment: data.teacherComment || "",
          teacherCommentNodes: data.teacherComment || "Your teacher is reviewing your work. Feedback will be available soon.",
          rewardText: data.reward || "",
          pendingReview: pending,
          objectiveText: pending ? `Objective score: ${data.objectiveScore || data.score || 0} pts` : ""
        });
      })
      .catch(() => {});
  },
  onShareAppMessage() {
    return {
      title: this.data.taskTitle ? `I completed a Starline exercise: ${this.data.taskTitle}` : "I completed a Starline exercise",
      path: "/pages/home/index"
    };
  },
  goBack() {
    wx.navigateBack({
      delta: 1,
      fail() {
        wx.navigateTo({ url: "/pages/tasks/index" });
      }
    });
  },
  goStudy() {
    wx.switchTab({ url: "/pages/study/index" });
  }
});

function scoreTag(score) {
  if (score >= 90) {
    return "Great work! 🎉";
  }
  if (score >= 60) {
    return "Keep going! 💪";
  }
  return "Keep practicing! 🌱";
}
