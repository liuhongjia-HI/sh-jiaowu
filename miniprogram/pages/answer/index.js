const { request } = require("../../utils/request");
const { activateContentSecurity } = require("../../utils/content-security");

Page({
  data: {
    homeworkId: "",
    taskTitle: "Exercises",
    deadlineText: "",
    rewardText: "Complete exercises to earn badges",
    questions: [],
    downloadUrl: "",
    watermarkText: "Loading watermark",
    watermarkTexts: ["Loading watermark", "Loading watermark", "Loading watermark", "Loading watermark", "Loading watermark", "Loading watermark"],
    securityNotice: "For your personal study only. Please do not share.",
    favorited: false,
    favoriteId: "",
    saving: false,
    isOverdue: false
  },
  onLoad(options) {
    const id = options.id || "";
    if (!id) {
      wx.showToast({ title: "Question information missing", icon: "none" });
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
          label: `${letter(optionIndex)}. ${question.type === "judge" ? ({ "正确": "True", "错误": "False" }[text] || text) : text}`,
          className: ""
        })),
        choice: "",
        choices: [],
        text: ""
      }));
      const watermarkText = homework.watermarkText || "Loading watermark";
      this.setData({
        taskTitle: `${homework.assessmentType === "mock_exam" ? "Mock Exam · " : "Exercise · "}${homework.title || "Exercises"}`,
        deadlineText: homework.isOverdue ? "Closed" : (homework.deadlineAt ? `Due ${formatDeadline(homework.deadlineAt)}` : (homework.deadline ? `Due ${homework.deadline}` : "")),
        rewardText: homework.course || "Complete exercises to earn badges",
        questions: restoreDraftAnswers(id, questions),
        downloadUrl: homework.downloadUrl || "",
        watermarkText,
        watermarkTexts: buildWatermarks(watermarkText),
        securityNotice: homework.securityNotice || "For your personal study only. Please do not share.",
        isOverdue: Boolean(homework.isOverdue)
      });
    }).catch(() => {
      this.setData({
        rewardText: "Failed to load questions",
        securityNotice: "Failed to load questions. Please reopen this page."
      });
    });
    this.refreshFavorite(id);
  },
  onShareAppMessage() {
    return {
      title: this.data.taskTitle ? `Starline Exercise: ${this.data.taskTitle}` : "Starline Exercises",
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
      wx.showToast({ title: "Downloads are not enabled for this exercise", icon: "none" });
      return;
    }
    wx.showLoading({ title: "Downloading" });
    downloadWithAuth(stripApiPrefix(downloadUrl)).then((tempFilePath) => new Promise((resolve, reject) => {
      wx.saveFile({ tempFilePath, success: resolve, fail: reject });
    })).then(() => {
      wx.showToast({ title: "Exercise saved", icon: "success" });
    }).catch((error) => {
      showFileError("Failed to download exercise", error);
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
          wx.showToast({ title: "Removed from favorites", icon: "none" });
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
        wx.showToast({ title: "Added to favorites", icon: "success" });
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
      wx.showToast({ title: "Question information missing", icon: "none" });
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
    wx.showToast({ title: "Draft saved", icon: "success" });
  },
  submit() {
    if (this.data.saving || this.data.isOverdue) {
      if (this.data.isOverdue) wx.showToast({ title: "This exercise is closed", icon: "none" });
      return;
    }
    const unanswered = this.data.questions.find((question) =>
      question.type === "single" || question.type === "judge" ? !question.choice : question.type === "multiple" ? !(question.choices || []).length : !question.text.trim()
    );
    if (unanswered) {
          wx.showToast({ title: "Please answer all questions", icon: "none" });
      return;
    }
    const answers = this.data.questions.map((question) => ({
      questionId: question.id, choice: question.choice, choices: question.choices || [], text: question.text
    }));
    const pendingKey = submissionRequestKey(this.data.homeworkId);
    const signature = JSON.stringify(answers);
    const savedRequest = (wx.getStorageSync && wx.getStorageSync(pendingKey)) || this._submissionRequest;
    const pending = savedRequest && savedRequest.signature === signature ? savedRequest : { id: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`, signature };
    this._submissionRequest = pending;
    if (wx.setStorageSync) wx.setStorageSync(pendingKey, pending);
    this.setData({ saving: true });
    request("/student/submissions", {
      method: "POST", data: { homeworkId: this.data.homeworkId, requestId: pending.id, answers }
    })
      .then((res) => {
        wx.removeStorageSync(draftKey(this.data.homeworkId));
        wx.removeStorageSync(pendingKey);
        this._submissionRequest = null;
        wx.showToast({ title: "Submitted", icon: "success" });
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
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
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
          reject(new Error(`Exercise request failed (${res.statusCode})`));
          return;
        }
        resolve(res.tempFilePath);
      },
      fail(err) {
        reject(new Error((err && err.errMsg) || "Download failed"));
      }
    });
  });
}

function showFileError(title, error) {
  const content = (error && error.message) || "Please try again later";
  if (wx.showModal) {
    wx.showModal({ title, content, showCancel: false, confirmText: "OK" });
    return;
  }
  wx.showToast({ title: content, icon: "none" });
}

function stripApiPrefix(path) {
  return String(path || "").replace(/^\/api/, "");
}

function submissionRequestKey(homeworkId) {
  const studentId = wx.getStorageSync ? wx.getStorageSync("starline_student_id") || "" : "";
  return `starline_submission_request_${studentId}_${homeworkId}`;
}
