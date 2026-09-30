const { refreshNoticeBadge } = require("../../utils/notice-badge");
const { request } = require("../../utils/request");
const { subjectLabel } = require("../../utils/subject");
const { showPhoneAuthFailed, isCancel } = require("../../utils/phone-auth");

Page({
  data: {
    statusBarHeight: 0,
    loading: true,
    studentAccounts: [],
    switchingStudentId: "",
    addingStudent: false,
    studentAddOpen: false,
    gradeLabels: Array.from({ length: 12 }, (_, index) => `Grade ${index + 1}`),
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
    profileEditText: "Edit",
    emptyMessage: "Log in to sync learning records and teacher feedback.",
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
    guardianProfile: buildGuardianProfile({}),
    supportNotice: buildSupportNotice([], []),
    profileForm: {
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
      title: studentName ? `${studentName}  - Starline Learning Profile` : "Starline Learning Profile",
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
    refreshNoticeBadge();
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
        const message = error.message || "Log in to sync learning records and teacher feedback.";
        if (options.silent && this.data.me) {
          wx.showToast({ title: error.message || "Failed to update learning records", icon: "none" });
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
      refreshNoticeBadge();
      wx.showToast({ title: `Switched to ${result.user.name}`, icon: "success" });
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
      wx.showToast({ title: "Enter the name, grade, and school", icon: "none" });
      return;
    }
    this.setData({ addingStudent: true });
    request("/student/accounts", { method: "POST", data: { name, grade, schoolName } })
      .then(() => {
        this.setData({ studentAddOpen: false });
        wx.showToast({ title: "Student added. Trial access enabled.", icon: "success" });
        this.loadStudentAccounts();
      })
      .catch((error) => wx.showToast({ title: error.message || "Submission failed. Please try again.", icon: "none" }))
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
      wx.showToast({ title: "Avatar not received. Please try again.", icon: "none" });
      return;
    }
    if (this.data.savingProfile) {
      return;
    }
    this.setData({
      "guardianProfile.avatarUrl": avatarUrl
    });
    this.uploadAvatar(avatarUrl);
  },
  uploadAvatar(filePath) {
    const app = getApp();
    const baseUrl = app && app.globalData ? app.globalData.apiBaseUrl : "";
    if (!baseUrl || !wx.uploadFile) {
      wx.showToast({ title: "Avatar upload is unavailable", icon: "none" });
      return;
    }
    this.setData({ savingProfile: true });
    wx.uploadFile({
      url: `${baseUrl}/student/guardian-profile/avatar`,
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
          wx.showToast({ title: body.message || "Failed to save avatar. Please try again.", icon: "none" });
          return;
        }
        this.applyUpdatedGuardian(body.data, "Avatar updated");
      },
      fail: () => {
        this.restoreProfileAvatar();
        wx.showToast({ title: "Failed to upload avatar. Please try again.", icon: "none" });
      },
      complete: () => this.setData({ savingProfile: false })
    });
  },
  restoreProfileAvatar() {
    if (!this.data.me) {
      this.setData({ "guardianProfile.avatarUrl": "" });
      return;
    }
    this.setData({
      guardianProfile: buildGuardianProfile((this.data.home && this.data.home.guardian) || {})
    });
  },
  onNicknameInput(event) {
    const nickname = event.detail && event.detail.value ? event.detail.value : "";
    this.setData({
      "guardianProfile.nickname": nickname
    });
  },
  commitNickname() {
    const nickname = (this.data.guardianProfile.nickname || "").trim();
    if (!nickname) {
      wx.showToast({ title: "Nickname is required", icon: "none" });
      return;
    }
    if (nickname === (((this.data.home && this.data.home.guardian) || {}).nickname || "")) {
      return;
    }
    this.saveGuardianProfile({ nickname }, "Nickname updated");
  },
  saveGuardianProfile(changes = {}, toastTitle = "Profile updated") {
    if (this.data.savingProfile) {
      return;
    }
    const data = {};
    if (Object.prototype.hasOwnProperty.call(changes, "nickname")) {
      data.nickname = (changes.nickname || "").trim();
    }
    if (Object.prototype.hasOwnProperty.call(changes, "avatarUrl")) {
      data.avatarUrl = changes.avatarUrl || "";
    }
    if (Object.keys(data).length === 0) {
      return;
    }
    this.setData({ savingProfile: true });
    request("/student/guardian-profile", { method: "PUT", data })
      .then((guardian) => this.applyUpdatedGuardian(guardian, toastTitle))
      .catch((error) => {
        this.restoreProfileAvatar();
        wx.showToast({ title: error.message || "Failed to save", icon: "none" });
      })
      .then(() => this.setData({ savingProfile: false }));
  },
  authorizePhone(event) {
    if (this.data.savingProfile) {
      return;
    }
    const detail = event.detail || {};
    if (isCancel(detail)) {
      wx.showToast({ title: "Phone authorization cancelled", icon: "none" });
      return;
    }
    if (!detail.code) {
      showPhoneAuthFailed();
      return;
    }
    this.saveProfileChanges({ phoneCode: detail.code }, "Phone authorized");
  },
  saveProfileChanges(changes = {}, toastTitle = "Profile updated") {
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
      .catch((error) => wx.showToast({ title: error.message || "Failed to save", icon: "none" }))
      .then(() => this.setData({ savingProfile: false }));
  },
  onBasicInput(event) {
    const field = event.currentTarget.dataset.field;
    this.setData({ [`profileForm.${field}`]: event.detail.value });
  },
  toggleProfileEdit() {
    const profileEditing = !this.data.profileEditing;
    this.setData({ profileEditing, profileEditText: profileEditing ? "Close" : "Edit" });
  },
  stopModalTap() {},
  submitBasicProfile() {
    const form = this.data.profileForm;
    const studentName = (form.studentName || "").trim();
    const schoolName = (form.schoolName || "").trim();
    if (!studentName) {
      wx.showToast({ title: "Enter the student name", icon: "none" });
      return;
    }
    if (!schoolName) {
      wx.showToast({ title: "Enter the school name", icon: "none" });
      return;
    }
    this.setData({ savingBasicProfile: true });
    request("/student/profile", {
      method: "PUT",
      data: {
        studentName,
        schoolName,
        guardianName: (form.guardianName || "").trim()
      }
    })
      .then((student) => this.applyUpdatedStudent(student, "Profile saved"))
      .catch((error) => wx.showToast({ title: error.message || "Failed to save", icon: "none" }))
      .then(() => this.setData({ savingBasicProfile: false }));
  },
  applyUpdatedStudent(student, toastTitle) {
    const home = this.data.home ? { ...this.data.home, student } : { student };
    const state = buildPageState(home);
    this.setData({ ...state, profileEditing: false, profileEditText: "Edit" });
    wx.showToast({ title: toastTitle, icon: "success" });
  },
  applyUpdatedGuardian(guardian, toastTitle) {
    const home = { ...(this.data.home || {}), guardian };
    const state = buildPageState(home);
    this.setData({ ...state });
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
      wx.showToast({ title: "Feedback will appear after your teacher reviews your work.", icon: "none" });
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
    throw new Error("Learning account information missing");
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
    guardianProfile: buildGuardianProfile(home.guardian || {}),
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
  return match ? `${match[1]}-${match[2]}` : text;
}

