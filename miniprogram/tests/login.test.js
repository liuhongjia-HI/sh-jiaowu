const assert = require("node:assert/strict");
const test = require("node:test");
const fs = require("node:fs");
const path = require("node:path");

function loadLoginPage(requestImpl, wxMock) {
  const pages = [];
  const requestPath = require.resolve("../utils/request");
  const pagePath = require.resolve("../pages/login/index.js");
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
      Object.keys(patch).forEach((key) => {
        const parts = key.split(".");
        if (parts.length === 1) {
          this.data[key] = patch[key];
          return;
        }
        let target = this.data;
        for (let index = 0; index < parts.length - 1; index += 1) {
          target = target[parts[index]];
        }
        target[parts[parts.length - 1]] = patch[key];
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

test("login page silently restores bound wechat session on load", async () => {
  const calls = [];
  const page = loadLoginPage((path, options) => {
    calls.push(["request", path, options]);
    return Promise.resolve({ token: "restored-token" });
  }, {
    login(args) {
      calls.push(["login"]);
      args.success({ code: "wx-login-code" });
    },
    setStorageSync(key, value) {
      calls.push(["setStorageSync", key, value]);
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    },
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    }
  });

  page.onLoad();
  await flushPromises();

  assert.deepEqual(calls.find((item) => item[0] === "request"), [
    "request",
    "/auth/wechat-login",
    { method: "POST", data: { code: "wx-login-code" } }
  ]);
  assert.deepEqual(calls.find((item) => item[0] === "setStorageSync"), ["setStorageSync", "starline_token", "restored-token"]);
  assert.deepEqual(calls.find((item) => item[0] === "switchTab"), ["switchTab", "/pages/home/index"]);
  assert.equal(calls.some((item) => item[0] === "removeStorageSync"), false);
});

test("login page owns its back affordance so return works even without a page stack", () => {
  const config = JSON.parse(fs.readFileSync(path.join(__dirname, "../pages/login/index.json"), "utf8"));
  const template = fs.readFileSync(path.join(__dirname, "../pages/login/index.wxml"), "utf8");

  assert.equal(config.navigationStyle, "custom");
  assert.match(template, /class="login-nav-back"[^>]*bindtap="leaveLogin"/);
});

test("login page returns to the protected material that triggered binding", async () => {
  const calls = [];
  const page = loadLoginPage(() => Promise.resolve({ token: "restored-token" }), {
    login(args) {
      args.success({ code: "wx-login-code" });
    },
    getStorageSync(key) {
      return key === "starline_after_login" ? "/pages/material-preview/index?id=material-001" : "";
    },
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    },
    setStorageSync() {},
    redirectTo(args) {
      calls.push(["redirectTo", args.url]);
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    }
  });

  page.onLoad();
  await flushPromises();

  assert.deepEqual(calls.find((item) => item[0] === "removeStorageSync"), ["removeStorageSync", "starline_after_login"]);
  assert.deepEqual(calls.find((item) => item[0] === "redirectTo"), ["redirectTo", "/pages/material-preview/index?id=material-001"]);
  assert.equal(calls.some((item) => item[0] === "switchTab"), false);
});

test("login page keeps profile binding form when wechat is not bound", async () => {
  const calls = [];
  const page = loadLoginPage((path, options) => {
    calls.push(["request", path, options]);
    return Promise.reject(new Error("微信账号未绑定，请先填写学生信息并授权手机号完成身份绑定"));
  }, {
    login(args) {
      args.success({ code: "wx-login-code" });
    },
    setStorageSync(key, value) {
      calls.push(["setStorageSync", key, value]);
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    },
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    }
  });

  page.onLoad();
  await flushPromises();

  assert.deepEqual(calls.find((item) => item[0] === "request"), [
    "request",
    "/auth/wechat-login",
    { method: "POST", data: { code: "wx-login-code" } }
  ]);
  assert.equal(calls.some((item) => item[0] === "setStorageSync"), false);
  assert.equal(calls.some((item) => item[0] === "switchTab"), false);
  assert.equal(calls.some((item) => item[0] === "removeStorageSync"), false);
});

