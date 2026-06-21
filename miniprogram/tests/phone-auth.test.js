const assert = require("node:assert/strict");
const test = require("node:test");

function loadPhoneAuth(wxMock) {
  delete require.cache[require.resolve("../utils/phone-auth")];
  global.wx = wxMock;
  return require("../utils/phone-auth");
}

test("cancel detection follows getPhoneNumber errMsg", () => {
  const phoneAuth = loadPhoneAuth({});

  assert.equal(phoneAuth.isCancel({ errMsg: "getPhoneNumber:fail user deny" }), true);
  assert.equal(phoneAuth.isCancel({ errMsg: "getPhoneNumber:ok", code: "phone-code" }), false);
});
