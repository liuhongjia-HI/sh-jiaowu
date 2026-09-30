const { refreshNoticeBadge } = require("../../utils/notice-badge");
const { request } = require("../../utils/request");
const { subjectLabel, subjectsMatchName } = require("../../utils/subject");

const ONBOARDING_SEEN_KEY = "starline_onboarding_seen";

Page({
  data: {
    loading: true,
    error: "",
    visitorMode: false,
    emptyMessage: "Log in or contact your teacher to activate courses.",
    greeting: "Hello",
    greetingName: "Student",
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
    openedSubjectCount: 0,
    noticeCount: 0,
    todoItems: [],
    todoGroups: [],
    visibleTodoGroup: [],
    todoGroupIndex: 0,
    todoRotationState: "todo-fade-in",
    feedbackItems: [],
    subscriptionReminder: null,
    subscribeEnabled: false,
    showSubscribePrompt: false,
    courseTitle: "No Active Courses",
    courseMeta: "",
    bannerTag: "Continue Learning",
    shortcuts: buildShortcuts(),
    recommendations: [],
    visibleRecommendations: [],
    recommendationIndex: 0,
    displayedRecommendations: [],
    recommendationsLoading: true,
    recommendationError: "",
    promoBanners: [],
    onboardingRedirected: false
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
      title: courseName ? `I'm learning with Starline: ${courseName}` : "Starline Learning: Exercises and Feedback",
      path: "/pages/home/index"
    };
  },
  onShow() {
    refreshNoticeBadge();
    this.homeHidden = false;
    this.refreshGreeting();
    if (!hasStudentToken()) {
      if (!this.data.visitorMode) {
        this.showVisitorHome();
      }
      this.redirectToParentOnboarding();
      return;
    }
    if (this.data.onboardingRedirected) {
      this.setData({ onboardingRedirected: false });
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
    this.loadRecommendations();
  },
  onHide() {
    this.homeHidden = true;
    this.stopTodoRotation();
    this.stopRecommendationRotation();
  },
  onUnload() {
    this.homeHidden = true;
    this.stopTodoRotation();
    this.stopRecommendationRotation();
  },
  showVisitorHome() {
    this.setData({
      loading: false,
      error: "",
      // 未登录用户停在访客首页，保留添加学生入口，登录后 onShow 再刷新学习数据。
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
      showSubscribePrompt: false,
      recommendations: [],
      visibleRecommendations: [],
      recommendationsLoading: false,
      promoBanners: [],
      openedSubjectCount: 0
    });
  },
  redirectToParentOnboarding() {
    if (this.data.onboardingRedirected || hasSeenParentOnboarding()) {
      return;
    }
    if (typeof wx.navigateTo !== "function") {
      return;
    }
    // 只引导一次，且必须保留首页栈。审核员点击左上角首页/返回后要能停在访客首页，
    // 不能再用 reLaunch 把页面栈清掉后循环拉回登录引导。
    markParentOnboardingSeen();
    this.setData({ onboardingRedirected: true });
    wx.navigateTo({ url: "/pages/parent-onboarding/index" });
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
        const todoItems = excludeSubscribeTodos(decorateTodos(normalizeTodayTodos(home, {
          pendingHomework,
          continueCourse,
          subscribeEnabled
        })));
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
          openedSubjectCount: countOpenedSubjects(student, courses),
          noticeCount: notices.length,
          todoItems,
          todoGroups: buildTodoGroups(todoItems),
          visibleTodoGroup: buildTodoGroups(todoItems)[0] || [],
          todoGroupIndex: 0,
          feedbackItems,
          subscriptionReminder: decorateSubscription(home.subscriptionReminder, subscribeEnabled),
          subscribeEnabled,
          showSubscribePrompt: !subscribeEnabled,
          courseTitle: selectedCourse.name || "No Active Courses",
          courseMeta: formatCourseMeta(selectedCourse),
          bannerTag: pendingTask ? "Today's Exercise" : "Continue Learning",
          loading: false
        }, () => {
          this.startTodoRotation();
          this.loadRecommendations();
        });
        this.loadLaunchCampaign(student);
      })
      .catch((error) => this.setData({
        error: error.message || "Failed to load",
          emptyMessage: error.message || "Log in or contact your teacher to activate courses.",
        hasContent: false,
        hasOpenedPackage: false,
        openedSubjectCount: 0,
        recommendations: [],
        visibleRecommendations: [],
        recommendationsLoading: false,
        todoItems: [],
        todoGroups: [],
        visibleTodoGroup: [],
        todoGroupIndex: 0,
        feedbackItems: [],
        showSubscribePrompt: false,
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
  submitLaunchCampaign() { const c=this.data.launchCampaign||{}; if (c.actionType !== "submit_reservation") { this.closeLaunchCampaign(); return; } request("/student/class-reservations",{method:"POST",data:{campaignId:c.id,timeOption:this.data.launchTimeOption}}).then(()=>{wx.showToast({title:"Reservation submitted",icon:"success"});this.closeLaunchCampaign();}).catch(e=>wx.showToast({title:e.message||"Submission failed",icon:"none"})); },
  changeCourse(event) {
    const index = Number(event.detail.current) || 0;
    const selectedCourse = this.data.courses[index] || {};
    this.setData({
      currentCourseIndex: index,
      continueCourse: selectedCourse,
      progressPercent: Number(selectedCourse.progress) || 0,
      courseTitle: selectedCourse.name || "No Active Courses",
      courseMeta: formatCourseMeta(selectedCourse),
      bannerTag: index === 0 && this.data.pendingTask ? "Today's Exercise" : "Continue Learning"
    });
  },
  changeKeyword(event) {
    this.setData({ keyword: event.detail.value }, () => this.applySearch());
  },
  clearKeyword() {
    this.setData({ keyword: "" }, () => this.applySearch());
  },
  startRecommendationRotation() {
    this.stopRecommendationRotation();
    if (this.homeHidden || (this.data.visibleRecommendations || []).length <= 2) return;
    this.recommendationRotationTimer = setInterval(() => {
      const list = this.data.visibleRecommendations || [];
      const index = ((this.data.recommendationIndex || 0) + 2) % list.length;
      this.setData({ recommendationIndex: index, displayedRecommendations: list.slice(index, index + 2).concat(index + 2 >= list.length ? list.slice(0, Math.max(0, index + 2 - list.length)) : []) });
    }, 4200);
  },
  stopRecommendationRotation() {
    if (this.recommendationRotationTimer) clearInterval(this.recommendationRotationTimer);
    this.recommendationRotationTimer = null;
  },
  applySearch() {
    const keyword = (this.data.keyword || "").trim().toLowerCase();
    const visibleRecommendations = this.data.recommendations.filter((item) => {
      if (!keyword) {
        return true;
      }
      return [item.subject, item.displayName, item.grade, item.teacherName, item.teacherIntro]
        .join(" ")
        .toLowerCase()
        .includes(keyword);
    });
    this.setData({ visibleRecommendations, recommendationIndex: 0, displayedRecommendations: visibleRecommendations.slice(0, 2) }, () => this.startRecommendationRotation());
  },
  loadRecommendations() {
    this.stopRecommendationRotation();
    this.setData({ recommendationsLoading: true, recommendationError: "" });
    request("/student/recommendations", { silent: true })
      .then((recommendations) => {
        this.setData({
          recommendations: (Array.isArray(recommendations) ? recommendations : []).map((item) => ({
            ...item,
            displayName: subjectLabel(item.subject),
            contentSampleText: (item.contentSamples || []).join("、")
          })),
          recommendationsLoading: false
        }, () => this.applySearch());
      })
      .catch((error) => this.setData({
        recommendations: [],
        visibleRecommendations: [],
        recommendationError: error.message || "Failed to load recommendations. Try again later.",
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
        success: () => wx.showToast({ title: "Link copied. Open it in your browser.", icon: "none" })
      });
    }
  },
  goStudyDetail(event) {
    const dataset = event && event.currentTarget && event.currentTarget.dataset;
    const courseId = (dataset && dataset.courseId) || (this.data.continueCourse && this.data.continueCourse.id);
    if (!courseId) {
      this.openLearningPlanet();
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${courseId}` });
  },
  openLearningPlanet() {
    wx.showModal({
      title: "Activate Courses",
      content: "Contact your teacher or school office to activate access. Courses, materials, and exercises will then appear in the Learning Center.",
      showCancel: false,
      confirmText: "OK"
    });
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
      wx.showToast({ title: "Materials will appear once your teacher publishes them.", icon: "none" });
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
      wx.showToast({ title: "Feedback will appear after your teacher reviews your work.", icon: "none" });
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
      wx.showToast({ title: "Reminders are being set up. Check Messages for now.", icon: "none" });
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
            const todoItems = excludeSubscribeTodos(this.data.todoItems);
            this.setData({
              subscribeEnabled: true,
              showSubscribePrompt: false,
              subscriptionReminder: decorateSubscription(reminder || this.data.subscriptionReminder, true),
              todoItems,
              todoGroups: buildTodoGroups(todoItems),
              visibleTodoGroup: buildTodoGroups(todoItems)[0] || [],
              todoGroupIndex: 0
            });
            wx.showToast({ title: "Learning reminders enabled", icon: "success" });
          }).catch((error) => {
            wx.showToast({ title: error.message || "Failed to enable reminders. Try again later.", icon: "none" });
          });
          return;
        }
        wx.showToast({ title: "Reminders not enabled. Try again later.", icon: "none" });
      },
      fail: () => wx.showToast({ title: "Reminders unavailable", icon: "none" })
    });
  },
  goLogin() {
    wx.navigateTo({ url: "/pages/login/index" });
  },
  goAddStudent() {
    wx.navigateTo({ url: "/pages/parent-onboarding/index" });
  },
  goOpen() {
    if (!this.data.home) {
      this.goLogin();
      return;
    }
    if (!this.data.hasOpenedPackage) {
      wx.showToast({ title: "Contact your teacher or school office to activate access.", icon: "none" });
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
      wx.showToast({ title: "Feedback not found", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/result/index?id=${id}` });
  },
  showRecommendation(event) {
    const subject = event.currentTarget.dataset.subject;
    const recommendation = this.data.recommendations.find((item) => item.subject === subject);
    if (!recommendation) {
      wx.showToast({ title: "Subject information missing", icon: "none" });
      return;
    }
    wx.showModal({
      title: recommendation.subject,
      content: `${recommendation.courseCount} courses · ${recommendation.materialCount} materials · ${recommendation.questionCount} questions · ${recommendation.homeworkCount} exercises\nTeacher: ${recommendation.teacherName || "Not assigned"}\n${recommendation.teacherIntro || ""}`,
      showCancel: false,
      confirmText: "OK"
    });
  },
  contactTeacher(event) {
    const name = event.currentTarget.dataset.name || "this subject";
    wx.showModal({
      title: "Contact Teacher",
      content: `Contact your teacher or school office to activate ${name}. Courses, materials, and exercises will then appear in the Learning Center.`,
      showCancel: false,
      confirmText: "OK"
    });
  }
});

