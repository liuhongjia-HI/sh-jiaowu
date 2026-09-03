const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    error: "",
    visitorMode: false,
    emptyMessage: "请先登录，或联系老师开通课程。",
    greeting: "你好",
    greetingName: "同学",
    keyword: "",
    home: null,
    hasContent: false,
    hasOpenedPackage: false,
    pendingTask: null,
    firstMaterial: null,
    continueCourse: null,
    courses: [],
    courseSlides: [],
    currentCourseIndex: 0,
    progressPercent: 0,
    pendingCount: 0,
    materialCount: 0,
    noticeCount: 0,
    todoItems: [],
    todoGroups: [],
    visibleTodoGroup: [],
    todoGroupIndex: 0,
    todoRotationState: "todo-fade-in",
    feedbackItems: [],
    subscriptionReminder: null,
    subscribeEnabled: false,
    courseTitle: "待开通课程",
    courseMeta: "",
    bannerTag: "继续学习",
    shortcuts: buildShortcuts(),
    recommendations: [],
    visibleRecommendations: [],
    recommendationsLoading: true,
    recommendationError: "",
    promoBanners: []
    ,launchCampaign: null, launchVisible: false, launchTimeOption: ""
  },
  onLoad() {
    this.refreshGreeting();
    if (!hasStudentToken()) {
      this.showVisitorHome();
      this.loadPromoBanners();
      return;
    }
    this.loadHome();
    this.loadPromoBanners();
  },
  onShareAppMessage() {
    const courseName = this.data.continueCourse && this.data.continueCourse.name;
    return {
      title: courseName ? `我正在 Starline 学习《${courseName}》` : "Starline 学习｜课后练习和学习反馈都在这里",
      path: "/pages/home/index"
    };
  },
  onShow() {
    this.refreshGreeting();
    if (!hasStudentToken()) {
      this.showVisitorHome();
      return;
    }
    if (this.data.visitorMode) {
      this.setData({ visitorMode: false });
      this.loadHome();
      this.loadPromoBanners();
      return;
    }
    if (!this.data.loading && !this.data.home) {
      this.loadHome();
    }
    this.startTodoRotation();
  },
  onHide() {
    this.stopTodoRotation();
  },
  onUnload() {
    this.stopTodoRotation();
  },
  showVisitorHome() {
    this.setData({
      loading: false,
      error: "",
      // 未登录用户也直接进入首页内容流，不再展示额外的欢迎卡片。
      // 保留访客态标记，登录后 onShow 可据此刷新真实学习数据。
      visitorMode: true,
      home: {},
      hasContent: false,
      hasOpenedPackage: false,
      courses: [],
      courseSlides: [],
      todoItems: [],
      todoGroups: [],
      visibleTodoGroup: [],
      todoGroupIndex: 0,
      feedbackItems: [],
      recommendations: [],
      visibleRecommendations: [],
      recommendationsLoading: false,
      promoBanners: []
    });
  },
  refreshGreeting(now = new Date()) {
    this.setData({ greeting: greetingForHour(now.getHours()) });
  },
  loadHome() {
    this.stopTodoRotation();
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
        const courses = normalizeHomeCourses(home, continueCourse);
        const selectedCourse = courses[0] || {};
        const courseSlides = buildCourseSlides(courses, pendingTask);
        const hasContent = courses.length > 0 || pendingHomework.length > 0 || materials.length > 0;
        const hasOpenedPackage = openedPackages.length > 0;
        const progressPercent = clampProgress(selectedCourse.progress);
        this.setData({
          home,
          visitorMode: false,
          greetingName: preferredGreetingName(student),
          courses,
          courseSlides,
          currentCourseIndex: 0,
          hasContent,
          hasOpenedPackage,
          emptyMessage: homeEmptyMessage(hasOpenedPackage),
          continueCourse: selectedCourse,
          pendingTask,
          firstMaterial,
          progressPercent,
          pendingCount: pendingHomework.length,
          materialCount: materials.length,
          noticeCount: notices.length,
          todoItems,
          todoGroups: buildTodoGroups(todoItems),
          visibleTodoGroup: buildTodoGroups(todoItems)[0] || [],
          todoGroupIndex: 0,
          feedbackItems,
          subscriptionReminder: decorateSubscription(home.subscriptionReminder, subscribeEnabled),
          subscribeEnabled,
          courseTitle: selectedCourse.name || "待开通课程",
          courseMeta: formatCourseMeta(selectedCourse),
          bannerTag: pendingTask ? "今日题目" : "继续学习",
          loading: false
        }, () => {
          this.startTodoRotation();
          this.loadRecommendations();
        });
        this.loadLaunchCampaign(student);
      })
      .catch((error) => this.setData({
        error: error.message || "加载失败",
          emptyMessage: error.message || "请先登录，或联系老师开通课程。",
        hasContent: false,
        hasOpenedPackage: false,
        recommendations: [],
        visibleRecommendations: [],
        recommendationsLoading: false,
        todoItems: [],
        todoGroups: [],
        visibleTodoGroup: [],
        todoGroupIndex: 0,
        feedbackItems: [],
        loading: false
      }));
  },
  loadLaunchCampaign(student) {
    const campaignKeyBase = `starline_launch_seen_${student.id || "current"}`;
    const seenKey = wx.getStorageSync(`${campaignKeyBase}_meta`);
    const today = new Date().toISOString().slice(0, 10);
    if (seenKey === "once" || seenKey === `daily:${today}`) return;
    request("/student/launch-campaign", { silent: true }).then((campaign) => {
      if (campaign) this.setData({ launchCampaign: campaign, launchVisible: true });
    }).catch(() => {});
  },
  closeLaunchCampaign() { const id=(this.data.home&&this.data.home.student&&this.data.home.student.id)||"current"; const frequency=(this.data.launchCampaign&&this.data.launchCampaign.frequency)||"once"; wx.setStorageSync(`starline_launch_seen_${id}_meta`, frequency === "daily" ? `daily:${new Date().toISOString().slice(0,10)}` : frequency === "every_entry" ? "" : "once"); this.setData({ launchVisible:false }); },
  chooseLaunchTime(e) { this.setData({ launchTimeOption: e.detail.value }); },
  submitLaunchCampaign() { const c=this.data.launchCampaign||{}; if (c.actionType !== "submit_reservation") { this.closeLaunchCampaign(); return; } request("/student/class-reservations",{method:"POST",data:{campaignId:c.id,timeOption:this.data.launchTimeOption}}).then(()=>{wx.showToast({title:"已提交预约",icon:"success"});this.closeLaunchCampaign();}).catch(e=>wx.showToast({title:e.message||"提交失败",icon:"none"})); },
  changeCourse(event) {
    const index = Number(event.detail.current) || 0;
    const selectedCourse = this.data.courses[index] || {};
    this.setData({
      currentCourseIndex: index,
      continueCourse: selectedCourse,
      progressPercent: Number(selectedCourse.progress) || 0,
          courseTitle: selectedCourse.name || "待开通课程",
      courseMeta: formatCourseMeta(selectedCourse),
      bannerTag: index === 0 && this.data.pendingTask ? "今日题目" : "继续学习"
    });
  },
  changeKeyword(event) {
    this.setData({ keyword: event.detail.value }, () => this.applySearch());
  },
  clearKeyword() {
    this.setData({ keyword: "" }, () => this.applySearch());
  },
  applySearch() {
    const keyword = (this.data.keyword || "").trim().toLowerCase();
    const visibleRecommendations = this.data.recommendations.filter((item) => {
      if (!keyword) {
        return true;
      }
      return [item.packageName, item.subject, item.grade, item.semester, item.summary, ...(item.contentSamples || [])]
        .join(" ")
        .toLowerCase()
        .includes(keyword);
    });
    this.setData({ visibleRecommendations });
  },
  loadRecommendations() {
    this.setData({ recommendationsLoading: true, recommendationError: "" });
    request("/student/recommendations", { silent: true })
      .then((recommendations) => {
        this.setData({
          recommendations: (Array.isArray(recommendations) ? recommendations : []).map((item) => ({
            ...item,
            contentSampleText: (item.contentSamples || []).join("、")
          })),
          recommendationsLoading: false
        }, () => this.applySearch());
      })
      .catch((error) => this.setData({
        recommendations: [],
        visibleRecommendations: [],
        recommendationError: error.message || "推荐课程加载失败，请稍后重试。",
        recommendationsLoading: false
      }));
  },
  // 轮播图是纯展示的运营位，加载失败不该挡住首页其余内容，所以用 silent 请求，
  // 失败就悄悄清空、不弹错误提示，也不影响 loadHome 那条主链路。
  loadPromoBanners() {
    // 运营位与登录、年级无关；skipAuth 让未登录访客也能读取公开接口。
    request("/student/banners", { silent: true, skipAuth: true })
      .then((banners) => {
        this.setData({
          promoBanners: (Array.isArray(banners) ? banners : []).map((item) => ({
            ...item,
            imageUrl: normalizeBannerImageUrl(item.imageUrl)
          }))
        });
      })
      .catch(() => this.setData({ promoBanners: [] }));
  },
  handlePromoBannerTap(event) {
    const id = event.currentTarget.dataset.id;
    const banner = (this.data.promoBanners || []).find((item) => item.id === id);
    if (!banner || banner.linkType === "none" || !banner.linkValue) {
      return;
    }
    if (banner.linkType === "page") {
      navigateByPath(banner.linkValue);
      return;
    }
    if (banner.linkType === "url") {
      // 小程序不能直接跳外部网页：没有配置业务域名和 web-view 页面时，
      // 复制链接到剪贴板是唯一能让用户实际打开这个地址的办法。
      wx.setClipboardData({
        data: banner.linkValue,
        success: () => wx.showToast({ title: "链接已复制，请在浏览器打开", icon: "none" })
      });
    }
  },
  goStudyDetail(event) {
    const dataset = event && event.currentTarget && event.currentTarget.dataset;
    const courseId = (dataset && dataset.courseId) || (this.data.continueCourse && this.data.continueCourse.id);
    if (!courseId) {
      wx.switchTab({ url: "/pages/study/index" });
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${courseId}` });
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
      wx.showToast({ title: "老师发布讲义后会显示在这里", icon: "none" });
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
      wx.showToast({ title: "老师批改后会显示反馈", icon: "none" });
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
              todoItems: this.data.todoItems.filter((item) => item.type !== "subscribe"),
              todoGroups: buildTodoGroups(this.data.todoItems.filter((item) => item.type !== "subscribe")),
              visibleTodoGroup: buildTodoGroups(this.data.todoItems.filter((item) => item.type !== "subscribe"))[0] || [],
              todoGroupIndex: 0
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
  startTodoRotation() {
    this.stopTodoRotation();
    if (!this.data.todoGroups || this.data.todoGroups.length <= 1) return;
    this.todoRotationTimer = setInterval(() => {
      const groups = this.data.todoGroups || [];
      if (groups.length <= 1) return;
      const nextIndex = (this.data.todoGroupIndex + 1) % groups.length;
      this.setData({ todoRotationState: "todo-fade-out" }, () => {
        this.todoRotationSwapTimer = setTimeout(() => {
          this.setData({ todoGroupIndex: nextIndex, todoRotationState: "todo-fade-in", visibleTodoGroup: groups[nextIndex] });
          this.todoRotationSwapTimer = null;
        }, 220);
      });
    }, 3500);
    // Node 测试环境下不让展示定时器阻止进程退出；微信运行时没有 unref，行为不受影响。
    if (this.todoRotationTimer && typeof this.todoRotationTimer.unref === "function") {
      this.todoRotationTimer.unref();
    }
  },
  stopTodoRotation() {
    if (this.todoRotationTimer) {
      clearInterval(this.todoRotationTimer);
      this.todoRotationTimer = null;
    }
    if (this.todoRotationSwapTimer) {
      clearTimeout(this.todoRotationSwapTimer);
      this.todoRotationSwapTimer = null;
    }
  },
  goFeedback(event) {
    const id = event.currentTarget.dataset.id;
    if (!id) {
      wx.showToast({ title: "反馈记录缺失", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/result/index?id=${id}` });
  },
  showRecommendation(event) {
    const packageId = event.currentTarget.dataset.packageId;
    const recommendation = this.data.recommendations.find((item) => item.packageId === packageId);
    if (!recommendation) {
      wx.showToast({ title: "套餐信息缺失", icon: "none" });
      return;
    }
    wx.showModal({
      title: recommendation.packageName,
      content: recommendation.summary || "该套餐包含学习内容和资料，开通后即可使用。",
      showCancel: false,
      confirmText: "我知道了"
    });
  },
  contactTeacher(event) {
    const name = event.currentTarget.dataset.name || "该套餐";
    wx.showModal({
      title: "联系老师",
      content: `请联系老师或教务开通“${name}”。开通后，学习内容和资料会自动出现在学习中心。`,
      showCancel: false,
      confirmText: "我知道了"
    });
  }
});

