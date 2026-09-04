const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

function loadPage(wxMock = {}) {
  const pages = [];
  const pagePath = require.resolve("../pages/parent-onboarding/index.js");
  delete require.cache[pagePath];
  global.wx = wxMock;
  global.Page = (definition) => pages.push(definition);
  require(pagePath);
  const definition = pages[0];
  const page = {
    data: JSON.parse(JSON.stringify(definition.data)),
    setData(patch) {
      Object.keys(patch).forEach((key) => {
        const parts = key.split(".");
        if (parts.length === 1) this.data[key] = patch[key];
        else this.data[parts[0]][parts[1]] = patch[key];
      });
    }
  };
  Object.keys(definition).forEach((key) => {
    if (key !== "data") page[key] = typeof definition[key] === "function" ? definition[key].bind(page) : definition[key];
  });
  return page;
}

test("parent onboarding explains the service and keeps the add-student path simple", () => {
  const template = fs.readFileSync(path.join(__dirname, "../pages/parent-onboarding/index.wxml"), "utf8");
  assert.match(template, /准备好后，添加一位学生/);
  assert.match(template, /添加学生/);
  assert.match(template, /已有学生信息，直接绑定/);
  assert.match(template, /wx:for="\{\{benefits\}\}"/);
  assert.doesNotMatch(template, /孩子所在年级|gradeOptions|onGradeChange/);
});

test("parent onboarding presents the service explanation before the binding form", () => {
  const template = fs.readFileSync(path.join(__dirname, "../pages/parent-onboarding/index.wxml"), "utf8");
  assert(template.indexOf("添加后可以查看") < template.indexOf("准备好后，添加一位学生"));
});

test("parent onboarding opens the binding form without collecting duplicate grade information", () => {
  const calls = [];
  const page = loadPage({ showToast: (args) => calls.push(args), navigateTo: (args) => calls.push(args) });
  page.goAddStudent();
  assert.equal(calls[0].url, "/pages/login/index?mode=add");
});

test("parent onboarding keeps direct binding as a separate simple entry", () => {
  const calls = [];
  const page = loadPage({ navigateTo: (args) => calls.push(args) });
  page.goBindStudent();
  assert.equal(calls[0].url, "/pages/login/index?mode=bind");
});