function hasStudentToken() {
  return Boolean(wx.getStorageSync && wx.getStorageSync("starline_token"));
}

function hasSeenParentOnboarding() {
  return Boolean(wx.getStorageSync && wx.getStorageSync(ONBOARDING_SEEN_KEY));
}

function markParentOnboardingSeen() {
  if (wx.setStorageSync) {
    wx.setStorageSync(ONBOARDING_SEEN_KEY, "1");
  }
}

function greetingForHour(hour) {
  if (hour >= 5 && hour < 11) return "Good morning";
  if (hour >= 11 && hour < 13) return "Hello";
  if (hour >= 13 && hour < 18) return "Good afternoon";
  return "Good evening";
}

function preferredGreetingName(student) {
  const nickname = String(student.nickname || "").trim();
  if (nickname && nickname !== "微信用户") return nickname;
  return String(student.name || "").trim() || "Student";
}

function decorateTodos(todos) {
  return (todos || []).map((item) => ({
    ...item,
    icon: todoIcon(item.type),
    className: `todo-${item.type || "default"}`
  }));
}

function excludeSubscribeTodos(todos) {
  return (todos || []).filter((item) => item && item.type !== "subscribe");
}

function buildTodoGroups(todos) {
  const items = Array.isArray(todos) ? todos : [];
  const groups = [];
  for (let index = 0; index < items.length; index += 2) {
    groups.push(items.slice(index, index + 2));
  }
  return groups;
}

