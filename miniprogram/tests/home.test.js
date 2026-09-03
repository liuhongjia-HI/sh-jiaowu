const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const SUBSCRIBE_TEMPLATE_ID = "vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0";

function loadHomePage(requestImpl, wxMock = {}, appMock = {}) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const pagePath = require.resolve("../pages/home/index.js");
  delete require.cache[requestPath];
  delete require.cache[pagePath];
  require.cache[requestPath] = {
    id: requestPath,
    filename: requestPath,
    loaded: true,
    exports: { request: requestImpl }
  };
  global.wx = wxMock;
  global.getApp = () => appMock;
  global.Page = (definition) => pages.push(definition);
  require(pagePath);
  const definition = pages[0];
  const page = {
    data: JSON.parse(JSON.stringify(definition.data)),
    setData(patch, callback) {
      Object.assign(this.data, patch);
      callback && callback();
    }
  };
  Object.keys(definition).forEach((key) => {
    if (key !== "data") {
      page[key] = typeof definition[key] === "function" ? definition[key].bind(page) : definition[key];
    }
  });
  return page;
}

function flushPromises() {
  return new Promise((resolve) => setImmediate(resolve));
}

test("home course swiper clips neighboring course cards", () => {
  const stylesheet = fs.readFileSync(path.join(__dirname, "../pages/home/index.wxss"), "utf8");
  const swiperItemRule = stylesheet.match(/\.home-course-swiper swiper-item\s*\{([^}]*)\}/);

  assert(swiperItemRule, "expected a dedicated course swiper-item style rule");
  assert.match(swiperItemRule[1], /overflow:\s*hidden\s*;/);
});

test("home page greeting follows the current local hour", () => {
  const page = loadHomePage(() => Promise.resolve({}));

  page.refreshGreeting(new Date(2026, 7, 23, 8, 52));
  assert.equal(page.data.greeting, "早上好");
  page.refreshGreeting(new Date(2026, 7, 23, 12, 0));
  assert.equal(page.data.greeting, "中午好");
  page.refreshGreeting(new Date(2026, 7, 23, 15, 0));
  assert.equal(page.data.greeting, "下午好");
  page.refreshGreeting(new Date(2026, 7, 23, 20, 0));
  assert.equal(page.data.greeting, "晚上好");
});

test("home page opens directly for a visitor without showing a welcome gate", () => {
  const calls = [];
  const page = loadHomePage((path) => {
    calls.push(path);
    return Promise.resolve({});
  }, {
    getStorageSync() {
      return "";
    }
  });

  page.onLoad();

  assert.equal(page.data.visitorMode, true);
  assert.deepEqual(page.data.home, {});
  assert.equal(page.data.loading, false);
  assert.deepEqual(calls, []);
});

test("home page greeting uses the student's nickname, name, then a friendly fallback", async () => {
  const students = [
    { nickname: "小星", name: "王同学" },
    { nickname: "微信用户", name: "李同学" },
    {}
  ];
  let index = 0;
  const page = loadHomePage((path) => Promise.resolve(path === "/student/home" ? { student: students[index++] || {} } : []), {
    getStorageSync() {
      return "";
    }
  });

  page.loadHome();
  await flushPromises();
  assert.equal(page.data.greetingName, "小星");

  page.loadHome();
  await flushPromises();
  assert.equal(page.data.greetingName, "李同学");

  page.loadHome();
  await flushPromises();
  assert.equal(page.data.greetingName, "同学");
});