function hasStudentToken() {
  return Boolean(wx.getStorageSync && wx.getStorageSync("starline_token"));
}

function greetingForHour(hour) {
  if (hour >= 5 && hour < 11) return "早上好";
  if (hour >= 11 && hour < 13) return "中午好";
  if (hour >= 13 && hour < 18) return "下午好";
  return "晚上好";
}

function preferredGreetingName(student) {
  const nickname = String(student.nickname || "").trim();
  if (nickname && nickname !== "微信用户") return nickname;
  return String(student.name || "").trim() || "同学";
}

function decorateTodos(todos) {
  return (todos || []).map((item) => ({
    ...item,
    icon: todoIcon(item.type),
    className: `todo-${item.type || "default"}`
  }));
}

function buildTodoGroups(todos) {
  const items = Array.isArray(todos) ? todos : [];
  const groups = [];
  for (let index = 0; index < items.length; index += 2) {
    groups.push(items.slice(index, index + 2));
  }
  return groups;
}

function normalizeHomeCourses(home, continueCourse) {
  const courses = Array.isArray(home.courses) ? home.courses : [];
  if (courses.length > 0) {
    return courses.map((course) => ({
      ...course,
      progress: clampProgress(course.progress)
    }));
  }
  if (continueCourse && continueCourse.id) {
    return [{ ...continueCourse, progress: clampProgress(home.continueProgress) }];
  }
  return [];
}

