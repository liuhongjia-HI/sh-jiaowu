const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

function loadAppConfig(envVersion) {
  delete require.cache[require.resolve("../app")];
  let appConfig = null;
  global.wx = {
    getStorageSync() {
      return "";
    },
    getAccountInfoSync() {
      return { miniProgram: { envVersion } };
    }
  };
  global.App = (config) => {
    appConfig = config;
  };
  require("../app");
  return appConfig;
}

test("schedule remains an inner page instead of occupying a tab bar slot", () => {
  const config = JSON.parse(fs.readFileSync(path.join(__dirname, "../app.json"), "utf8"));
  const tabPages = config.tabBar.list.map((item) => item.pagePath);

  assert.equal(config.pages.includes("pages/schedule/index"), true);
  assert.equal(tabPages.includes("pages/schedule/index"), false);
  assert.deepEqual(tabPages, [
    "pages/home/index",
    "pages/study/index",
    "pages/notices/index",
    "pages/me/index"
  ]);
});

test("develop build uses real WeChat login", () => {
  const config = loadAppConfig("develop");

  assert.equal(config.globalData.apiBaseUrl, "https://gate.starlineeducation.com.cn/api");
  assert.equal(config.globalData.useRealWechatLogin, true);
  assert.equal(config.globalData.demoLoginCode, undefined);
  assert.deepEqual(config.globalData.subscribeTemplateIds, ["vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0"]);
});

test("trial and release builds use real WeChat login", () => {
  const trial = loadAppConfig("trial");
  const release = loadAppConfig("release");
  assert.equal(trial.globalData.useRealWechatLogin, true);
  assert.equal(release.globalData.useRealWechatLogin, true);
  assert.equal(trial.globalData.apiBaseUrl, "https://gate.starlineeducation.com.cn/api");
  assert.equal(release.globalData.apiBaseUrl, "https://gate.starlineeducation.com.cn/api");
});

test("app launch does not call wechat login before phone binding", () => {
  delete require.cache[require.resolve("../app")];
  const calls = [];
  let appConfig = null;
  global.wx = {
    getAccountInfoSync() {
      return { miniProgram: { envVersion: "develop" } };
    },
    getStorageSync() {
      calls.push(["getStorageSync"]);
      return "";
    },
    login() {
      calls.push(["login"]);
    },
    request() {
      calls.push(["request"]);
    }
  };
  global.App = (config) => {
    appConfig = config;
  };
  require("../app");

  appConfig.onLaunch();

  assert.equal(calls.some((item) => item[0] === "login"), false);
  assert.equal(calls.some((item) => item[0] === "request"), false);
});

test("ensureLogin exchanges wx.login code for token", async () => {
  delete require.cache[require.resolve("../app")];
  const calls = [];
  let storedToken = "";
  let appConfig = null;
  global.wx = {
    getStorageSync(key) {
      return key === "starline_token" ? storedToken : "";
    },
    setStorageSync(key, value) {
      calls.push(["setStorageSync", key, value]);
      if (key === "starline_token") {
        storedToken = value;
      }
    },
    getAccountInfoSync() {
      return { miniProgram: { envVersion: "develop" } };
    },
    login(args) {
      calls.push(["login"]);
      args.success({ code: "wx-code" });
    },
    request(options) {
      calls.push(["request", options.url, options.data]);
      options.success({ data: { code: 0, data: { token: "silent-token" } } });
    }
  };
  global.App = (config) => {
    appConfig = config;
  };
  require("../app");

  const token = await appConfig.ensureLogin();

  assert.equal(token, "silent-token");
  assert.equal(storedToken, "silent-token");
  assert.deepEqual(calls.find((item) => item[0] === "login"), ["login"]);
  assert.deepEqual(
    calls.find((item) => item[0] === "request"),
    ["request", "https://gate.starlineeducation.com.cn/api/auth/wechat-login", { code: "wx-code" }]
  );
});
