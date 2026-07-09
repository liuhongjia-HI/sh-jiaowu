const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    emptyMessage: "老师录入成绩或完成批改后，这里会显示学习效果。",
    examScores: [],
    practiceRecords: [],
    latestSummary: null
  },
  onLoad() {
    this.loadScores();
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
        emptyMessage: error.message || "成绩加载失败",
        loading: false
      }));
  }
});

function normalizeExamScores(scores) {
  return scores.map((summary) => {
    const latest = summary.latestRecord || {};
    const first = summary.firstRecord || {};
    const trend = latest.id && first.id && latest.id !== first.id
      ? `${summary.improvement >= 0 ? "+" : ""}${summary.improvement} 分`
      : "暂无对比";
    return {
      ...summary,
      latest,
      first,
      trend,
      examTypeText: latest.examType || "阶段测评",
      latestScoreText: latest.fullScore ? `${latest.score}/${latest.fullScore}` : `${latest.score || 0}`,
      problemPoint: summary.problemPoint || summary.description || "老师还没有标记问题点。",
      nextStep: summary.nextStep || latest.teacherComment || "老师还没有填写下一步建议。",
      teacherComment: latest.teacherComment || summary.description || "老师还没有填写建议。"
    };
  });
}

function latestSummary(examScores, practiceRecords) {
  if (examScores.length > 0) {
    const latest = examScores[0].latest || {};
    return {
      title: `${examScores[0].subject || "考试"} ${examScores[0].latestScoreText}`,
      subtitle: examScores[0].description || latest.examName || "最近一次考试成绩"
    };
  }
  if (practiceRecords.length > 0) {
    const latest = practiceRecords[0];
    return {
      title: `${latest.title} ${latest.scoreText} 分`,
      subtitle: latest.description || "最近一次平时练习"
    };
  }
  return null;
}
