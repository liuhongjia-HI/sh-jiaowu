const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "有新小挑战、批改结果或资料更新时，会提醒你。",
    notices: []
  },
  onLoad() {
    this.setData({ loading: true, error: "" });
    request("/student/notices")
      .then((notices) => this.setData({ notices: decorateNotices(notices || []), loading: false }))
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "有新小挑战、批改结果或资料更新时，会提醒你。",
        loading: false
      }));
  }
});

function decorateNotices(notices) {
  return notices.map((notice) => ({
    ...notice,
    icon: notice.type || "新"
  }));
}
