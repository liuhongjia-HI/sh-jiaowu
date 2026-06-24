const { request } = require("../../utils/request");

Page({
  data: {
    homeworkId: "",
    taskTitle: "课后小挑战",
    deadlineText: "",
    rewardText: "做完就能获得新徽章",
    questions: [],
    favorited: false,
    favoriteId: "",
    saving: false
  },
  onLoad(options) {
    const id = options.id || "";
    if (!id) {
      wx.showToast({ title: "题目信息缺失", icon: "none" });
      return;
    }
    this.setData({ homeworkId: id });
    request(`/student/homework/${id}`).then((homework) => {
      const questions = (homework.questions || []).map((question, index) => ({
        ...question,
        index: index + 1,
        options: (question.options || []).map((text, optionIndex) => ({
          value: text,
          label: `${letter(optionIndex)}. ${text}`,
          className: ""
        })),
        choice: "",
        choices: [],
        text: ""
      }));
      this.setData({
        taskTitle: homework.title || "课后小挑战",
        deadlineText: homework.deadline ? `${homework.deadline} 前完成` : "",
        rewardText: homework.course || "做完就能获得新徽章",
        questions: restoreDraftAnswers(id, questions)
      });
    });
    this.refreshFavorite(id);
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
    if (this.data.saving) {
      return;
    }
    const unanswered = this.data.questions.find((question) =>
      question.type === "single" ? !question.choice : question.type === "multiple" ? !(question.choices || []).length : !question.text.trim()
    );
    if (unanswered) {
      wx.showToast({ title: "还有题目没有完成哦", icon: "none" });
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