test("home page renders today todos and classroom feedback from student home", async () => {
  const page = loadHomePage(() => Promise.resolve({
    student: { id: "stu-001", grade: "五年级", openedPackages: ["英语套餐"] },
    continueCourse: { id: "course-001", name: "五年级英语S1Q1课程", grade: "五年级", subject: "英语", chapterCount: 8 },
    continueProgress: 50,
    pendingHomework: [],
    materials: [],
    notices: [],
    todayTodos: [
      { id: "todo-homework-1", type: "homework", title: "英语阅读挑战", summary: "截止 2026-12-31", actionText: "去完成", path: "/pages/answer/index?id=hw-1", status: "待完成" },
      { id: "todo-subscribe", type: "subscribe", title: "开启学习提醒", summary: "接收提醒", actionText: "开启提醒", status: "建议开启" }
    ],
    classroomFeedback: [
      {
        id: "feedback-sub-1",
        courseName: "五年级英语S1Q1课程",
        lessonTitle: "英语阅读挑战",
        teacherName: "英语老师",
        performance: "本次掌握扎实，表达和准确率表现稳定。",
        focus: "关键词找得准确。",
        nextStep: "保持当前节奏。",
        score: 95,
        relatedSubmissionId: "sub-1"
      }
    ],
    subscriptionReminder: { title: "学习提醒", summary: "开启后接收提醒", actionText: "开启提醒", templateIds: [SUBSCRIBE_TEMPLATE_ID] }
  }), {
    getStorageSync() {
      return "";
    }
  });

  page.loadHome();
  await flushPromises();

  assert.equal(page.data.loading, false);
  assert.equal(page.data.todoItems.length, 2);
  assert.equal(page.data.todoItems[0].icon, "练");
  assert.equal(page.data.todoItems[1].icon, "醒");
  assert.equal(page.data.feedbackItems.length, 1);
  assert.equal(page.data.feedbackItems[0].score, 95);
  assert.equal(page.data.subscriptionReminder.actionText, "开启提醒");
});

test("home page exposes every course for swipeable banner cards", async () => {
  const calls = [];
  const page = loadHomePage((path) => Promise.resolve(path === "/student/home" ? {
    student: { id: "stu-001", openedPackages: ["英语套餐"] },
    courses: [
      { id: "course-001", name: "五年级英语", grade: "五年级", subject: "英语", lessonCount: 8, progress: 25 },
      { id: "course-002", name: "五年级地理", grade: "五年级", subject: "地理", lessonCount: 6, progress: 60 }
    ],
    continueCourse: { id: "course-001", name: "五年级英语" },
    continueProgress: 25,
    pendingHomework: [], materials: [], notices: []
  } : []), {
    getStorageSync() { return ""; },
    navigateTo(args) { calls.push(args.url); }
  });

  page.loadHome();
  await flushPromises();

  assert.equal(page.data.courses.length, 2);
  assert.equal(page.data.courseSlides.length, 2);
  assert.equal(page.data.courseSlides[1].progress, 60);
  page.changeCourse({ detail: { current: 1 } });
  assert.equal(page.data.continueCourse.id, "course-002");
  assert.equal(page.data.progressPercent, 60);
  page.goStudyDetail({ currentTarget: { dataset: { courseId: "course-002" } } });
  assert.equal(calls[0], "/pages/study-detail/index?id=course-002");
});

test("home page shortcut labels use commercial learning actions", () => {
  const page = loadHomePage(() => Promise.resolve({}), {
    getStorageSync() {
      return "";
    }
  });
  const labels = page.data.shortcuts.map((item) => item.label);

  assert(labels.includes("课表"));
  assert(labels.includes("课堂反馈"));
  assert.equal(labels.includes("1V1/课表"), false);
  assert.equal(labels.includes("快捷开通"), false);
});

test("home summary cards expose direct actions for todos, materials, and notices", () => {
  const template = fs.readFileSync(path.join(__dirname, "../pages/home/index.wxml"), "utf8");

  assert.match(template, /class="status-item"\s+data-action="tasks"\s+bindtap="handleShortcut"/);
  assert.match(template, /class="status-item"\s+data-action="materials"\s+bindtap="handleShortcut"/);
  assert.match(template, /class="status-item"\s+data-action="notices"\s+bindtap="handleShortcut"/);
});

test("home page displays unopened package recommendations", async () => {
  const page = loadHomePage((path) => {
    if (path === "/student/recommendations") {
      return Promise.resolve([{
        packageId: "pkg-english-reading",
        packageName: "五年级英语阅读提升",
        grade: "五年级",
        semester: "S1",
        subject: "英语",
        courseCount: 2,
        materialCount: 3,
        contentSamples: ["阅读课程", "阅读讲义"],
        recommendationReason: "同学习空间推荐",
        summary: "提升阅读理解能力"
      }]);
    }
    return Promise.resolve({
      student: { id: "stu-001", grade: "五年级", openedPackages: ["英语套餐"] },
      continueCourse: {}, pendingHomework: [], materials: [], notices: []
    });
  }, {
    getStorageSync() {
      return "";
    }
  });

  page.loadHome();
  await flushPromises();
  await flushPromises();

  assert.equal(page.data.recommendations.length, 1);
  assert.equal(page.data.visibleRecommendations[0].contentSampleText, "阅读课程、阅读讲义");
  assert.equal(page.data.visibleRecommendations[0].recommendationReason, "同学习空间推荐");
});

