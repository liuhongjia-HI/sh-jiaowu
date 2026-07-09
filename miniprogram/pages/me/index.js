const { request } = require("../../utils/request");

Page({
  data: {
    loading: true,
    savingProfile: false,
    savingBasicProfile: false,
    profileEditing: false,
    profileEditText: "编辑",
    emptyMessage: "登录后可以同步学习记录、小挑战结果和老师反馈。",
    me: null,
    home: null,
    continueCourse: null,
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
    this.loadMe();
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
      })
      .catch((error) => {
        const message = error.message || "登录后可以同步学习记录、小挑战结果和老师反馈。";
        if (options.silent && this.data.me) {
          wx.showToast({ title: error.message || "学习记录刷新失败", icon: "none" });
          return;
        }
        this.setData({
          me: null,
          emptyMessage: message,
          loading: false
        });
      });
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
    this.goStudy();
  },
  authorizeProfile() {
    if (this.data.savingProfile) {
      return;
    }
    if (!wx.getUserProfile) {
      wx.showToast({ title: "当前微信版本不支持资料授权", icon: "none" });
      return;
    }
    wx.getUserProfile({
      desc: "用于完善学生头像和昵称",
      success: (res) => {
        const userInfo = res.userInfo || {};
        this.submitProfile((userInfo.nickName || "").trim(), userInfo.avatarUrl || "");
      },
      fail: () => wx.showToast({ title: "已取消头像昵称授权", icon: "none" })
    });
  },
  submitProfile(nickname, avatarUrl) {
    if (!avatarUrl) {
      wx.showToast({ title: "请授权微信头像", icon: "none" });
      return;
    }
    if (!nickname) {
      wx.showToast({ title: "请授权微信昵称", icon: "none" });
      return;
    }
    this.setData({ savingProfile: true });
    const form = this.data.profileForm;
    request("/student/profile", {
      method: "PUT",
      data: {
        nickname,
        avatarUrl,
        studentName: form.studentName || this.data.me.name || "",
        grade: form.grade || this.data.me.grade || "",
        schoolName: form.schoolName || this.data.me.schoolName || "",
        guardianName: form.guardianName || ""
      }
    })
      .then((student) => this.applyUpdatedStudent(student, "已更新头像"))
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
  submitBasicProfile() {
    const form = this.data.profileForm;
    const studentName = (form.studentName || "").trim();
    const grade = (form.grade || "").trim();
    const schoolName = (form.schoolName || "").trim();
    if (!studentName) {
      wx.showToast({ title: "请输入学生姓名", icon: "none" });
      return;
    }
    if (!grade) {
      wx.showToast({ title: "请输入年级", icon: "none" });
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
        grade,
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
    studentProfile: buildStudentProfile(student),
    primaryTask: buildPrimaryTask(home, pendingTask, continueCourse),
    overviewMetrics: buildOverviewMetrics(student),
    quickActions: buildQuickActions(student, pendingHomework),
    profileCompleteness: buildProfileCompleteness(student),
    supportNotice: buildSupportNotice(notices, pendingHomework),
    profileForm: profileFormFromStudent(student)
  };
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
  const name = student.nickname || student.name || "同学";
  const grade = student.grade || "年级待补全";
  const school = student.schoolName || "学校待补全";
  const latest = student.lastStudyAt || student.lastSubmittedAt || "";
  return {
    displayName: `${name} 同学`,
    avatarUrl: student.avatarUrl || "",
    avatarText: shortAvatarText(name),
    meta: `${grade} · ${school}`,
    status: latest ? `最近学习 ${latest}` : "准备开始今天的学习",
    profileHint: student.nickname && student.avatarUrl ? "资料已同步" : "可补充头像昵称"
  };
}

function buildPrimaryTask(home, pendingTask, continueCourse) {
  if (pendingTask && pendingTask.id) {
    const meta = [pendingTask.course, pendingTask.questionNum ? `${pendingTask.questionNum} 道题` : "", pendingTask.deadline ? `截止 ${pendingTask.deadline}` : ""].filter(Boolean).join(" · ");
    return {
      action: "answer",
      tone: "urgent",
      label: "待完成",
      title: pendingTask.title || "有小挑战待完成",
      desc: meta || "完成后可以查看得分和老师反馈。",
      buttonText: "去完成"
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
      desc: "看看老师建议，再安排下一次练习。",
      buttonText: "查看反馈"
    };
  }
  return {
    action: "study",
    tone: "quiet",
    label: "学习状态",
    title: "老师发布内容后会提醒你",
    desc: "你可以先进入学习中心查看已开通内容。",
    buttonText: "去学习中心"
  };
}

function buildOverviewMetrics(student = {}) {
  const averageScore = Number(student.averageScore) || 0;
  const streakDays = Number(student.streakDays) || 0;
  const badgeCount = Number(student.badgeCount) || 0;
  return [
    { label: "平均分", value: averageScore > 0 ? `${averageScore}` : "--", hint: averageScore > 0 ? "近期练习" : "完成后生成" },
    { label: "连续学习", value: streakDays > 0 ? `${streakDays}天` : "今天开始", hint: streakDays > 0 ? "保持节奏" : "完成一次点亮" },
    { label: "徽章", value: badgeCount > 0 ? `${badgeCount}枚` : "待点亮", hint: badgeCount > 0 ? "已获得" : "挑战后获得" }
  ];
}

function buildQuickActions(student = {}, pendingHomework = []) {
  const pendingCount = Array.isArray(pendingHomework) ? pendingHomework.length : 0;
  const hasScore = Number(student.averageScore) > 0;
  return [
    { title: "学习中心", desc: "课程与资料", action: "study", symbol: "学", badge: "" },
    { title: "小挑战", desc: "练习任务", action: "tasks", symbol: "练", badge: pendingCount > 0 ? "待完成" : "" },
    { title: "成绩反馈", desc: "分数与建议", action: "scores", symbol: "绩", badge: hasScore ? "可查看" : "" },
    { title: "学习记录", desc: "进度记录", action: "growth", symbol: "记", badge: "" },
    { title: "我的课表", desc: "上课安排", action: "schedule", symbol: "课", badge: "" },
    { title: "收藏", desc: "资料重点", action: "favorites", symbol: "藏", badge: "" }
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
    detail: complete ? "老师记录成绩和反馈时会使用这些信息。" : "补全后，老师录入成绩和反馈会更准确。"
  };
}

function buildSupportNotice(notices = [], pendingHomework = []) {
  const pendingCount = Array.isArray(pendingHomework) ? pendingHomework.length : 0;
  const noticeCount = Array.isArray(notices) ? notices.length : 0;
  if (pendingCount > 0) {
    return { action: "tasks", title: "还有小挑战待完成", desc: `${pendingCount} 个任务等待处理`, actionText: "去处理" };
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
