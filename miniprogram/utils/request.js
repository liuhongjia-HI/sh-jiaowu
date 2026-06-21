function request(path, options = {}) {
  const app = getApp();
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
