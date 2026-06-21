const PRODUCTION_API_BASE_URL = "https://gate.starlineeducation.com.cn/api";

function resolveEnvVersion() {
  try {
    return wx.getAccountInfoSync().miniProgram.envVersion || "develop";
  } catch (error) {
    return "develop";
  }
}

function resolveApiBaseUrl() {
  const urls = {
    develop: PRODUCTION_API_BASE_URL,
    trial: PRODUCTION_API_BASE_URL,
    release: PRODUCTION_API_BASE_URL
  };
  const envVersion = resolveEnvVersion();
  return urls[envVersion] || urls.develop;
}

function resolveUseRealWechatLogin() {
  const envVersion = resolveEnvVersion();
  const apiBaseUrl = resolveApiBaseUrl();
  return envVersion !== "develop" || apiBaseUrl === PRODUCTION_API_BASE_URL;
}

App({
  onLaunch() {
    if (this.globalData.useRealWechatLogin) {
      this.ensureLogin().catch(() => {});
    }
  },
  ensureLogin(options = {}) {
    const cachedToken = wx.getStorageSync("starline_token");
    if (cachedToken && !options.force) {
      return Promise.resolve(cachedToken);
    }
    if (this.globalData.loginPromise && !options.force) {
      return this.globalData.loginPromise;
    }
    const promise = new Promise((resolve, reject) => {
      wx.login({
        success: (res) => {
          const code = this.globalData.useRealWechatLogin ? res.code : (this.globalData.demoLoginCode || "student");
          if (!code) {
            reject(new Error("微信登录失败"));
            return;
          }
          wx.request({
            url: `${this.globalData.apiBaseUrl}/auth/wechat-login`,
            method: "POST",
            data: { code },
            header: { "content-type": "application/json" },
            success: (loginRes) => {
              const body = loginRes.data || {};
              if (body.code === 0 && body.data && body.data.token) {
                wx.setStorageSync("starline_token", body.data.token);
                resolve(body.data.token);
                return;
              }
              reject(new Error(body.message || "微信登录失败"));
            },
            fail: reject
          });
        },
        fail: reject
      });
    });
    this.globalData.loginPromise = promise
      .then((token) => {
        this.globalData.loginPromise = null;
        return token;
      })
      .catch((error) => {
        this.globalData.loginPromise = null;
        throw error;
      });
    return this.globalData.loginPromise;
  },
  globalData: {
    apiBaseUrl: resolveApiBaseUrl(),
    // 生产环境置为 true：登录时上送 wx.login() 真实 code，由后端 jscode2session 换取 openId。
    // 演示环境保持 false：上送 demoLoginCode，后端用演示映射直接登录，无需微信凭据。
    useRealWechatLogin: resolveUseRealWechatLogin(),
    demoLoginCode: "student"
  }
});