function countOpenedSubjects(student, courses) {
  const fromSubjects = uniqueSubjectCount(Array.isArray(student && student.openedSubjects) ? student.openedSubjects : []);
  if (fromSubjects > 0) {
    return fromSubjects;
  }
  const fromCourses = uniqueSubjectCount((courses || []).map((course) => course && course.subject));
  if (fromCourses > 0) {
    return fromCourses;
  }
  const packages = Array.isArray(student && student.openedPackages) ? student.openedPackages : [];
  return packages.length;
}

function uniqueSubjectCount(values) {
  const subjects = [];
  (values || []).forEach((value) => {
    const subject = String(value || "").trim();
    if (!subject || subjects.some((item) => subjectsMatchName(item, subject))) {
      return;
    }
    subjects.push(subject);
  });
  return subjects.length;
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
      name: "No Active Courses",
      progress: 0,
      hasCourse: false,
      meta: "No active learning plan. Contact your teacher or school office.",
      bannerTag: "Continue Learning",
      actionText: "Activate Courses"
    }];
  }
  return courses.map((course, index) => ({
    ...course,
    hasCourse: true,
    meta: formatCourseMeta(course),
    bannerTag: index === 0 && pendingTask ? "Today's Exercise" : "Continue Learning",
    actionText: "Continue Learning"
  }));
}

