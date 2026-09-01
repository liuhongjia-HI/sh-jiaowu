const assert = require("node:assert/strict");
const test = require("node:test");

function loadRequestWithWx(wxMock, pages = [], appMock = null) {
  delete require.cache[require.resolve("../utils/request")];
  global.wx = wxMock;
  global.getApp = () => appMock || ({ globalData: { apiBaseUrl: "https://api.example.com/api" } });
  global.getCurrentPages = () => pages;
  return require("../utils/request").request;
}

test("request prefixes student API path and sends stored token", async () => {
  const calls = [];
  const wxMock = {
    getStorageSync(key) {
      return key === "starline_token" ? "student-token" : "";
    },
    showLoading(args) {
      calls.push(["showLoading", args]);
    },
    hideLoading() {
      calls.push(["hideLoading"]);
    },
    request(options) {
      calls.push(["request", options]);
      options.success({ statusCode: 200, data: { code: 0, message: "ok", data: { name: "home" } } });
      options.complete();
    }
  };
  const request = loadRequestWithWx(wxMock);

  const data = await request("/student/home");
  const requestCall = calls.find((item) => item[0] === "request")[1];

  assert.deepEqual(data, { name: "home" });
  assert.equal(requestCall.url, "https://api.example.com/api/student/home");
  assert.equal(requestCall.method, "GET");
  assert.equal(requestCall.header.Authorization, "Bearer student-token");
  assert.equal(calls.filter((item) => item[0] === "hideLoading").length, 1);
});

test("request turns wx network failures into one actionable user-facing error", async () => {
  const calls = [];
  const wxMock = {
    getStorageSync() {
      return "";
    },
    showLoading() {},
    hideLoading() {},
    showToast(args) {
      calls.push(["showToast", args]);
    },
    request(options) {
      options.fail({ errMsg: "request:fail url not in domain list" });
      options.complete();
    }
  };
  const request = loadRequestWithWx(wxMock);

  await assert.rejects(
    () => request("/auth/wechat-login", { method: "POST" }),
    (error) => error.message === "网络连接失败，请检查网络后重试" && error.userNotified === true
  );

  assert.deepEqual(calls, [[
    "showToast",
    { title: "网络连接失败，请检查网络后重试", icon: "none" }
  ]]);
});

test("request redirects to login before student API when token is missing", async () => {
  const calls = [];
  const originalSetTimeout = global.setTimeout;
  global.setTimeout = (fn) => {
    fn();
    return 0;
  };
  const wxMock = {
    getStorageSync() {
      return "";
    },
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    },
    showToast(args) {
      calls.push(["showToast", args]);
    },
    navigateTo(args) {
      calls.push(["navigateTo", args.url]);
    }
  };
  const appMock = {
    globalData: { apiBaseUrl: "https://api.example.com/api" },
    ensureLogin() {
      calls.push(["ensureLogin"]);
      return Promise.resolve("silent-token");
    }
  };
  const request = loadRequestWithWx(wxMock, [], appMock);

  await assert.rejects(() => request("/student/home"), /请先完成登录绑定/);
  global.setTimeout = originalSetTimeout;

  assert.equal(calls.some((item) => item[0] === "ensureLogin"), false);
  assert.equal(calls.some((item) => item[0] === "request"), false);
  assert.deepEqual(calls.find((item) => item[0] === "showToast"), [
    "showToast",
    { title: "请先完成登录绑定", icon: "none" }
  ]);
  assert.deepEqual(calls.find((item) => item[0] === "navigateTo"), ["navigateTo", "/pages/login/index"]);
});

test("request remembers the protected page before redirecting an unbound visitor", async () => {
  const calls = [];
  const originalSetTimeout = global.setTimeout;
  global.setTimeout = (fn) => {
    fn();
    return 0;
  };
  const wxMock = {
    getStorageSync() {
      return "";
    },
    setStorageSync(key, value) {
      calls.push(["setStorageSync", key, value]);
    },
    removeStorageSync() {},
    showToast() {},
    navigateTo(args) {
      calls.push(["navigateTo", args.url]);
    }
  };
  const request = loadRequestWithWx(wxMock, [{
    route: "pages/material-preview/index",
    options: { id: "material-001" }
  }]);

  await assert.rejects(() => request("/student/materials/material-001"), /请先完成登录绑定/);
  global.setTimeout = originalSetTimeout;

  assert.deepEqual(calls.find((item) => item[0] === "setStorageSync"), [
    "setStorageSync",
    "starline_after_login",
    "/pages/material-preview/index?id=material-001"
  ]);
  assert.deepEqual(calls.find((item) => item[0] === "navigateTo"), ["navigateTo", "/pages/login/index"]);
});

