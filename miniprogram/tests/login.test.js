const assert = require("node:assert/strict");
const test = require("node:test");

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
  assert.equal(calls.some((item) => item[1] === "微信登录失败，请重试"), true);
});