function formatCourseMeta(course = {}) {
  const chapterCount = Number(course.chapterCount) || countChapters(course.curriculum) || Number(course.lessonCount) || 0;
  return [course.grade, subjectLabel(course.subject), `${chapterCount} chapters`].filter(Boolean).join(" · ");
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

function buildFallbackTodos({ pendingHomework, continueCourse }) {
  const homeworkTodos = (pendingHomework || []).slice(0, 3).map((item, index) => ({
    id: `todo-homework-${item.id || index}`,
    type: "homework",
    title: item.title || "Pending Exercise",
    summary: [item.course, item.deadline ? `Due ${item.deadline}` : "", item.questionCount ? `${item.questionCount} questions` : ""].filter(Boolean).join(" · "),
    actionText: "Start Exercise",
    path: item.id ? `/pages/answer/index?id=${item.id}` : "/pages/tasks/index",
    priority: 100 - index,
    status: item.studentStatus || "Pending"
  }));
  const courseTodo = continueCourse && continueCourse.id ? [{
    id: `todo-study-${continueCourse.id}`,
    type: "schedule",
    title: "Continue Learning",
    summary: [continueCourse.name, continueCourse.grade, subjectLabel(continueCourse.subject)].filter(Boolean).join(" · "),
    actionText: "Continue Learning",
    path: `/pages/study-detail/index?id=${continueCourse.id}`,
    priority: 60,
    status: "In Progress"
  }] : [];
  return homeworkTodos.concat(courseTodo);
}

function decorateSubscription(reminder, enabled) {
  const item = reminder || {};
  const templateIds = Array.isArray(item.templateIds) ? item.templateIds.filter(Boolean) : [];
  return {
    ...item,
    templateIds,
    enabled,
    title: item.title || "Learning Reminders",
    summary: enabled ? "Reminders are enabled for classes, homework, and review results." : (item.summary || "Reminders are being set up. Check Messages for learning updates."),
    actionText: enabled ? "Enabled" : (item.actionText || (templateIds.length > 0 ? "Enable Reminders" : "View Messages"))
  };
}

function todoIcon(type) {
  const icons = {
    homework: "✍️",
    schedule: "📅",
    feedback: "💬",
    subscribe: "🔔"
  };
  return icons[type] || "📌";
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
    wx.showToast({ title: "Task details not found", icon: "none" });
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
    { label: "Exercises", action: "tasks", icon: "/assets/icons/shortcut-question.png" },
    { label: "Materials", action: "materials", icon: "/assets/icons/shortcut-material.png" },
    { label: "Schedule", action: "schedule", icon: "/assets/icons/shortcut-schedule.png" },
    { label: "Class Feedback", action: "feedback", icon: "/assets/icons/shortcut-open.png" }
  ];
}

function homeEmptyMessage(hasOpenedPackage) {
  if (hasOpenedPackage) {
    return "Your courses are active. Content will appear once your teacher publishes it.";
  }
  return "Contact your teacher to activate courses and start learning.";
}
