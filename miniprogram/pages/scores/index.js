const { request } = require("../../utils/request");
const { subjectLabel } = require("../../utils/subject");

Page({
  data: {
    loading: true,
    emptyMessage: "Your scores will appear here after exercises or assessments.",
    examScores: [],
    practiceRecords: [],
    latestSummary: null
  },
  onLoad() {
    this.loadScores();
  },
  onShareAppMessage() {
    return {
      title: "Starline Progress",
      path: "/pages/scores/index"
    };
  },
  onShow() {
    if (!this.data.loading && this.data.examScores.length === 0 && this.data.practiceRecords.length === 0) {
      this.loadScores();
    }
  },
  loadScores() {
    this.setData({ loading: true });
    Promise.all([
      request("/student/scores"),
      request("/student/growth")
    ])
      .then(([scores, growth]) => {
        const examScores = normalizeExamScores(scores || []);
        const practiceRecords = (growth || [])
          .filter((item) => item.type === "小挑战")
          .map((item) => ({
            ...item,
            scoreText: item.fullScore ? `${item.score || 0}/${item.fullScore}` : `${item.score || 0}`
          }));
        this.setData({
          examScores,
          practiceRecords,
          latestSummary: latestSummary(examScores, practiceRecords),
          loading: false
        });
      })
      .catch((error) => this.setData({
        emptyMessage: error.message || "Failed to load scores",
        loading: false
      }));
  }
});

function normalizeExamScores(scores) {
  return scores.map((summary) => {
    const latest = summary.latestRecord || {};
    const first = summary.firstRecord || {};
    const trend = latest.id && first.id && latest.id !== first.id
      ? `${summary.improvement >= 0 ? "+" : ""}${summary.improvement} pts`
      : "No comparison yet";
    return {
      ...summary,
      displayName: subjectLabel(summary.subject),
      latest,
      first,
      trend,
      examTypeText: latest.examType || "Progress Assessment",
      latestScoreText: latest.fullScore ? `${latest.score}/${latest.fullScore}` : `${latest.score || 0}`,
      problemPoint: summary.problemPoint || summary.description || "Your teacher has not added areas to improve yet.",
      nextStep: summary.nextStep || latest.teacherComment || "Your teacher has not added next steps yet.",
      teacherComment: latest.teacherComment || summary.description || "Your teacher has not added advice yet."
    };
  });
}

function latestSummary(examScores, practiceRecords) {
  if (examScores.length > 0) {
    const latest = examScores[0].latest || {};
    return {
      title: `${examScores[0].displayName || examScores[0].subject || "Exam"} ${examScores[0].latestScoreText}`,
      subtitle: examScores[0].description || latest.examName || "Latest exam score"
    };
  }
  if (practiceRecords.length > 0) {
    const latest = practiceRecords[0];
    return {
      title: `${latest.title} ${latest.scoreText} pts`,
      subtitle: latest.description || "Latest practice"
    };
  }
  return null;
}
