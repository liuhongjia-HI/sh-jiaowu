const { request } = require("../../utils/request");

Page({
  data: {
    taskTitle: "批改结果",
    resultTitle: "完成啦 🎉",
    teacherComment: "老师正在批改，稍后就能看到反馈。",
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
          taskTitle: data.taskTitle || "批改结果",
          resultTitle: pending ? "已提交，等待老师批改" : `${data.score} 分，${scoreTag(data.score)}`,
          teacherComment: data.teacherComment || "",
          rewardText: data.reward || "",
          pendingReview: pending,
          objectiveText: pending ? `客观题得分 ${data.objectiveScore || data.score || 0} 分` : ""
        });
      })
      .catch(() => {});
  },
  goBack() {
    wx.navigateBack({
      delta: 1,
      fail() {
        wx.switchTab({ url: "/pages/tasks/index" });
      }
    });
  },
  goStudy() {
    wx.switchTab({ url: "/pages/study/index" });
  }
});

function scoreTag(score) {
  if (score >= 90) {
    return "表现很棒 🎉";
  }
  if (score >= 60) {
    return "继续加油 💪";
  }
  return "下次会更好 🌱";
}
