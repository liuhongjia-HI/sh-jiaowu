const { request } = require("../../utils/request");
const { activateContentSecurity } = require("../../utils/content-security");

Page({
  data: {
    homeworkId: "",
    taskTitle: "课后练习",
    deadlineText: "",
    rewardText: "完成练习可获得徽章",
    questions: [],
    downloadUrl: "",
    watermarkText: "水印加载中",
    watermarkTexts: ["水印加载中", "水印加载中", "水印加载中", "水印加载中", "水印加载中", "水印加载中"],
    securityNotice: "仅供本人学习，请勿外传。",
    favorited: false,
    favoriteId: "",
    saving: false,
    isOverdue: false
  },
  onLoad(options) {
    const id = options.id || "";
    if (!id) {
      wx.showToast({ title: "题目信息缺失", icon: "none" });
      return;
    }
    this.setData({ homeworkId: id });
    this.stopContentSecurity = activateContentSecurity({
      targetType: "homework",
      targetId: id,
      pagePath: "pages/answer/index"
    });
    request(`/student/homework/${id}`).then((homework) => {
      const questions = (homework.questions || []).map((question, index) => ({
        ...question,
        index: index + 1,
        options: ((question.type === "judge" && (!question.options || question.options.length === 0)) ? ["正确", "错误"] : (question.options || [])).map((text, optionIndex) => ({
          value: text,
          label: `${letter(optionIndex)}. ${text}`,
          className: ""
        })),
        choice: "",
        choices: [],
        text: ""
      }));
      const watermarkText = homework.watermarkText || "水印加载中";
      this.setData({
        taskTitle: `${homework.assessmentType === "mock_exam" ? "模拟考试 · " : "练习 · "}${homework.title || "课后练习"}`,
        deadlineText: homework.isOverdue ? "已截止" : (homework.deadlineAt ? `截止 ${formatDeadline(homework.deadlineAt)}` : (homework.deadline ? `${homework.deadline} 前完成` : "")),
        rewardText: homework.course || "完成练习可获得徽章",
        questions: restoreDraftAnswers(id, questions),
        downloadUrl: homework.downloadUrl || "",
        watermarkText,
        watermarkTexts: buildWatermarks(watermarkText),
        securityNotice: homework.securityNotice || "仅供本人学习，请勿外传。",
        isOverdue: Boolean(homework.isOverdue)
      });
    }).catch(() => {
      this.setData({
        rewardText: "题目加载失败",
        securityNotice: "题目加载失败，请重新进入。"
      });
    });
    this.refreshFavorite(id);
  },
  onShareAppMessage() {
    return {
      title: this.data.taskTitle ? `Starline 练习：${this.data.taskTitle}` : "Starline 课后练习",
      path: this.data.homeworkId ? `/pages/answer/index?id=${encodeURIComponent(this.data.homeworkId)}` : "/pages/tasks/index"
    };
  },
  onUnload() {
    if (this.stopContentSecurity) {
      this.stopContentSecurity();
      this.stopContentSecurity = null;
    }
  },
  downloadHomework() {
    const downloadUrl = this.data.downloadUrl;
    if (!downloadUrl) {
      wx.showToast({ title: "当前习题没有开放下载", icon: "none" });
      return;
    }
    wx.showLoading({ title: "正在下载" });
    downloadWithAuth(stripApiPrefix(downloadUrl)).then((tempFilePath) => new Promise((resolve, reject) => {
      wx.saveFile({ tempFilePath, success: resolve, fail: reject });
    })).then(() => {
      wx.showToast({ title: "已保存习题", icon: "success" });
    }).catch((error) => {
      showFileError("习题下载失败", error);
    }).finally(() => wx.hideLoading());
  },
  refreshFavorite(homeworkId) {
    request("/student/favorites").then((favorites) => {
      const matched = (favorites || []).find(
        (item) => item.targetType === "homework" && item.targetId === homeworkId
      );
      this.setData({ favorited: !!matched, favoriteId: matched ? matched.id : "" });
    }).catch(() => {});
  },
  toggleFavorite() {
    if (!this.data.homeworkId) {
      return;
    }
    if (this.data.favorited && this.data.favoriteId) {
      request(`/student/favorites/${this.data.favoriteId}`, { method: "DELETE" })
        .then(() => {
          wx.showToast({ title: "已取消收藏", icon: "none" });
          this.setData({ favorited: false, favoriteId: "" });
        })
        .catch(() => {});
      return;
    }
    request("/student/favorites", {
      method: "POST",
      data: { targetType: "homework", targetId: this.data.homeworkId }
    })
      .then((favorite) => {
        wx.showToast({ title: "已收藏", icon: "success" });
        this.setData({ favorited: true, favoriteId: favorite.id });
      })
      .catch(() => {});
  },
  chooseOption(event) {
    const qindex = Number(event.currentTarget.dataset.qindex);
    const value = event.currentTarget.dataset.value;
    const questions = this.data.questions.map((question, index) => {
      if (index !== qindex) {
        return question;
      }
      if (question.type === "multiple") {
        const current = question.choices || [];
        const choices = current.includes(value) ? current.filter((item) => item !== value) : current.concat(value);
        return {
          ...question,
          choices,
          options: question.options.map((option) => ({ ...option, className: choices.includes(option.value) ? "active" : "" }))
        };
      }
      return {
        ...question,
        choice: value,
        options: question.options.map((option) => ({ ...option, className: option.value === value ? "active" : "" }))
      };
    });
    this.setData({ questions });
  },
  changeAnswer(event) {
    const qindex = Number(event.currentTarget.dataset.qindex);
    const questions = this.data.questions.map((question, index) =>
      index === qindex ? { ...question, text: event.detail.value } : question
    );
    this.setData({ questions });
  },
  saveDraft() {
    if (!this.data.homeworkId) {
      wx.showToast({ title: "题目信息缺失", icon: "none" });
      return;
    }
    wx.setStorageSync(draftKey(this.data.homeworkId), {
      savedAt: Date.now(),
      answers: this.data.questions.map((question) => ({
        questionId: question.id,
        choice: question.choice || "",
        choices: question.choices || [],
        text: question.text || ""
      }))
    });
    wx.showToast({ title: "草稿已保存", icon: "success" });
  },
  submit() {
    if (this.data.saving || this.data.isOverdue) {
      if (this.data.isOverdue) wx.showToast({ title: "本次练习已截止", icon: "none" });
      return;
    }
    const unanswered = this.data.questions.find((question) =>
      question.type === "single" || question.type === "judge" ? !question.choice : question.type === "multiple" ? !(question.choices || []).length : !question.text.trim()
    );
    if (unanswered) {
          wx.showToast({ title: "还有题目未完成", icon: "none" });
      return;
    }
    this.setData({ saving: true });
    request("/student/submissions", {
      method: "POST",
      data: {
        homeworkId: this.data.homeworkId,
        answers: this.data.questions.map((question) => ({
          questionId: question.id,
          choice: question.choice,
          choices: question.choices || [],
          text: question.text
        }))
      }
    })
      .then((res) => {
        wx.removeStorageSync(draftKey(this.data.homeworkId));
        wx.showToast({ title: "已提交", icon: "success" });
        wx.navigateTo({ url: `/pages/result/index?id=${res.submissionId}` });
      })
      .catch(() => {
        this.setData({ saving: false });
      });
  }
});