test("home page does not expose a seven-day trial action", async () => {
  const calls = [];
  const page = loadHomePage((path, options) => {
    calls.push(["request", path, options && options.method, options && options.data]);
    return Promise.resolve({ student: {}, trial: { state: "eligible", remainingDays: 7 } });
  }, {
    getStorageSync() { return ""; }
  });

  page.loadHome();
  await flushPromises();
  await flushPromises();

  assert.equal(page.startTrial, undefined);
  assert.equal(calls.some((item) => item[1] === "/student/trial/start"), false);
});

test("home page guides student to contact teacher for recommendation", () => {
  const calls = [];
  const page = loadHomePage(() => Promise.resolve({}), {
    getStorageSync() {
      return "";
    },
    showModal(args) {
      calls.push(args);
    }
  });

  page.contactTeacher({ currentTarget: { dataset: { name: "英语阅读提升" } } });

  assert.equal(calls[0].title, "联系老师开通");
  assert.match(calls[0].content, /英语阅读提升/);
});

test("home page feedback shortcut opens latest classroom feedback", () => {
  const calls = [];
  const page = loadHomePage(() => Promise.resolve({}), {
    getStorageSync() {
      return "";
    },
    navigateTo(args) {
      calls.push(["navigateTo", args.url]);
    },
    showToast(args) {
      calls.push(["showToast", args.title]);
    }
  });
  page.setData({
    feedbackItems: [{ relatedSubmissionId: "sub-feedback-1" }]
  });

  page.handleShortcut({ currentTarget: { dataset: { action: "feedback" } } });

  assert.deepEqual(calls[0], ["navigateTo", "/pages/result/index?id=sub-feedback-1"]);
});

test("home page derives today todos when student home has legacy fields only", async () => {
  const page = loadHomePage(() => Promise.resolve({
    student: { id: "stu-001", grade: "五年级", openedPackages: ["英语套餐"] },
    continueCourse: { id: "course-001", name: "五年级英语S1Q1课程", grade: "五年级", subject: "英语", chapterCount: 8 },
    continueProgress: 50,
    pendingHomework: [
      { id: "hw-1", title: "英语阅读挑战", course: "五年级英语S1Q1课程", deadline: "2027-01-15", studentStatus: "待完成", questionCount: 2 }
    ],
    materials: [],
    notices: []
  }), {
    getStorageSync() {
      return "";
    }
  });

  page.loadHome();
  await flushPromises();

  assert.equal(page.data.todoItems.length, 3);
  assert.equal(page.data.todoItems[0].type, "homework");
  assert.equal(page.data.todoItems[0].title, "英语阅读挑战");
  assert.equal(page.data.todoItems[1].type, "schedule");
  assert.equal(page.data.todoItems[2].type, "subscribe");
});

test("home page requests mini program subscription messages from todo action", async () => {
  const calls = [];
  const page = loadHomePage((path, options) => {
    calls.push(["request", path, options && options.data]);
    return Promise.resolve({ title: "学习提醒", actionText: "已开启", templateIds: [SUBSCRIBE_TEMPLATE_ID] });
  }, {
    getStorageSync() {
      return "";
    },
    setStorageSync(key, value) {
      calls.push(["setStorageSync", key, value]);
    },
    requestSubscribeMessage(args) {
      calls.push(["requestSubscribeMessage", args.tmplIds]);
      args.success({ [SUBSCRIBE_TEMPLATE_ID]: "accept" });
    },
    showToast(args) {
      calls.push(["showToast", args.title]);
    }
  }, {
    globalData: { subscribeTemplateIds: [SUBSCRIBE_TEMPLATE_ID] }
  });
  page.setData({
    todoItems: [{ id: "todo-subscribe", type: "subscribe" }],
    subscriptionReminder: { title: "学习提醒", actionText: "开启提醒", templateIds: [SUBSCRIBE_TEMPLATE_ID] }
  });

  page.handleTodo({ currentTarget: { dataset: { type: "subscribe" } } });
  await flushPromises();
  await flushPromises();

  assert.deepEqual(calls.find((item) => item[0] === "requestSubscribeMessage"), ["requestSubscribeMessage", [SUBSCRIBE_TEMPLATE_ID]]);
  assert.deepEqual(calls.find((item) => item[0] === "request"), ["request", "/student/subscription", { templateIds: [SUBSCRIBE_TEMPLATE_ID] }]);
  assert.deepEqual(calls.find((item) => item[0] === "setStorageSync"), ["setStorageSync", "starline_subscribe_enabled", "1"]);
  assert.equal(page.data.subscribeEnabled, true);
  assert.equal(page.data.todoItems.length, 0);
});

