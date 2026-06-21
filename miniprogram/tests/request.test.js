const assert = require("node:assert/strict");
const test = require("node:test");

function loadRequestWithWx(wxMock, pages = []) {
  delete require.cache[require.resolve("../utils/request")];
  global.wx = wxMock;
  global.getApp = () => ({ globalData: { apiBaseUrl: "https://api.example.com/api" } });
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
