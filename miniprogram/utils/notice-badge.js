// Ignore stale responses after account changes or a newer read-state update.
let revision = 0;
let badgeToken = "";
function currentToken() {
  return wx.getStorageSync ? wx.getStorageSync("starline_token") || "" : "";
}
function render(count) {
  if (count > 0 && wx.setTabBarBadge) {
    wx.setTabBarBadge({ index: 2, text: count > 99 ? "99+" : String(count) });
  } else if (wx.removeTabBarBadge) {
    wx.removeTabBarBadge({ index: 2 });
  }
}
function updateNoticeBadge(notices) {
  revision++;
  badgeToken = currentToken();
  render(notices.filter((item) => item.isRead !== true).length);
}
function refreshNoticeBadge() {
  const token = currentToken();
  const version = ++revision;
  if (token !== badgeToken || !token) render(0);
  badgeToken = token;
  if (!token || typeof getApp !== "function") return;
  const app = getApp();
  if (!app || !app.globalData) return;
  // Background refreshes must not show loading indicators or interrupt navigation.
  wx.request({
    url: `${app.globalData.apiBaseUrl}/student/notices`,
    header: { Authorization: `Bearer ${token}` },
    success(res) {
      if (version !== revision || token !== currentToken()) return;
      const body = res.data || {};
      if (body.code === 0 && Array.isArray(body.data)) {
        render(body.data.filter((item) => item.isRead !== true).length);
      } else if (res.statusCode === 401 || body.code === 401) {
        render(0);
      }
    },
    fail() {}
  });
}
module.exports = { refreshNoticeBadge, updateNoticeBadge };
