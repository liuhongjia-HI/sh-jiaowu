const { request } = require("../../utils/request");
const { showPhoneAuthFailed, isCancel } = require("../../utils/phone-auth");

Page({
  data: {
    statusBarHeight: 0,
    loading: true,
    studentAccounts: [],
    switchingStudentId: "",
    addingStudent: false,
    studentAddOpen: false,
    gradeOptions: ["一年级", "二年级", "三年级", "四年级", "五年级", "六年级", "七年级", "八年级", "九年级", "十年级", "十一年级", "十二年级"],
    studentAddGradeIndex: -1,
    studentAddForm: {
      name: "",
      grade: "",
      schoolName: ""
    },
    savingProfile: false,
    savingBasicProfile: false,
    profileEditing: false,
    profileEditText: "编辑",
    emptyMessage: "登录后可同步学习记录和老师反馈。",
    me: null,
    home: null,
    continueCourse: null,
    recentLearning: null,
    pendingTask: null,
    studentProfile: buildStudentProfile({}),
    primaryTask: buildPrimaryTask(null, null, null),
    overviewMetrics: buildOverviewMetrics({}),
    quickActions: buildQuickActions({}, []),
    profileCompleteness: buildProfileCompleteness({}),
    supportNotice: buildSupportNotice([], []),
    profileForm: {
      nickname: "",
      avatarUrl: "",
      studentName: "",
      grade: "",
      schoolName: "",
      guardianName: ""
    }
  },
  onLoad() {
    this.syncStatusBarHeight();
    this.loadMe();
  },
  onShareAppMessage() {
    const studentName = this.data.studentProfile && this.data.studentProfile.name;
    return {
      title: studentName ? `${studentName} 的 Starline 学习主页` : "Starline 学习主页",
      path: "/pages/home/index"
    };
  },
  syncStatusBarHeight() {
    try {
      const systemInfo = wx.getWindowInfo ? wx.getWindowInfo() : (wx.getSystemInfoSync ? wx.getSystemInfoSync() : null);
      const statusBarHeight = systemInfo && Number(systemInfo.statusBarHeight);
      if (statusBarHeight > 0) {
        this.setData({ statusBarHeight });
      }
    } catch (error) {
      // 部分旧版基础库没有窗口信息 API，保留 0 让页面按默认安全区渲染。
    }
  },
  onShow() {
    if (!this.data.loading) {
      this.loadMe({ silent: !!this.data.me });
    }
  },
  loadMe(options = {}) {
    if (!options.silent) {
      this.setData({ loading: true });
    }
    request("/student/home")
      .then((home) => {
        const state = buildPageState(home);
        this.setData({ ...state, loading: false });
        this.loadStudentAccounts();
      })
      .catch((error) => {
        const message = error.message || "登录后可同步学习记录和老师反馈。";
        if (options.silent && this.data.me) {
          wx.showToast({ title: error.message || "学习记录更新失败", icon: "none" });
          return;
        }
        this.setData({
          me: null,
          emptyMessage: message,
          loading: false
        });
      });
  },
  loadStudentAccounts() {
    request("/student/accounts", { silent: true }).then((accounts) => {
      this.setData({ studentAccounts: Array.isArray(accounts) ? accounts : [] });
    }).catch(() => this.setData({ studentAccounts: [] }));
  },
  switchStudent(event) {
    const studentId = event.currentTarget.dataset.studentId;
    const account = (this.data.studentAccounts || []).find((item) => item.studentId === studentId);
    if (!studentId || !account || !account.canSwitch || account.active || this.data.switchingStudentId) return;
    this.setData({ switchingStudentId: studentId });
    request(`/student/accounts/${studentId}/switch`, { method: "POST", data: {} }).then((result) => {
      wx.setStorageSync("starline_token", result.token);
      wx.setStorageSync("starline_student_id", studentId);
      wx.showToast({ title: `已切换到${result.user.name}`, icon: "success" });
      this.setData({ switchingStudentId: "" });
      this.loadMe();
    }).catch(() => this.setData({ switchingStudentId: "" }));
  },
  openStudentAddModal() {
    this.setData({
      studentAddOpen: true,
      studentAddGradeIndex: -1,
      studentAddForm: { name: "", grade: "", schoolName: "" }
    });
  },
  closeStudentAddModal() {
    if (!this.data.addingStudent) {
      this.setData({ studentAddOpen: false });
    }
  },
  onStudentAddInput(event) {
    const field = event.currentTarget.dataset.field;
    this.setData({ [`studentAddForm.${field}`]: event.detail.value });
  },
  onStudentAddGradeChange(event) {
    const index = Number(event.detail.value);
    this.setData({
      studentAddGradeIndex: index,
      "studentAddForm.grade": this.data.gradeOptions[index] || ""
    });
  },
  submitStudentAdd() {
    if (this.data.addingStudent) return;
    const form = this.data.studentAddForm || {};
    const name = (form.name || "").trim();
    const grade = (form.grade || "").trim();
    const schoolName = (form.schoolName || "").trim();
    if (!name || !grade || !schoolName) {
      wx.showToast({ title: "请填写姓名、年级和学校", icon: "none" });
      return;
    }
    this.setData({ addingStudent: true });
    request("/student/accounts", { method: "POST", data: { name, grade, schoolName } })
      .then(() => {
        this.setData({ studentAddOpen: false });
        wx.showToast({ title: "添加成功，已开通体验", icon: "success" });
        this.loadStudentAccounts();
      })
      .catch((error) => wx.showToast({ title: error.message || "提交失败，请重试", icon: "none" }))
      .then(() => this.setData({ addingStudent: false }));
  },
  goLogin() {
    wx.navigateTo({ url: "/pages/login/index" });
  },
  handlePrimaryAction() {
    this.navigateByAction(this.data.primaryTask.action);
  },
  handleQuickAction(event) {
    this.navigateByAction(event.currentTarget.dataset.action);
  },
  handleSupportTap() {
    this.navigateByAction(this.data.supportNotice.action);
  },
  navigateByAction(action) {
    if (action === "answer") {
      this.goAnswer();
      return;
    }
    if (action === "course") {
      this.goStudyDetail();
      return;
    }
    if (action === "scores") {
      this.goScores();
      return;
    }
    if (action === "tasks") {
      this.goTasks();
      return;
    }
    if (action === "growth") {
      this.goGrowth();
      return;
    }
    if (action === "schedule") {
      this.goSchedule();
      return;
    }
    if (action === "favorites") {
      this.goFavorites();
      return;
    }
    if (action === "notices") {
      this.goNotices();
      return;
    }
    if (action === "feedback") {
      this.goLatestFeedback();
      return;
    }
    if (action === "profile") {
      this.toggleProfileEdit();
      return;
    }
    this.goStudy();
  },
  onChooseAvatar(event) {
    const avatarUrl = event.detail && event.detail.avatarUrl;
    if (!avatarUrl) {
      wx.showToast({ title: "没有获取到头像，请重试", icon: "none" });
      return;
    }
    if (this.data.savingProfile) {
      return;
    }
    this.setData({
      "profileForm.avatarUrl": avatarUrl,
      "studentProfile.avatarUrl": avatarUrl
    });
    this.uploadAvatar(avatarUrl);
  },
  uploadAvatar(filePath) {
    const app = getApp();
    const baseUrl = app && app.globalData ? app.globalData.apiBaseUrl : "";
    if (!baseUrl || !wx.uploadFile) {
      wx.showToast({ title: "当前环境不支持头像上传", icon: "none" });
      return;
    }
    this.setData({ savingProfile: true });
    wx.uploadFile({
      url: `${baseUrl}/student/profile/avatar`,
      filePath,
      name: "file",
      header: {
        Authorization: wx.getStorageSync("starline_token") ? `Bearer ${wx.getStorageSync("starline_token")}` : ""
      },
      success: (response) => {
        let body = {};
        try {
          body = JSON.parse(response.data || "{}");
        } catch (error) {
          body = {};
        }
        if (response.statusCode !== 200 || body.code !== 0 || !body.data) {
          this.restoreProfileAvatar();
          wx.showToast({ title: body.message || "头像保存失败，请重试", icon: "none" });
          return;
        }
        this.applyUpdatedStudent(body.data, "头像已更新");
      },
      fail: () => {
        this.restoreProfileAvatar();
        wx.showToast({ title: "头像上传失败，请重试", icon: "none" });
      },
      complete: () => this.setData({ savingProfile: false })
    });
  },
  restoreProfileAvatar() {
    if (!this.data.me) {
      this.setData({ "studentProfile.avatarUrl": "" });
      return;
    }
    this.setData({
      studentProfile: buildStudentProfile(this.data.me),
      "profileForm.avatarUrl": this.data.me.avatarUrl || ""
    });
  },
  onNicknameInput(event) {
    const nickname = event.detail && event.detail.value ? event.detail.value : "";
    this.setData({
      "profileForm.nickname": nickname,
      "studentProfile.displayName": nickname || "微信用户"
    });
  },
  commitNickname() {
    const nickname = (this.data.profileForm.nickname || "").trim();
    if (!nickname) {
      wx.showToast({ title: "昵称不能为空", icon: "none" });
      return;
    }
    if (nickname === ((this.data.me && this.data.me.nickname) || "")) {
      return;
    }
    this.saveProfileChanges({ nickname }, "昵称已更新");
  },
  authorizePhone(event) {
    if (this.data.savingProfile) {
      return;
    }
    const detail = event.detail || {};
    if (isCancel(detail)) {
      wx.showToast({ title: "已取消手机号授权", icon: "none" });
      return;
    }
    if (!detail.code) {
      showPhoneAuthFailed();
      return;
    }
    this.saveProfileChanges({ phoneCode: detail.code }, "手机号已授权");
  },
  saveProfileChanges(changes = {}, toastTitle = "资料已更新") {
    if (this.data.savingProfile) {
      return;
    }
    const form = { ...this.data.profileForm, ...changes };
    const data = {};
    if (Object.prototype.hasOwnProperty.call(changes, "nickname")) {
      data.nickname = (form.nickname || "").trim();
    }
    if (Object.prototype.hasOwnProperty.call(changes, "avatarUrl")) {
      data.avatarUrl = form.avatarUrl || "";
    }
    if (Object.prototype.hasOwnProperty.call(changes, "phoneCode")) {
      data.phoneCode = form.phoneCode || "";
    }
    ["studentName", "grade", "schoolName", "guardianName"].forEach((field) => {
      if (Object.prototype.hasOwnProperty.call(changes, field)) {
        data[field] = (form[field] || "").trim();
      }
    });
    if (Object.keys(data).length === 0) {
      return;
    }
    this.setData({ savingProfile: true });
    request("/student/profile", { method: "PUT", data })
      .then((student) => this.applyUpdatedStudent(student, toastTitle))
      .catch((error) => wx.showToast({ title: error.message || "保存失败", icon: "none" }))
      .then(() => this.setData({ savingProfile: false }));
  },
  onBasicInput(event) {
    const field = event.currentTarget.dataset.field;
    this.setData({ [`profileForm.${field}`]: event.detail.value });
  },
  toggleProfileEdit() {
    const profileEditing = !this.data.profileEditing;
    this.setData({ profileEditing, profileEditText: profileEditing ? "收起" : "编辑" });
  },
  stopModalTap() {},
  submitBasicProfile() {
    const form = this.data.profileForm;
    const studentName = (form.studentName || "").trim();
    const schoolName = (form.schoolName || "").trim();
    if (!studentName) {
      wx.showToast({ title: "请输入学生姓名", icon: "none" });
      return;
    }
    if (!schoolName) {
      wx.showToast({ title: "请输入学校", icon: "none" });
      return;
    }
    this.setData({ savingBasicProfile: true });
    request("/student/profile", {
      method: "PUT",
      data: {
        nickname: form.nickname || "",
        avatarUrl: form.avatarUrl || "",
        studentName,
        schoolName,
        guardianName: (form.guardianName || "").trim()
      }
    })
      .then((student) => this.applyUpdatedStudent(student, "资料已保存"))
      .catch((error) => wx.showToast({ title: error.message || "保存失败", icon: "none" }))
      .then(() => this.setData({ savingBasicProfile: false }));
  },
  applyUpdatedStudent(student, toastTitle) {
    const home = this.data.home ? { ...this.data.home, student } : { student };
    const state = buildPageState(home);
    this.setData({ ...state, profileEditing: false, profileEditText: "编辑" });
    wx.showToast({ title: toastTitle, icon: "success" });
  },
  goStudyDetail() {
    if (!this.data.continueCourse || !this.data.continueCourse.id) {
      this.goStudy();
      return;
    }
    wx.navigateTo({ url: `/pages/study-detail/index?id=${this.data.continueCourse.id}` });
  },
  goLatestFeedback() {
    const feedback = (this.data.home && this.data.home.classroomFeedback || [])[0];
    if (!feedback || !feedback.relatedSubmissionId) {
      wx.showToast({ title: "老师批改后会显示反馈", icon: "none" });
      return;
    }
    wx.navigateTo({ url: `/pages/result/index?id=${feedback.relatedSubmissionId}` });
  },
  goAnswer() {
    if (!this.data.pendingTask || !this.data.pendingTask.id) {
      this.goTasks();
      return;
    }
    wx.navigateTo({ url: `/pages/answer/index?id=${this.data.pendingTask.id}` });
  },
  goStudy() {
    wx.switchTab({ url: "/pages/study/index" });
  },
  goTasks() {
    wx.navigateTo({ url: "/pages/tasks/index" });
  },
  goNotices() {
    wx.switchTab({ url: "/pages/notices/index" });
  },
  goSchedule() {
    wx.navigateTo({ url: "/pages/schedule/index" });
  },
  goGrowth() {
    wx.navigateTo({ url: "/pages/growth/index" });
  },
  goScores() {
    wx.navigateTo({ url: "/pages/scores/index" });
  },
  goFavorites() {
    wx.navigateTo({ url: "/pages/favorites/index" });
  }
});