test("login page sends wx code, phone code, and student profile when binding phone", async () => {
  const calls = [];
  const page = loadLoginPage((path, options) => {
    calls.push(["request", path, options]);
    return Promise.resolve({ token: "student-token" });
  }, {
    login(args) {
      calls.push(["login"]);
      args.success({ code: "wx-login-code" });
    },
    setStorageSync(key, value) {
      calls.push(["setStorageSync", key, value]);
    },
    showToast(args) {
      calls.push(["showToast", args]);
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    }
  });
  page.setData({
    "form.studentName": " 小明 ",
    "form.schoolName": " 星河小学 ",
    "form.grade": " 五年级 "
  });

  page.bindPhone({ detail: { errMsg: "getPhoneNumber:ok", code: "phone-code" } });
  await flushPromises();

  assert.deepEqual(calls.find((item) => item[0] === "login"), ["login"]);
  assert.deepEqual(calls.find((item) => item[0] === "request"), [
    "request",
    "/auth/wechat-login",
    {
      method: "POST",
      data: {
        code: "wx-login-code",
        phoneCode: "phone-code",
        studentName: "小明",
        schoolName: "星河小学",
        grade: "五年级"
      }
    }
  ]);
  assert.deepEqual(calls.find((item) => item[0] === "setStorageSync"), ["setStorageSync", "starline_token", "student-token"]);
  assert.deepEqual(calls.find((item) => item[0] === "switchTab"), ["switchTab", "/pages/home/index"]);
});

test("login page lets users return home after cancelling phone authorization", () => {
  const calls = [];
  const page = loadLoginPage(() => Promise.resolve({ token: "unused" }), {
    showModal(args) {
      calls.push(["showModal", args.title, args.content, args.confirmText, args.cancelText]);
      args.success({ confirm: true });
    },
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    },
    showToast(args) {
      calls.push(["showToast", args.title]);
    }
  });

  page.bindPhone({ detail: { errMsg: "getPhoneNumber:fail user deny" } });

  assert.deepEqual(calls.find((item) => item[0] === "showModal"), [
    "showModal",
    "已取消手机号授权",
    "你可以继续填写资料，也可以先返回首页，之后再完成绑定。",
    "返回首页",
    "继续填写"
  ]);
  assert.deepEqual(calls.find((item) => item[0] === "removeStorageSync"), ["removeStorageSync", "starline_after_login"]);
  assert.deepEqual(calls.find((item) => item[0] === "switchTab"), ["switchTab", "/pages/home/index"]);
  assert.equal(calls.some((item) => item[0] === "showToast"), false, "cancel should not be replaced by form validation feedback");
});

test("login page keeps the form when users cancel authorization and choose to continue", () => {
  const calls = [];
  const page = loadLoginPage(() => Promise.resolve({ token: "unused" }), {
    showModal(args) {
      calls.push(["showModal", args.title]);
      args.success({ confirm: false });
    },
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    }
  });

  page.bindPhone({ detail: { errMsg: "getPhoneNumber:fail user deny" } });

  assert.deepEqual(calls, [["showModal", "已取消手机号授权"]]);
});

test("login page falls back to relaunching home when switchTab fails", () => {
  const calls = [];
  const page = loadLoginPage(() => Promise.resolve({ token: "unused" }), {
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
      args.fail();
    },
    reLaunch(args) {
      calls.push(["reLaunch", args.url]);
    }
  });

  page.leaveLogin();

  assert.deepEqual(calls, [
    ["removeStorageSync", "starline_after_login"],
    ["switchTab", "/pages/home/index"],
    ["reLaunch", "/pages/home/index"]
  ]);
});

test("leaving login page clears a stale protected-page destination", () => {
  const calls = [];
  const page = loadLoginPage(() => Promise.resolve({ token: "unused" }), {
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    }
  });

  page.onUnload();

  assert.deepEqual(calls, [["removeStorageSync", "starline_after_login"]]);
});

