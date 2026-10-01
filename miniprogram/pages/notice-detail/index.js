const { request } = require("../../utils/request");
const { refreshNoticeBadge } = require("../../utils/notice-badge");

Page({
  data: { loading: true, error: "", detail: null, lessons: [], studentName: "", title: "", summary: "", createdAt: "" },
  onLoad(options = {}) {
    this.noticeId = String(options.id || "");
    this.loadDetail();
  },
  onUnload() { this.generation = (this.generation || 0) + 1; },
  loadDetail() {
    const generation = this.generation = (this.generation || 0) + 1;
    this.setData({ loading: true, error: "", detail: null, lessons: [], studentName: "", title: "", summary: "" });
    if (!/^[a-f0-9]{64}$/.test(this.noticeId)) {
      this.setData({ loading: false, error: "This message is unavailable." });
      return Promise.resolve();
    }
    return request(`/student/business-notices/${this.noticeId}`)
      .then(async (detail) => {
        if (generation !== this.generation) return null;
        if (detail.canSwitch) {
          const result = await request(`/student/accounts/${encodeURIComponent(detail.event.studentId)}/switch`, { method: "POST", data: {} });
          if (generation !== this.generation) return null;
          if (!result.token || !result.user || result.user.studentId !== detail.event.studentId) throw new Error("Unable to open the linked student account.");
          wx.setStorageSync("starline_token", result.token);
          wx.setStorageSync("starline_student_id", result.user.studentId);
          refreshNoticeBadge();
          return request(`/student/business-notices/${this.noticeId}`);
        }
        return detail;
      })
      .then((detail) => {
        if (!detail || generation !== this.generation) return;
        const current = new Map((detail.currentLessons || []).map((item) => [item.id, item]));
        const lessons = (detail.event.lessons || []).map((change) => {
          const latest = current.get(change.after.id);
          return { id: change.after.id, course: change.after.courseName || change.after.name, before: change.before ? lessonTime(change.before) : "", notified: lessonTime(change.after), current: latest ? lessonTime(latest) : "", changed: !!latest && lessonTime(latest) !== lessonTime(change.after), status: !latest ? "Unavailable" : latest.status === "已取消" ? "Cancelled" : latest.auditStatus !== "已通过" || latest.status !== "已确认" ? "Awaiting confirmation" : "Confirmed", teacher: latest ? latest.teacherName : "", room: latest ? latest.roomName : "", campus: latest ? latest.campusId : "" };
        });
        this.setData({ loading: false, detail, lessons, studentName: detail.event.studentName, title: detail.event.title, summary: detail.event.summary, createdAt: detail.event.createdAt ? detail.event.createdAt.replace("T", " ").slice(0, 16) : "" });
        request(`/student/notices/business-${this.noticeId}/read`, { method: "POST", data: {}, silent: true }).then(() => refreshNoticeBadge()).catch(() => {});
      })
      .catch((error) => {
        if (generation !== this.generation) return;
        this.setData({ loading: false, error: error.message || "Unable to load this message.", detail: null, lessons: [], studentName: "" });
      });
  },
  openRelated() {
    const event = this.data.detail && this.data.detail.event;
    if (!event) return;
    if (event.kind === "homework_submitted" || event.kind === "review_completed") {
      wx.navigateTo({ url: `/pages/result/index?id=${encodeURIComponent(event.relatedId)}` });
    } else if (event.kind === "homework_published") {
      wx.navigateTo({ url: `/pages/answer/index?id=${encodeURIComponent(event.relatedId)}` });
    } else if (event.kind.startsWith("homework_") || event.kind === "review_exception") {
      wx.navigateTo({ url: "/pages/tasks/index" });
    } else wx.navigateTo({ url: "/pages/schedule/index" });
  }
});

function lessonTime(lesson) {
  return `${lesson.lessonDate || lesson.startDate || ""} ${lesson.startTime || ""}–${lesson.endTime || ""}`.trim();
}
