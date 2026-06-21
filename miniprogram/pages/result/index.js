const { request } = require("../../utils/request");

Page({
  data: {
    taskTitle: "批改结果",
    resultTitle: "完成啦 🎉",
    teacherComment: "老师正在批改，稍后就能看到反馈。",
    rewardText: ""
  },
  onLoad(options) {
    const id = options.id || "";
    if (id.indexOf("sub-") !== 0) {
      // 兼容演示入口（任务列表的占位数据），展示默认文案
      return;
    }
    request(`/student/submissions/${id}`)
      .then((data) => {
        this.setData({
          taskTitle: data.taskTitle || "批改结果",
          resultTitle: `${data.score} 分，${scoreTag(data.score)}`,
          teacherComment: data.teacherComment || "",
          rewardText: data.reward || ""
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
