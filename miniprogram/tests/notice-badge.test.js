const assert = require("node:assert/strict");
const test = require("node:test");
const { refreshNoticeBadge, updateNoticeBadge } = require("../utils/notice-badge");
test("badge caps counts, clears on account change/logout and ignores stale responses", () => {
  let token = "a";
  const calls = [], pending = [];
  global.getApp = () => ({ globalData: { apiBaseUrl: "/api" } });
  global.wx = {
    getStorageSync: () => token,
    setTabBarBadge: ({ text }) => calls.push(text),
    removeTabBarBadge: () => calls.push("0"),
    request: options => pending.push(options)
  };
  updateNoticeBadge(Array.from({ length: 100 }, () => ({ isRead: false })));
  assert.equal(calls.at(-1), "99+");
  refreshNoticeBadge();
  token = "b";
  refreshNoticeBadge();
  assert.equal(calls.at(-1), "0");
  pending[0].success({ data: { code: 0, data: [{ isRead: false }] } });
  assert.equal(calls.at(-1), "0");
  pending[1].success({ data: { code: 0, data: [{ isRead: false }, { isRead: true }] } });
  assert.equal(calls.at(-1), "1");
  refreshNoticeBadge();
  updateNoticeBadge([]);
  pending[2].success({ data: { code: 0, data: [{ isRead: false }] } });
  assert.equal(calls.at(-1), "0");
  token = "";
  refreshNoticeBadge();
  assert.equal(calls.at(-1), "0");
  assert.equal(pending.length, 3);
});