test("home page opens notice tab when mini program subscribe templates are not configured", () => {
  const calls = [];
  const page = loadHomePage(() => Promise.resolve({}), {
    getStorageSync() {
      return "";
    },
    showToast(args) {
      calls.push(["showToast", args.title]);
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    }
  }, {
    globalData: { subscribeTemplateIds: [] }
  });
  page.setData({
    subscriptionReminder: { title: "学习提醒", actionText: "查看通知", templateIds: [] }
  });

  page.handleTodo({ currentTarget: { dataset: { type: "subscribe" } } });

  assert.deepEqual(calls.find((item) => item[0] === "showToast"), ["showToast", "提醒服务开通中，可先查看通知消息"]);
  assert.deepEqual(calls.find((item) => item[0] === "switchTab"), ["switchTab", "/pages/notices/index"]);
});

test("home page loads promo banners and resolves image urls against the api origin", async () => {
  const page = loadHomePage((path) => {
    if (path === "/student/banners") {
      return Promise.resolve([
        { id: "banner-1", imageUrl: "/api/banners/images/banner-1.jpg", linkType: "none", linkValue: "" }
      ]);
    }
    return Promise.resolve({});
  }, {}, { globalData: { apiBaseUrl: "https://gate.starlineeducation.com.cn/api" } });

  page.loadPromoBanners();
  await flushPromises();

  assert.equal(page.data.promoBanners.length, 1);
  assert.equal(page.data.promoBanners[0].imageUrl, "https://gate.starlineeducation.com.cn/api/banners/images/banner-1.jpg");
});

test("home page promo banner tap navigates to an in-app page", async () => {
  const calls = [];
  const page = loadHomePage((path) => {
    if (path === "/student/banners") {
      return Promise.resolve([{ id: "banner-1", imageUrl: "/img.jpg", linkType: "page", linkValue: "/pages/study/index" }]);
    }
    return Promise.resolve({});
  }, {
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    },
    navigateTo(args) {
      calls.push(["navigateTo", args.url]);
    }
  });

  page.loadPromoBanners();
  await flushPromises();
  page.handlePromoBannerTap({ currentTarget: { dataset: { id: "banner-1" } } });

  assert.deepEqual(calls, [["switchTab", "/pages/study/index"]]);
});

test("home page promo banner tap copies an external link instead of failing silently", async () => {
  const calls = [];
  const page = loadHomePage((path) => {
    if (path === "/student/banners") {
      return Promise.resolve([{ id: "banner-1", imageUrl: "/img.jpg", linkType: "url", linkValue: "https://example.com/promo" }]);
    }
    return Promise.resolve({});
  }, {
    setClipboardData(args) {
      calls.push(["setClipboardData", args.data]);
      args.success && args.success();
    },
    showToast(args) {
      calls.push(["showToast", args.title]);
    }
  });

  page.loadPromoBanners();
  await flushPromises();
  page.handlePromoBannerTap({ currentTarget: { dataset: { id: "banner-1" } } });

  assert.deepEqual(calls, [
    ["setClipboardData", "https://example.com/promo"],
    ["showToast", "链接已复制，请在浏览器打开"]
  ]);
});

test("home page ignores promo banner requests that fail instead of breaking the page", async () => {
  const page = loadHomePage((path) => {
    if (path === "/student/banners") {
      return Promise.reject(new Error("network error"));
    }
    return Promise.resolve({});
  });

  page.loadPromoBanners();
  await flushPromises();

  assert.deepEqual(page.data.promoBanners, []);
});