function profileFormFromStudent(student) {
  return {
    studentName: student.name || "",
    grade: student.grade || "",
    schoolName: student.schoolName || "",
    guardianName: student.guardianName || ""
  };
}

function buildGuardianProfile(guardian = {}) {
  return {
    nickname: guardian.nickname || "",
    avatarUrl: normalizeAvatarUrl(guardian.avatarUrl),
    displayName: guardian.nickname || "WeChat User"
  };
}

function buildStudentProfile(student = {}) {
  const name = student.nickname || student.name || "Student";
  const grade = student.grade || "Grade not provided";
  const school = student.schoolName || "School not provided";
  const latest = student.lastStudyAt || student.lastSubmittedAt || "";
  const avatarUrl = normalizeAvatarUrl(student.avatarUrl);
  const phoneAuthorized = isAuthorizedPhone(student.phone, student.bindStatus);
  return {
    name: student.name || "Student",
    displayName: name,
    avatarUrl,
    avatarText: shortAvatarText(name),
    meta: `${grade} · ${school}`,
    status: latest ? `Last studied: ${latest}` : "Ready to start learning today",
    phoneAuthorized,
    phoneHint: phoneAuthorized ? maskPhone(student.phone) : "Authorize Phone"
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
  return text || "Authorize Phone";
}

function buildPrimaryTask(home, pendingTask, continueCourse) {
  if (pendingTask && pendingTask.id) {
    const meta = [pendingTask.course, pendingTask.questionNum ? `${pendingTask.questionNum} questions` : "", pendingTask.deadline ? `Due ${pendingTask.deadline}` : ""].filter(Boolean).join(" · ");
    return {
      action: "answer",
      tone: "urgent",
      label: "Pending",
      title: pendingTask.title || "You Have Pending Exercises",
      desc: meta || "Complete the exercise to view your score and feedback.",
      buttonText: "Start Exercise"
    };
  }
  if (continueCourse && continueCourse.id) {
    const progress = Number(home && home.continueProgress) || 0;
    return {
      action: "course",
      tone: "active",
      label: "Continue Learning",
      title: continueCourse.name || "Resume Learning",
      desc: [continueCourse.grade, subjectLabel(continueCourse.subject), progress > 0 ? `Completed ${progress}%` : ""].filter(Boolean).join(" · ") || "Continue from where you left off.",
      buttonText: "Continue Learning"
    };
  }
  const student = home && home.student ? home.student : {};
  if (Number(student.averageScore) > 0) {
    return {
      action: "scores",
      tone: "review",
      label: "Learning Feedback",
      title: "View Your Latest Feedback",
      desc: "Review your teacher's advice and keep practicing.",
      buttonText: "View Feedback"
    };
  }
  return {
    action: "study",
    tone: "quiet",
    label: "Learning Status",
      title: "You Will Be Notified of New Content",
      desc: "View your active courses in the Learning Center.",
      buttonText: "Start Learning"
  };
}

function buildOverviewMetrics(student = {}, home = {}, continueCourse = null, pendingHomework = []) {
  const chapterCount = Number(continueCourse && continueCourse.chapterCount) || 0;
  const progress = Math.max(0, Math.min(100, Number(home.continueProgress) || 0));
  const completedLessons = chapterCount > 0 ? Math.round(chapterCount * progress / 100) : 0;
  const courseCount = continueCourse && continueCourse.id ? 1 : 0;
  const pendingCount = Array.isArray(pendingHomework) ? pendingHomework.length : 0;
  return [
    { label: "Courses", value: `${courseCount}` },
    { label: "Lessons Completed", value: `${completedLessons}` },
    { label: "Tasks", value: `${pendingCount}`, emphasis: pendingCount > 0 }
  ];
}

function buildQuickActions() {
  return [
    { title: "My Schedule", action: "schedule", symbol: "▣", tone: "schedule" },
    { title: "Course Materials", action: "study", symbol: "▰", tone: "materials" },
    { title: "Class Feedback", action: "feedback", symbol: "▤", tone: "feedback" },
    { title: "Favorites", action: "favorites", symbol: "★", tone: "favorites" },
    { title: "Learning Reminders", action: "notices", symbol: "🔔", tone: "notice" },
    { title: "Account Settings", action: "profile", symbol: "⚙", tone: "settings" }
  ];
}

function buildProfileCompleteness(student = {}) {
  const missing = [];
  if (!student.name) missing.push("Name");
  if (!student.grade) missing.push("Grade");
  if (!student.schoolName) missing.push("School");
  const complete = missing.length === 0;
  return {
    complete,
    statusClass: complete ? "complete" : "pending",
    status: complete ? "Profile Complete" : "Profile Incomplete",
    summary: complete ? `${student.name} · ${student.grade} · ${student.schoolName}` : `Missing: ${missing.join(", ")}`,
    detail: complete ? "Your teacher uses these details to record scores and feedback." : "Complete your profile for more accurate score and feedback records."
  };
}

function buildSupportNotice(notices = [], pendingHomework = []) {
  const pendingCount = Array.isArray(pendingHomework) ? pendingHomework.length : 0;
  const noticeCount = Array.isArray(notices) ? notices.length : 0;
  if (pendingCount > 0) {
    return { action: "tasks", title: "Pending Exercises", desc: `${pendingCount} pending exercises`, actionText: "Start Exercise" };
  }
  if (noticeCount > 0) {
    return { action: "notices", title: "New Messages", desc: `${noticeCount} unread messages`, actionText: "View" };
  }
  return { action: "notices", title: "Messages and Feedback", desc: "New courses, feedback, and reminders appear here", actionText: "View" };
}

function shortAvatarText(name) {
  const text = String(name || "Me").trim();
  return text ? text.slice(0, 1) : "M";
}
