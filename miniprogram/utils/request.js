const LOGIN_RETURN_KEY = "starline_after_login";
let loginRedirectInFlight = false;

function request(path, options = {}) {
  const app = getApp();
  return ensureRequestAuth(app, path, options)
    .catch((error) => {
      if (options.skipAuth) {
        throw error;
      }
      if (shouldEnsureAuth(path, options)) {
        handleUnauthorized(error.message || "登录失败，请重新进入");
      }
      throw error;
    })
    .then(() => doRequest(app, path, options));
}

function doRequest(app, path, options = {}) {
  const baseUrl = app.globalData.apiBaseUrl;
  let loading = !options.silent;
  if (loading) {
    wx.showLoading({ title: "加载中" });
  }

  function finishLoading() {
    if (!loading) {
      return;
    }
    loading = false;
    wx.hideLoading();
  }

  return new Promise((resolve, reject) => {
    wx.request({
      url: `${baseUrl}${path}`,
      method: options.method || "GET",
      data: options.data || {},
      header: {
        "content-type": "application/json",
        Authorization: wx.getStorageSync("starline_token") ? `Bearer ${wx.getStorageSync("starline_token")}` : "",
        ...(options.header || {})
      },
      success(res) {
        const body = res.data || {};
        if (body.code === 0) {
          resolve(body.data);
          return;
        }
        finishLoading();
        if (res.statusCode === 401 || body.code === 401) {
          if (!shouldEnsureAuth(path, options)) {
            reject(new Error(body.message || "请求失败"));
            return;
          }
          handleUnauthorized(body.message || "登录已过期，请重新登录");
          reject(new Error(body.message || "登录已过期，请重新登录"));
          return;
        }
        wx.showToast({ title: body.message || "请求失败", icon: "none" });
        reject(new Error(body.message || "请求失败"));
      },
      fail(err) {
        finishLoading();
        const error = new Error("网络连接失败，请检查网络后重试");
        error.cause = err;
        error.userNotified = true;
        wx.showToast({ title: error.message, icon: "none" });
        reject(error);
      },
      complete() {
        finishLoading();
      }
    });
  });
}

function ensureRequestAuth(app, path, options = {}) {
  if (!shouldEnsureAuth(path, options)) {
    return Promise.resolve();
  }
  if (wx.getStorageSync("starline_token")) {
    return Promise.resolve();
  }
  return Promise.reject(new Error("请先完成登录绑定"));
}

function shouldEnsureAuth(path, options = {}) {
  return !options.skipAuth && path.indexOf("/auth/") !== 0 && path.indexOf("/student") === 0;
}

function handleUnauthorized(message) {
  const pages = getCurrentPages();
  const current = pages[pages.length - 1];
  if ((current && current.route === "pages/login/index") || loginRedirectInFlight) {
    return;
  }
  loginRedirectInFlight = true;
  wx.removeStorageSync("starline_token");
  rememberLoginDestination();
  wx.showToast({ title: message, icon: "none" });
  setTimeout(() => {
    wx.navigateTo({
      url: "/pages/login/index",
      fail() {
        wx.redirectTo({ url: "/pages/login/index" });
      }
    });
  }, 600);
}

function completeLoginRedirect() {
  loginRedirectInFlight = false;
}

function rememberLoginDestination() {
  const pages = getCurrentPages();
  const current = pages[pages.length - 1];
  if (!current || current.route === "pages/login/index") {
    return;
  }
  const destination = buildPagePath(current);
  if (destination && wx.setStorageSync) {
    wx.setStorageSync(LOGIN_RETURN_KEY, destination);
  }
}

function buildPagePath(page) {
  const route = String((page && page.route) || "").replace(/^\/+/, "");
  if (!route.startsWith("pages/") || route === "pages/login/index") {
    return "";
  }
  const options = (page && page.options) || {};
  const query = Object.keys(options)
    .filter((key) => ["string", "number", "boolean"].includes(typeof options[key]))
    .map((key) => `${encodeURIComponent(key)}=${encodeURIComponent(String(options[key]))}`)
    .join("&");
  return `/${route}${query ? `?${query}` : ""}`;
}

module.exports = { request, LOGIN_RETURN_KEY, completeLoginRedirect };
