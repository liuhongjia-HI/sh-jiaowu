const assert = require("node:assert/strict");
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

test("develop build uses real WeChat login", () => {
  const config = loadAppConfig("develop");

  assert.equal(config.globalData.apiBaseUrl, "https://gate.starlineeducation.com.cn/api");
  assert.equal(config.globalData.useRealWechatLogin, true);
  assert.equal(config.globalData.demoLoginCode, undefined);
});

test("trial and release builds use real WeChat login", () => {
  assert.equal(loadAppConfig("trial").globalData.useRealWechatLogin, true);
  assert.equal(loadAppConfig("release").globalData.useRealWechatLogin, true);
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
