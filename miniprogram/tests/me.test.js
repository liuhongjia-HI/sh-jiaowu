const assert = require("node:assert/strict");
const test = require("node:test");

function loadMePage(requestImpl, wxMock) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const pagePath = require.resolve("../pages/me/index.js");
  delete require.cache[requestPath];
  delete require.cache[pagePath];
  require.cache[requestPath] = {
    id: requestPath,
    filename: requestPath,
    loaded: true,
    exports: { request: requestImpl }
  };
  global.wx = wxMock;
  global.Page = (definition) => pages.push(definition);
  require(pagePath);
  const definition = pages[0];
  const page = {
    data: JSON.parse(JSON.stringify(definition.data)),
    setData(patch, callback) {
      Object.entries(patch).forEach(([key, value]) => {
        if (!key.includes(".")) {
          this.data[key] = value;
          return;
        }
        const parts = key.split(".");
        let target = this.data;
        parts.slice(0, -1).forEach((part) => {
          if (!target[part]) target[part] = {};
          target = target[part];
        });
        target[parts[parts.length - 1]] = value;
      });
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

test("me page submits student profile information from the mini program form", async () => {
  const calls = [];
  const updatedStudent = {
    id: "stu-closed-loop",
    name: "小星",
    nickname: "Star",
    avatarUrl: "https://example.com/avatar.png",
    grade: "五年级",
    schoolName: "星线小学",
    guardianName: "星星家长",
    openedPackages: [],
    learningStatus: "学习中",
    accountStatus: "正常",
    streakDays: 0,
    averageScore: 0,
    badgeCount: 0
  };
  const page = loadMePage((url, options = {}) => {
    calls.push(["request", url, options]);
    return Promise.resolve(updatedStudent);
  }, {
    showToast(args) {
      calls.push(["showToast", args]);
    }
  });
  page.setData({
    me: {
      id: "stu-closed-loop",
      name: "小星",
      nickname: "",
      avatarUrl: "",
      grade: "四年级",
      schoolName: "旧学校",
      guardianName: "",
      openedPackages: [],
      learningStatus: "未开始",
      accountStatus: "正常",
      streakDays: 0,
      averageScore: 0,
      badgeCount: 0
    },
    home: { student: updatedStudent },
    profileForm: {
      nickname: "Star",
      avatarUrl: "https://example.com/avatar.png",
      studentName: " 小星 ",
      grade: " 五年级 ",
      schoolName: " 星线小学 ",
      guardianName: " 星星家长 "
    }
  });

  page.submitBasicProfile();
  await flushPromises();

  const requestCall = calls.find((item) => item[0] === "request");
  assert.equal(requestCall[1], "/student/profile");
  assert.deepEqual(requestCall[2], {
    method: "PUT",
    data: {
      nickname: "Star",
      avatarUrl: "https://example.com/avatar.png",
      studentName: "小星",
      grade: "五年级",
      schoolName: "星线小学",
      guardianName: "星星家长"
    }
  });
  assert.equal(page.data.savingBasicProfile, false);
  assert.equal(page.data.profileEditing, false);
  assert.equal(page.data.me.guardianName, "星星家长");
  assert.deepEqual(calls.find((item) => item[0] === "showToast"), [
    "showToast",
    { title: "资料已保存", icon: "success" }
  ]);
});

test("me page blocks profile submission when required fields are missing", () => {
  const calls = [];
  const page = loadMePage((url, options = {}) => {
    calls.push(["request", url, options]);
    return Promise.resolve({});
  }, {
    showToast(args) {
      calls.push(["showToast", args]);
    }
  });
  page.setData({
    profileForm: {
      nickname: "",
      avatarUrl: "",
      studentName: "",
      grade: "五年级",
      schoolName: "星线小学",
      guardianName: ""
    }
  });

  page.submitBasicProfile();

  assert.equal(calls.some((item) => item[0] === "request"), false);
  assert.deepEqual(calls.find((item) => item[0] === "showToast"), [
    "showToast",
    { title: "请输入学生姓名", icon: "none" }
  ]);
});
