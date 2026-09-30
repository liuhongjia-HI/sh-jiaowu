const LOGIN_RETURN_KEY = "starline_after_login";
let loginRedirectInFlight = false;
let loginRedirectTimer = null;
let loginRedirectRevision = 0;

function request(path, options = {}) {
  const app = getApp();
  return ensureRequestAuth(app, path, options)
    .catch((error) => {
      if (options.skipAuth) {
        throw error;
      }
      if (shouldEnsureAuth(path, options)) {
        handleUnauthorized(error.message || "Login failed. Please try again.");
      }
      throw error;
    })
    .then(() => doRequest(app, path, options));
}

function doRequest(app, path, options = {}) {
  const baseUrl = app.globalData.apiBaseUrl;
  let loading = !options.silent;
  if (loading) {
    wx.showLoading({ title: "Loading" });
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
            reject(new Error(body.message || "Request failed"));
            return;
          }
          handleUnauthorized(body.message || "Session expired. Please log in again.");
          reject(new Error(body.message || "Session expired. Please log in again."));
          return;
        }
        wx.showToast({ title: body.message || "Request failed", icon: "none" });
        reject(new Error(body.message || "Request failed"));
      },
      fail(err) {
        finishLoading();
        const error = new Error("Connection failed. Check your network and try again.");
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
  return Promise.reject(new Error("Please log in and link your account first."));
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
  const revision = loginRedirectRevision;
  loginRedirectTimer = setTimeout(() => {
    if (revision !== loginRedirectRevision) return;
    loginRedirectTimer = null;
    wx.navigateTo({
      url: "/pages/login/index",
      fail() {
        if (revision !== loginRedirectRevision) return;
        wx.redirectTo({ url: "/pages/login/index" });
      }
    });
  }, 600);
}

function completeLoginRedirect() {
  loginRedirectRevision++;
  if (loginRedirectTimer !== null) {
    clearTimeout(loginRedirectTimer);
    loginRedirectTimer = null;
  }
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