function buildPageState(home = {}) {
  const student = home.student;
  if (!student) {
    throw new Error("学习账号信息缺失");
  }
  const pendingHomework = Array.isArray(home.pendingHomework) ? home.pendingHomework : [];
  const notices = Array.isArray(home.notices) ? home.notices : [];
  const continueCourse = home.continueCourse || null;
  const pendingTask = pendingHomework[0] || null;
  return {
    me: student,
    home,
    continueCourse,
    pendingTask,
    recentLearning: buildRecentLearning(home, continueCourse),
    studentProfile: buildStudentProfile(student),
    primaryTask: buildPrimaryTask(home, pendingTask, continueCourse),
    overviewMetrics: buildOverviewMetrics(student, home, continueCourse, pendingHomework),
    quickActions: buildQuickActions(),
    profileCompleteness: buildProfileCompleteness(student),
    supportNotice: buildSupportNotice(notices, pendingHomework),
    profileForm: profileFormFromStudent(student)
  };
}

function buildRecentLearning(home, continueCourse) {
  if (!continueCourse || !continueCourse.id) {
    return null;
  }
  const progress = Math.max(0, Math.min(100, Number(home.continueProgress) || 0));
  const chapterCount = Number(continueCourse.chapterCount) || 0;
  return {
    ...continueCourse,
    progress,
    chapterCount,
    completedLessons: chapterCount > 0 ? Math.round(chapterCount * progress / 100) : 0,
    lastStudyAt: formatRecentDate((home.student && (home.student.lastStudyAt || home.student.lastSubmittedAt)) || "")
  };
}