function buildCourseSlides(courses, pendingTask) {
  if (!courses.length) {
    return [{
      id: "empty-course",
      name: "待开通课程",
      progress: 0,
      hasCourse: false,
      meta: "",
      bannerTag: "继续学习",
      actionText: "查看学习中心"
    }];
  }
  return courses.map((course, index) => ({
    ...course,
    hasCourse: true,
    meta: formatCourseMeta(course),
    bannerTag: index === 0 && pendingTask ? "今日题目" : "继续学习",
    actionText: "继续学习"
  }));
}

function formatCourseMeta(course = {}) {
  const chapterCount = Number(course.chapterCount) || countChapters(course.curriculum) || Number(course.lessonCount) || 0;
  return [course.grade, course.subject, `${chapterCount} 个章节`].filter(Boolean).join(" · ");
}

function countChapters(curriculum) {
  if (!Array.isArray(curriculum)) return 0;
  return curriculum.filter((node) => node && node.type === "chapter").length;
}

function clampProgress(value) {
  const progress = Number(value) || 0;
  return Math.max(0, Math.min(100, progress));
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
    actionText: "开始练习",
    path: item.id ? `/pages/answer/index?id=${item.id}` : "/pages/tasks/index",
    priority: 100 - index,
    status: item.studentStatus || "待完成"
  }));
  const courseTodo = continueCourse && continueCourse.id ? [{
    id: `todo-study-${continueCourse.id}`,
    type: "schedule",
    title: "继续学习",
    summary: [continueCourse.name, continueCourse.grade, continueCourse.subject].filter(Boolean).join(" · "),
    actionText: "继续学习",
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

// 后端返回的图片地址是相对服务器根路径的绝对路径（如 /api/banners/images/xxx），
// image 组件必须给完整地址才能加载，同款转换逻辑在 pages/me/index.js 里也有一份
// （那边转的是头像），两处独立维护是因为这个仓库里资源地址转换一直是各页面自己内联一份，
// 不是抽公共方法——跟着现有约定走，不引入新的共享模块。
function normalizeBannerImageUrl(value) {
  const text = String(value || "").trim();
  if (!text || /^https?:\/\//i.test(text)) {
    return text;
  }
  if (text.indexOf("/api/") === 0) {
    const app = typeof getApp === "function" ? getApp() : null;
    const baseUrl = app && app.globalData ? String(app.globalData.apiBaseUrl || "").replace(/\/$/, "") : "";
    if (baseUrl.endsWith("/api")) {
      return `${baseUrl.slice(0, -4)}${text}`;
    }
    return baseUrl ? `${baseUrl}${text}` : text;
  }
  return text;
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
    { label: "上次练习", action: "last", icon: "/assets/icons/shortcut-last.png" },
    { label: "通知消息", action: "notices", icon: "/assets/icons/shortcut-notice.png" },
    { label: "学习中心", action: "study", icon: "/assets/icons/shortcut-study.png" },
    { label: "成绩报告", action: "scores", icon: "/assets/icons/shortcut-score.png" },
    { label: "课表", action: "schedule", icon: "/assets/icons/shortcut-schedule.png" },
    { label: "课堂反馈", action: "feedback", icon: "/assets/icons/shortcut-open.png" }
  ];
}

function homeEmptyMessage(hasOpenedPackage) {
  if (hasOpenedPackage) {
    return "课程已开通，内容发布后会显示在这里。";
  }
  return "暂时还没有开通课程，请联系老师确认。";
}
