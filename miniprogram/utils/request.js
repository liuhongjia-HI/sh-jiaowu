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
  let loading = true;
  wx.showLoading({ title: "加载中" });

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
          if (shouldEnsureAuth(path, options) && !options.__retried && app.ensureLogin) {
            wx.removeStorageSync("starline_token");
            app.ensureLogin({ force: true })
              .then(() => doRequest(app, path, { ...options, __retried: true }).then(resolve).catch(reject))
              .catch((error) => {
                handleUnauthorized(error.message || body.message || "登录已过期，请重新登录");
                reject(new Error(error.message || body.message || "登录已过期，请重新登录"));
              });
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
        wx.showToast({ title: "网络连接失败", icon: "none" });
        reject(err);
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
  if (!app.ensureLogin) {
    return Promise.resolve();
  }
  return app.ensureLogin();
}

function shouldEnsureAuth(path, options = {}) {
  return !options.skipAuth && path.indexOf("/auth/") !== 0 && path.indexOf("/student") === 0;
}

function handleUnauthorized(message) {
  wx.removeStorageSync("starline_token");
  wx.showToast({ title: message, icon: "none" });
  const pages = getCurrentPages();
  const current = pages[pages.length - 1];
  if (current && current.route === "pages/login/index") {
    return;
  }
  setTimeout(() => {
    wx.navigateTo({
      url: "/pages/login/index",
      fail() {
        wx.redirectTo({ url: "/pages/login/index" });
      }
    });
  }, 600);
}

module.exports = { request };