function formatRecentDate(value) {
  const text = String(value || "");
  const match = text.match(/^\d{4}-(\d{1,2})-(\d{1,2})/);
  return match ? `${Number(match[1])}月${Number(match[2])}日` : text;
}

function profileFormFromStudent(student) {
  return {
    nickname: student.nickname || "",
    avatarUrl: student.avatarUrl || "",
    studentName: student.name || "",
    grade: student.grade || "",
    schoolName: student.schoolName || "",
    guardianName: student.guardianName || ""
  };
}

function buildStudentProfile(student = {}) {
  const name = student.nickname || student.name || "学员";
  const grade = student.grade || "年级待补全";
  const school = student.schoolName || "学校待补全";
  const latest = student.lastStudyAt || student.lastSubmittedAt || "";
  const avatarUrl = normalizeAvatarUrl(student.avatarUrl);
  const phoneAuthorized = isAuthorizedPhone(student.phone, student.bindStatus);
  return {
    name: student.name || "学员",
    displayName: name,
    avatarUrl,
    avatarText: shortAvatarText(name),
    meta: `${grade} · ${school}`,
    status: latest ? `最近学习 ${latest}` : "准备开始今天的学习",
    phoneAuthorized,
    phoneHint: phoneAuthorized ? maskPhone(student.phone) : "授权手机号"
  };
}

