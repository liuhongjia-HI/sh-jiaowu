function resolveApiBaseUrl() {
  // 体验版/正式版发布前，把下面域名替换为已备案并加入 request 合法域名的 API 域名。
  const urls = {
    develop: "http://127.0.0.1:8892/api",
    trial: "https://api.starline.example.com/api",
    release: "https://api.starline.example.com/api"
  };
  try {
    const envVersion = wx.getAccountInfoSync().miniProgram.envVersion || "develop";
    return urls[envVersion] || urls.develop;
  } catch (error) {
    return urls.develop;
  }
}

function resolveUseRealWechatLogin() {
  try {
    const envVersion = wx.getAccountInfoSync().miniProgram.envVersion || "develop";
    return envVersion !== "develop";
  } catch (error) {
    return false;
  }
}

App({
  globalData: {
    apiBaseUrl: resolveApiBaseUrl(),
    // 生产环境置为 true：登录时上送 wx.login() 真实 code，由后端 jscode2session 换取 openId。
    // 演示环境保持 false：上送 demoLoginCode，后端用演示映射直接登录，无需微信凭据。
    useRealWechatLogin: resolveUseRealWechatLogin(),
    demoLoginCode: "student"
  }
});