test("login page uses selected grade option when binding phone", async () => {
  const calls = [];
  const page = loadLoginPage((path, options) => {
    calls.push(["request", path, options]);
    return Promise.resolve({ token: "student-token" });
  }, {
    login(args) {
      args.success({ code: "wx-login-code" });
    },
    setStorageSync() {},
    showToast() {},
    switchTab() {}
  });
  page.setData({
    "form.studentName": "王同学",
    "form.schoolName": "乐成学校"
  });

  page.onGradeChange({ detail: { value: "4" } });
  page.bindPhone({ detail: { errMsg: "getPhoneNumber:ok", code: "phone-code" } });
  await flushPromises();

  assert.equal(page.data.form.grade, "五年级");
  assert.equal(calls.find((item) => item[0] === "request")[2].data.grade, "五年级");
});

test("login page keeps the network error instead of overwriting it with generic login failure", async () => {
  const calls = [];
  const networkError = new Error("网络连接失败，请检查网络后重试");
  networkError.userNotified = true;
  const page = loadLoginPage(() => Promise.reject(networkError), {
    login(args) {
      args.success({ code: "wx-login-code" });
    },
    showToast(args) {
      calls.push(["showToast", args.title]);
    }
  });
  page.setData({
    "form.studentName": "小明",
    "form.schoolName": "星河小学",
    "form.grade": "五年级"
  });

  page.bindPhone({ detail: { errMsg: "getPhoneNumber:ok", code: "phone-code" } });
  await flushPromises();

  assert.equal(calls.some((item) => item[1] === "登录失败"), false);
});

test("login page offers a picker and resubmits with selectedStudentId when the phone matches multiple children", async () => {
  const calls = [];
  const requests = [];
  const page = loadLoginPage((path, options) => {
    requests.push(options.data);
    calls.push(["request", path, options]);
    if (requests.length === 1) {
      return Promise.resolve({
        needsSelection: true,
        candidates: [
          { studentId: "stu-a", name: "大娃", grade: "五年级" },
          { studentId: "stu-b", name: "二娃", grade: "三年级" }
        ]
      });
    }
    return Promise.resolve({ token: "sibling-token" });
  }, {
    login(args) {
      args.success({ code: "wx-login-code" });
    },
    setStorageSync(key, value) {
      calls.push(["setStorageSync", key, value]);
    },
    showToast(args) {
      calls.push(["showToast", args]);
    },
    showActionSheet(args) {
      calls.push(["showActionSheet", args.itemList]);
      args.success({ tapIndex: 1 });
    },
    switchTab(args) {
      calls.push(["switchTab", args.url]);
    }
  });
  page.setData({
    "form.studentName": "大娃",
    "form.schoolName": "星河小学",
    "form.grade": "五年级"
  });

  page.bindPhone({ detail: { errMsg: "getPhoneNumber:ok", code: "phone-code" } });
  await flushPromises();
  await flushPromises();

  assert.equal(requests.length, 2, "expected a first request and a resubmission after picking");
  assert.equal(requests[0].selectedStudentId, undefined);
  assert.equal(requests[1].selectedStudentId, "stu-b");
  assert.deepEqual(calls.find((item) => item[0] === "showActionSheet"), ["showActionSheet", ["大娃 · 五年级", "二娃 · 三年级"]]);
  assert.deepEqual(calls.find((item) => item[0] === "setStorageSync"), ["setStorageSync", "starline_token", "sibling-token"]);
  assert.deepEqual(calls.find((item) => item[0] === "switchTab"), ["switchTab", "/pages/home/index"]);
  assert.equal(page.data.binding, false);
});

test("login page does not submit binding when wx.login returns no code", async () => {
  const calls = [];
  const page = loadLoginPage((path, options) => {
    calls.push(["request", path, options]);
    return Promise.resolve({ token: "student-token" });
  }, {
    login(args) {
      args.success({});
    },
    showToast(args) {
      calls.push(["showToast", args.title]);
    }
  });
  page.setData({
    "form.studentName": "小明",
    "form.schoolName": "星河小学",
    "form.grade": "五年级"
  });

  page.bindPhone({ detail: { errMsg: "getPhoneNumber:ok", code: "phone-code" } });
  await flushPromises();

  assert.equal(calls.some((item) => item[0] === "request"), false);
  assert.equal(calls.some((item) => item[1] === "登录失败，请重试"), true);
});
