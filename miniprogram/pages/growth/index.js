const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    emptyMessage: "完成小挑战、学习资料后，这里会记录你的成长足迹。",
    records: []
  },
  onLoad() {
    this.loadGrowth();
  },
  onShow() {
    if (!this.data.loading && this.data.records.length === 0) {
      this.loadGrowth();
    }
  },
  loadGrowth() {
    this.setData({ loading: true });
    request("/student/growth")
      .then((records) => this.setData({ records: records || [], loading: false }))
      .catch((error) => this.setData({
        emptyMessage: error.message || "加载失败",
        loading: false
      }));
  }
});