function normalizeAvatarUrl(value) {
  const text = String(value || "").trim();
  if (!text || /^https?:\/\//i.test(text) || text.indexOf("wxfile://") === 0) {
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

function isAuthorizedPhone(value, bindStatus) {
  const text = String(value || "").trim();
  return !!text && (bindStatus === "已绑定" || !text.includes("*"));
}

function maskPhone(value) {
  const text = String(value || "").trim();
  if (text.length === 11 && !text.includes("*")) {
    return `${text.slice(0, 3)}****${text.slice(-4)}`;
  }
  return text || "授权手机号";
}

function buildPrimaryTask(home, pendingTask, continueCourse) {
  if (pendingTask && pendingTask.id) {
    const meta = [pendingTask.course, pendingTask.questionNum ? `${pendingTask.questionNum} 道题` : "", pendingTask.deadline ? `截止 ${pendingTask.deadline}` : ""].filter(Boolean).join(" · ");
    return {
      action: "answer",
      tone: "urgent",
      label: "待完成",
      title: pendingTask.title || "有练习待完成",
      desc: meta || "完成后查看得分和反馈。",
      buttonText: "开始练习"
    };
  }
  if (continueCourse && continueCourse.id) {
    const progress = Number(home && home.continueProgress) || 0;
    return {
      action: "course",
      tone: "active",
      label: "继续学习",
      title: continueCourse.name || "继续上次学习",
      desc: [continueCourse.grade, continueCourse.subject, progress > 0 ? `已学 ${progress}%` : ""].filter(Boolean).join(" · ") || "从上次进度继续学习。",
      buttonText: "继续学习"
    };
  }
  const student = home && home.student ? home.student : {};
  if (Number(student.averageScore) > 0) {
    return {
      action: "scores",
      tone: "review",
      label: "学习反馈",
      title: "查看最近成绩反馈",
      desc: "查看老师建议，继续练习。",
      buttonText: "查看反馈"
    };
  }
  return {
    action: "study",
    tone: "quiet",
    label: "学习状态",
      title: "有新内容时会提醒你",
      desc: "去学习中心查看已开通课程。",
      buttonText: "去学习"
  };
}

function buildOverviewMetrics(student = {}, home = {}, continueCourse = null, pendingHomework = []) {
  const chapterCount = Number(continueCourse && continueCourse.chapterCount) || 0;
  const progress = Math.max(0, Math.min(100, Number(home.continueProgress) || 0));
  const completedLessons = chapterCount > 0 ? Math.round(chapterCount * progress / 100) : 0;
  const courseCount = continueCourse && continueCourse.id ? 1 : 0;
  const pendingCount = Array.isArray(pendingHomework) ? pendingHomework.length : 0;
  return [
    { label: "学习课程", value: `${courseCount}` },
    { label: "完成课时", value: `${completedLessons}` },
    { label: "待办", value: `${pendingCount}`, emphasis: pendingCount > 0 }
  ];
}

function buildQuickActions() {
  return [
    { title: "我的课表", action: "schedule", symbol: "▣", tone: "schedule" },
    { title: "课程讲义", action: "study", symbol: "▰", tone: "materials" },
    { title: "课堂反馈", action: "feedback", symbol: "▤", tone: "feedback" },
    { title: "收藏课程", action: "favorites", symbol: "★", tone: "favorites" },
    { title: "学习提醒", action: "notices", symbol: "🔔", tone: "notice" },
    { title: "账号设置", action: "profile", symbol: "⚙", tone: "settings" }
  ];
}

function buildProfileCompleteness(student = {}) {
  const missing = [];
  if (!student.name) missing.push("姓名");
  if (!student.grade) missing.push("年级");
  if (!student.schoolName) missing.push("学校");
  const complete = missing.length === 0;
  return {
    complete,
    statusClass: complete ? "complete" : "pending",
    status: complete ? "资料完整" : "资料待补全",
    summary: complete ? `${student.name} · ${student.grade} · ${student.schoolName}` : `待补全：${missing.join("、")}`,
    detail: complete ? "老师会用这些信息记录成绩和反馈。" : "补全后，成绩和反馈记录会更准确。"
  };
}

function buildSupportNotice(notices = [], pendingHomework = []) {
  const pendingCount = Array.isArray(pendingHomework) ? pendingHomework.length : 0;
  const noticeCount = Array.isArray(notices) ? notices.length : 0;
  if (pendingCount > 0) {
    return { action: "tasks", title: "还有练习待完成", desc: `${pendingCount} 个练习待完成`, actionText: "开始练习" };
  }
  if (noticeCount > 0) {
    return { action: "notices", title: "有新的通知", desc: `${noticeCount} 条通知需要查看`, actionText: "查看" };
  }
  return { action: "notices", title: "通知与反馈", desc: "新的课程、反馈和提醒会同步到这里", actionText: "查看" };
}

function shortAvatarText(name) {
  const text = String(name || "我").trim();
  return text ? text.slice(0, 1) : "我";
}