function letter(index) {
  return String.fromCharCode(65 + index);
}

function draftKey(homeworkId) {
  return `starline_homework_draft_${homeworkId}`;
}

function restoreDraftAnswers(homeworkId, questions) {
  const draft = wx.getStorageSync(draftKey(homeworkId));
  if (!draft || !Array.isArray(draft.answers)) {
    return questions;
  }
  const answerByQuestion = draft.answers.reduce((map, answer) => {
    map[answer.questionId] = answer;
    return map;
  }, {});
  return questions.map((question) => {
    const answer = answerByQuestion[question.id];
    if (!answer) {
      return question;
    }
    const choice = answer.choice || "";
    const choices = answer.choices || [];
    return {
      ...question,
      choice,
      choices,
      text: answer.text || "",
      options: question.options.map((option) => ({
        ...option,
        className: option.value === choice || choices.includes(option.value) ? "active" : ""
      }))
    };
  });
}

function buildWatermarks(text) {
  return Array.from({ length: 10 }).map(() => text);
}

function formatDeadline(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return `${date.getMonth() + 1}月${date.getDate()}日 ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function downloadWithAuth(path) {
  const app = getApp();
  return new Promise((resolve, reject) => {
    wx.downloadFile({
      url: `${app.globalData.apiBaseUrl}${path}`,
      header: {
        Authorization: wx.getStorageSync("starline_token") ? `Bearer ${wx.getStorageSync("starline_token")}` : ""
      },
      success(res) {
        if (res.statusCode !== 200) {
          reject(new Error(`习题请求失败（${res.statusCode}）`));
          return;
        }
        resolve(res.tempFilePath);
      },
      fail(err) {
        reject(new Error((err && err.errMsg) || "网络下载失败"));
      }
    });
  });
}

function showFileError(title, error) {
  const content = (error && error.message) || "请稍后重试";
  if (wx.showModal) {
    wx.showModal({ title, content, showCancel: false, confirmText: "知道了" });
    return;
  }
  wx.showToast({ title: content, icon: "none" });
}

function stripApiPrefix(path) {
  return String(path || "").replace(/^\/api/, "");
}
