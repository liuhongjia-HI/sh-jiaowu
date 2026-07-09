const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    emptyMessage: "请先登录绑定，或联系老师开通学习套餐。",
    keyword: "",
    home: null,
    hasContent: false,
    hasOpenedPackage: false,
    pendingTask: null,
    firstMaterial: null,
    continueCourse: null,
    progressPercent: 0,
    pendingCount: 0,
    materialCount: 0,
    noticeCount: 0,
    todoItems: [],
    feedbackItems: [],
    subscriptionReminder: null,
    subscribeEnabled: false,
    courseTitle: "待解锁学习星球",
    courseMeta: "",
    bannerTag: "继续学习",
    shortcuts: buildShortcuts(),
    contentItems: [],
    visibleItems: []
  },
  onLoad() {
    this.loadHome();
  },
  onShow() {
    if (!this.data.loading && !this.data.home) {
      this.loadHome();
    }
  },
  loadHome() {
    this.setData({ loading: true, error: "" });
    request("/student/home")
      .then((home) => {
        const continueCourse = home.continueCourse || {};
        const student = home.student || {};
        const pendingHomework = home.pendingHomework || [];
        const materials = home.materials || [];
        const notices = home.notices || [];
        const feedbackItems = home.classroomFeedback || [];
        const subscribeEnabled = wx.getStorageSync("starline_subscribe_enabled") === "1" || !!(home.subscriptionReminder && home.subscriptionReminder.enabled);
        const todoItems = decorateTodos(normalizeTodayTodos(home, {
          pendingHomework,
          continueCourse,
          subscribeEnabled
        }));
        const openedPackages = Array.isArray(student.openedPackages) ? student.openedPackages : [];
        const pendingTask = pendingHomework[0] || null;
        const firstMaterial = materials[0] || null;
        const hasContent = !!continueCourse.id || pendingHomework.length > 0 || materials.length > 0;
        const hasOpenedPackage = openedPackages.length > 0;
        const progressPercent = Number(home.continueProgress) || 0;
        const contentItems = buildContentItems(pendingHomework, materials, continueCourse);
        this.setData({
          home,
          hasContent,
          hasOpenedPackage,
          emptyMessage: homeEmptyMessage(hasOpenedPackage),
          continueCourse,
          pendingTask,
          firstMaterial,
          progressPercent,
          pendingCount: pendingHomework.length,
          materialCount: materials.length,
          noticeCount: notices.length,
          todoItems,
          feedbackItems,
          subscriptionReminder: decorateSubscription(home.subscriptionReminder, subscribeEnabled),
          subscribeEnabled,
          courseTitle: continueCourse.name || "待解锁学习星球",
          courseMeta: `${continueCourse.grade || ""} · ${continueCourse.subject || ""} · ${continueCourse.chapterCount || 0} 个章节`,
          bannerTag: pendingTask ? "今日题目" : "继续学习",
          contentItems,
          loading: false
        }, () => this.applySearch());
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
        emptyMessage: error.message || "请先登录绑定，或联系老师开通学习套餐。",
        hasContent: false,
        hasOpenedPackage: false,
        contentItems: [],
        visibleItems: [],
        todoItems: [],
        feedbackItems: [],
        loading: false
      }));
  },
  changeKeyword(event) {
    this.setData({ keyword: event.detail.value }, () => this.applySearch());
  },
  clearKeyword() {
    this.setData({ keyword: "" }, () => this.applySearch());
  },
  applySearch() {
    const keyword = (this.data.keyword || "").trim().toLowerCase();
    const visibleItems = this.data.contentItems.filter((item) => {
      if (!keyword) {
        return true;
      }
      return [item.title, item.meta, item.tag, item.typeName].join(" ").toLowerCase().includes(keyword);
    });
    this.setData({ visibleItems });
  },
  goStudyDetail() {
    if (!this.data.continueCourse || !this.data.continueCourse.id) {
      wx.switchTab({ url: "/pages/study/index" });
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${this.data.continueCourse.id}` });
  },
  goAnswer() {
    if (!this.data.pendingTask) {
      wx.navigateTo({ url: "/pages/tasks/index" });
      return;
    }
    wx.navigateTo({ url: `/pages/answer/index?id=${this.data.pendingTask.id}` });
  },
  goFirstMaterial() {
    if (!this.data.firstMaterial || !this.data.firstMaterial.id) {
      wx.showToast({ title: "老师发布资料后会显示在这里", icon: "none" });
      wx.switchTab({ url: "/pages/study/index" });
      return;
    }
    wx.navigateTo({ url: `/pages/material-preview/index?id=${this.data.firstMaterial.id}` });
  },
  stopCardTap() {},
  goTasks() {
    wx.navigateTo({ url: "/pages/tasks/index" });
  },
  goSchedule() {
    wx.navigateTo({ url: "/pages/schedule/index" });
  },
  goScores() {
    wx.navigateTo({ url: "/pages/scores/index" });
  },
  goLatestFeedback() {
    const latest = (this.data.feedbackItems || [])[0];
    if (!latest || !latest.relatedSubmissionId) {
      wx.showToast({ title: "老师完成批改后会显示课堂反馈", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/result/index?id=${latest.relatedSubmissionId}` });
  },
  goStudy() {
    wx.switchTab({ url: "/pages/study/index" });
  },
  goNotices() {
    wx.switchTab({ url: "/pages/notices/index" });
  },
  requestLearningSubscribe() {
    const app = getApp();
    const reminder = this.data.subscriptionReminder || {};
    const tmplIds = (reminder.templateIds || (app.globalData || {}).subscribeTemplateIds || []).filter(Boolean);
    if (!wx.requestSubscribeMessage || tmplIds.length === 0) {
      wx.showToast({ title: "提醒服务开通中，可先查看通知消息", icon: "none" });
      wx.switchTab({ url: "/pages/notices/index" });
      return;
    }
    wx.requestSubscribeMessage({
      tmplIds,
      success: (res) => {
        const acceptedIds = tmplIds.filter((id) => res[id] === "accept");
        if (acceptedIds.length > 0) {
          request("/student/subscription", {
            method: "POST",
            data: { templateIds: acceptedIds }
          }).then((reminder) => {
            wx.setStorageSync("starline_subscribe_enabled", "1");
            this.setData({
              subscribeEnabled: true,
              subscriptionReminder: decorateSubscription(reminder || this.data.subscriptionReminder, true),
              todoItems: this.data.todoItems.filter((item) => item.type !== "subscribe")
            });
            wx.showToast({ title: "已开启学习提醒", icon: "success" });
          }).catch((error) => {
            wx.showToast({ title: error.message || "提醒开启失败，请稍后再试", icon: "none" });
          });
          return;
        }
        wx.showToast({ title: "未开启提醒，可稍后再试", icon: "none" });
      },
      fail: () => wx.showToast({ title: "暂时无法开启提醒", icon: "none" })
    });
  },
  goLogin() {
    wx.navigateTo({ url: "/pages/login/index" });
  },
  goOpen() {
    if (!this.data.home) {
      this.goLogin();
      return;
    }
    if (!this.data.hasOpenedPackage) {
      wx.showToast({ title: "请联系老师或教务确认开通", icon: "none" });
      return;
    }
    wx.switchTab({ url: "/pages/study/index" });
  },
  handleShortcut(event) {
    const action = event.currentTarget.dataset.action;
    const handlers = {
      tasks: this.goTasks,
      materials: this.goFirstMaterial,
      schedule: this.goSchedule,
      feedback: this.goLatestFeedback,
      study: this.goStudy,
      scores: this.goScores,
      last: this.goAnswer,
      notices: this.goNotices
    };
    if (handlers[action]) {
      handlers[action].call(this);
    }
  },
  handleTodo(event) {
    const { type, path } = event.currentTarget.dataset;
    if (type === "subscribe") {
      this.requestLearningSubscribe();
      return;
    }
    navigateByPath(path);
  },
  goFeedback(event) {
    const id = event.currentTarget.dataset.id;
    if (!id) {
      wx.showToast({ title: "反馈记录缺失", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/result/index?id=${id}` });
  },
  goContent(event) {
    const { type, id } = event.currentTarget.dataset;
    if (!id) {
      wx.showToast({ title: "内容信息缺失", icon: "none" });
      return;
    }
    if (type === "task") {
      wx.navigateTo({ url: `/pages/answer/index?id=${id}` });
      return;
    }
    if (type === "material") {
      wx.navigateTo({ url: `/pages/material-preview/index?id=${id}` });
      return;
    }
    if (type === "course") {
      wx.navigateTo({ url: `/pages/study-detail/index?id=${id}` });
    }
  }
});

function decorateTodos(todos) {
  return (todos || []).map((item) => ({
    ...item,
    icon: todoIcon(item.type),
    className: `todo-${item.type || "default"}`
  }));
}

function normalizeTodayTodos(home, context) {
  const todos = Array.isArray(home.todayTodos) ? home.todayTodos : [];
  if (todos.length > 0) {
    return todos;
  }
  return buildFallbackTodos(context);
}

function buildFallbackTodos({ pendingHomework, continueCourse, subscribeEnabled }) {
  const homeworkTodos = (pendingHomework || []).slice(0, 3).map((item, index) => ({
    id: `todo-homework-${item.id || index}`,
    type: "homework",
    title: item.title || "待完成练习",
    summary: [item.course, item.deadline ? `截止 ${item.deadline}` : "", item.questionCount ? `${item.questionCount} 道题` : ""].filter(Boolean).join(" · "),
    actionText: "去完成",
    path: item.id ? `/pages/answer/index?id=${item.id}` : "/pages/tasks/index",
    priority: 100 - index,
    status: item.studentStatus || "待完成"
  }));
  const courseTodo = continueCourse && continueCourse.id ? [{
    id: `todo-study-${continueCourse.id}`,
    type: "schedule",
    title: "继续学习",
    summary: [continueCourse.name, continueCourse.grade, continueCourse.subject].filter(Boolean).join(" · "),
    actionText: "去学习",
    path: `/pages/study-detail/index?id=${continueCourse.id}`,
    priority: 60,
    status: "进行中"
  }] : [];
  const subscribeTodo = subscribeEnabled ? [] : [{
    id: "todo-subscribe-learning",
    type: "subscribe",
    title: "开启学习提醒",
    summary: "接收上课、作业和批改完成提醒，避免错过关键学习安排。",
    actionText: "开启提醒",
    priority: 50,
    status: "建议开启"
  }];
  return homeworkTodos.concat(courseTodo, subscribeTodo);
}

function decorateSubscription(reminder, enabled) {
  const item = reminder || {};
  const templateIds = Array.isArray(item.templateIds) ? item.templateIds.filter(Boolean) : [];
  return {
    ...item,
    templateIds,
    enabled,
    title: item.title || "学习提醒",
    summary: enabled ? "已开启学习提醒，上课、作业和批改结果会及时通知你。" : (item.summary || "提醒服务开通中，可先在通知消息查看学习安排。"),
    actionText: enabled ? "已开启" : (item.actionText || (templateIds.length > 0 ? "开启提醒" : "查看通知"))
  };
}

function todoIcon(type) {
  const icons = {
    homework: "练",
    schedule: "课",
    feedback: "评",
    subscribe: "醒"
  };
  return icons[type] || "待";
}

function navigateByPath(path) {
  if (!path) {
    wx.showToast({ title: "待办信息缺失", icon: "none" });
    return;
  }
  const tabPages = ["/pages/home/index", "/pages/study/index", "/pages/notices/index", "/pages/me/index"];
  if (tabPages.includes(path)) {
    wx.switchTab({ url: path });
    return;
  }
  wx.navigateTo({ url: path });
}

function buildShortcuts() {
  return [
    { label: "题库练习", action: "tasks", icon: "/assets/icons/shortcut-question.png" },
    { label: "学习资料", action: "materials", icon: "/assets/icons/shortcut-material.png" },
    { label: "课表", action: "schedule", icon: "/assets/icons/shortcut-schedule.png" },
    { label: "课堂反馈", action: "feedback", icon: "/assets/icons/shortcut-open.png" },
    { label: "学习中心", action: "study", icon: "/assets/icons/shortcut-study.png" },
    { label: "成绩报告", action: "scores", icon: "/assets/icons/shortcut-score.png" },
    { label: "上次练习", action: "last", icon: "/assets/icons/shortcut-last.png" },
    { label: "通知消息", action: "notices", icon: "/assets/icons/shortcut-notice.png" }
  ];
}

function buildContentItems(homework, materials, course) {
  const taskItems = (homework || []).map((item) => ({
    id: item.id,
    type: "task",
    typeName: "题库练习",
    title: item.title || "课后题目",
    meta: [item.course, item.packageName, item.deadline ? `截止 ${item.deadline}` : ""].filter(Boolean).join(" · "),
    tag: item.studentStatus || "待完成",
    icon: "题",
    className: "task"
  }));
  const materialItems = (materials || []).map((item) => ({
    id: item.id,
    type: "material",
    typeName: "学习资料",
    title: item.title || "学习资料",
    meta: [item.course, item.chapter, item.viewCount ? `${item.viewCount} 人学过` : ""].filter(Boolean).join(" · "),
    tag: item.type || "资料",
    icon: "资",
    className: "material"
  }));
  const courseItems = course && course.id ? [{
    id: course.id,
    type: "course",
    typeName: "学习中心",
    title: course.name || "继续学习",
    meta: [course.grade, course.subject, course.chapterCount ? `${course.chapterCount} 个章节` : ""].filter(Boolean).join(" · "),
    tag: "继续学习",
    icon: "课",
    className: "course"
  }] : [];
  return taskItems.concat(materialItems).concat(courseItems);
}

function homeEmptyMessage(hasOpenedPackage) {
  if (hasOpenedPackage) {
    return "学习套餐已开通，老师发布课程、学习资料或小挑战后会显示在这里。";
  }
  return "你的身份已绑定，暂时还没有开通学习套餐，请联系老师或教务确认开通。";
}
