const assert = require("node:assert/strict");
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