test("concurrent protected requests send an unbound visitor to binding only once", async () => {
  const calls = [];
  const originalSetTimeout = global.setTimeout;
  global.setTimeout = (fn) => {
    fn();
    return 0;
  };
  const wxMock = {
    getStorageSync() {
      return "";
    },
    setStorageSync() {},
    removeStorageSync() {},
    showToast() {},
    navigateTo(args) {
      calls.push(args.url);
    }
  };
  const request = loadRequestWithWx(wxMock, [{ route: "pages/study/index" }]);

  await Promise.allSettled([
    request("/student/study"),
    request("/student/favorites", { silent: true })
  ]);
  global.setTimeout = originalSetTimeout;

  assert.deepEqual(calls, ["/pages/login/index"]);
});

test("request clears token and redirects when student session expires", async () => {
  const calls = [];
  const originalSetTimeout = global.setTimeout;
  global.setTimeout = (fn) => {
    fn();
    return 0;
  };
  const wxMock = {
    getStorageSync() {
      return "expired-token";
    },
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    },
    showLoading() {},
    hideLoading() {},
    showToast(args) {
      calls.push(["showToast", args]);
    },
    navigateTo(args) {
      calls.push(["navigateTo", args.url]);
      args.fail && args.fail();
    },
    redirectTo(args) {
      calls.push(["redirectTo", args.url]);
    },
    request(options) {
      options.success({ statusCode: 401, data: { code: 401, message: "登录已过期" } });
      options.complete();
    }
  };
  const request = loadRequestWithWx(wxMock, [{ route: "pages/home/index" }]);

  await assert.rejects(() => request("/student/home"), /登录已过期/);
  global.setTimeout = originalSetTimeout;

  assert.deepEqual(calls.find((item) => item[0] === "removeStorageSync"), ["removeStorageSync", "starline_token"]);
  assert.deepEqual(calls.find((item) => item[0] === "navigateTo"), ["navigateTo", "/pages/login/index"]);
  assert.deepEqual(calls.find((item) => item[0] === "redirectTo"), ["redirectTo", "/pages/login/index"]);
});

test("request does not silently exchange wx code after student API returns unauthorized", async () => {
  const calls = [];
  const originalSetTimeout = global.setTimeout;
  global.setTimeout = (fn) => {
    fn();
    return 0;
  };
  const wxMock = {
    getStorageSync() {
      return "expired-token";
    },
    removeStorageSync(key) {
      calls.push(["removeStorageSync", key]);
    },
    showLoading() {},
    hideLoading() {},
    showToast(args) {
      calls.push(["showToast", args]);
    },
    navigateTo(args) {
      calls.push(["navigateTo", args.url]);
    },
    request(options) {
      calls.push(["request", options.url]);
      options.success({ statusCode: 401, data: { code: 401, message: "登录已过期" } });
      options.complete();
    }
  };
  const appMock = {
    globalData: { apiBaseUrl: "https://api.example.com/api" },
    ensureLogin() {
      calls.push(["ensureLogin"]);
      return Promise.resolve("silent-token");
    }
  };
  const request = loadRequestWithWx(wxMock, [{ route: "pages/home/index" }], appMock);

  await assert.rejects(() => request("/student/home"), /登录已过期/);
  global.setTimeout = originalSetTimeout;

  assert.deepEqual(calls.filter((item) => item[0] === "request"), [["request", "https://api.example.com/api/student/home"]]);
  assert.equal(calls.some((item) => item[0] === "ensureLogin"), false);
  assert.deepEqual(calls.find((item) => item[0] === "navigateTo"), ["navigateTo", "/pages/login/index"]);
});

test("auth API unauthorized response does not trigger global login redirect", async () => {
  const calls = [];
  const wxMock = {
    getStorageSync() {
      return "";
    },
    showLoading() {},
    hideLoading() {},
    showToast(args) {
      calls.push(["showToast", args]);
    },
    navigateTo(args) {
      calls.push(["navigateTo", args.url]);
    },
    request(options) {
      calls.push(["request", options.url, options.data]);
      options.success({ statusCode: 401, data: { code: 401, message: "微信账号未绑定" } });
      options.complete();
    }
  };
  const request = loadRequestWithWx(wxMock, [{ route: "pages/login/index" }]);

  await assert.rejects(() => request("/auth/wechat-login", { method: "POST", data: { code: "wx-code" } }), /微信账号未绑定/);

  assert.deepEqual(calls.find((item) => item[0] === "request"), [
    "request",
    "https://api.example.com/api/auth/wechat-login",
    { code: "wx-code" }
  ]);
  assert.equal(calls.some((item) => item[0] === "navigateTo"), false);
  assert.equal(calls.some((item) => item[0] === "showToast"), false);
});
