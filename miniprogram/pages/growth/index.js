const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    emptyMessage: "Your progress will appear here as you complete exercises and study courses.",
    records: []
  },
  onLoad() {
    this.loadGrowth();
  },
  onShareAppMessage() {
    return {
      title: "My Starline Progress",
      path: "/pages/growth/index"
    };
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
        emptyMessage: error.message || "Failed to load",
        loading: false
      }));
  }
});
