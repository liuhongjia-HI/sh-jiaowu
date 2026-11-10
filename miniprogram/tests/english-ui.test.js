const assert = require("node:assert/strict");
const test = require("node:test");
const fs = require("node:fs");
const path = require("node:path");
const { formatDateTime } = require("../utils/date-time");

test("all registered pages and navigation labels use English static copy", () => {
  const root = path.join(__dirname, "..");
  const app = JSON.parse(fs.readFileSync(path.join(root, "app.json"), "utf8"));
  assert.doesNotMatch(JSON.stringify(app), /\p{Script=Han}/u);
  for (const page of app.pages) {
    const template = fs.readFileSync(path.join(root, `${page}.wxml`), "utf8").replace(/<!--[\s\S]*?-->/g, "");
    assert.doesNotMatch(template, /\p{Script=Han}/u, page);
    assert.doesNotMatch(fs.readFileSync(path.join(root, `${page}.json`), "utf8"), /\p{Script=Han}/u, page);
  }
});

test("date formatting is independent of device locale and handles missing or invalid input", () => {
  assert.equal(formatDateTime(new Date(2026, 0, 2, 3, 4).toISOString()), "2026-01-02 03:04");
  assert.equal(formatDateTime(""), "");
  assert.equal(formatDateTime(null), "");
  assert.equal(formatDateTime("未提供时间"), "未提供时间");
});
