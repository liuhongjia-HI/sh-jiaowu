# Starline 生产部署

`master` 分支推送后，GitHub Actions 会构建后端二进制和管理后台静态文件，并上传到服务器 `/opt/starline/releases/<commit>`，再切换 `/opt/starline/current` 并重启 `starline-api`。

## GitHub Secrets

仓库 `Settings -> Secrets and variables -> Actions` 需要配置：

- `DEPLOY_HOST`：服务器 IP，例如 `111.229.119.133`
- `DEPLOY_USER`：SSH 用户，默认可用 `root`
- `DEPLOY_PORT`：SSH 端口，默认 `22`
- `DEPLOY_SSH_KEY`：可登录服务器的私钥内容

## 服务器初始化

首次部署前在服务器执行一次：

```bash
mkdir -p /opt/starline/releases /etc/starline /etc/ssl/starline
cp deploy/production/starline-api.service /etc/systemd/system/starline-api.service
cp deploy/production/nginx-http.conf /etc/nginx/conf.d/starline.conf
systemctl daemon-reload
systemctl enable starline-api
nginx -t && systemctl reload nginx
```

再创建 `/etc/starline/learning-api.env`，写入生产环境变量：

```bash
APP_ENV=production
HTTP_PORT=8892
AUTH_TOKEN_SECRET=<高强度随机密钥>
MYSQL_DSN=<生产 MySQL DSN>
WECHAT_APPID=<微信小程序 AppID>
WECHAT_SECRET=<微信小程序 Secret>
DEMO_SEED_DATA=false
DEMO_STUDENT_LOGIN_ENABLED=false
ADMIN_PASSWORD_LOGIN_ENABLED=true
```

## 域名

- 管理后台：`sa.starlineeducation.com.cn`
- 接口域名：`gate.starlineeducation.com.cn`
- 管理后台和小程序统一通过 `https://gate.starlineeducation.com.cn/api` 访问接口；管理后台生产构建建议显式设置：
  ```bash
  VITE_API_BASE_URL=https://gate.starlineeducation.com.cn/api
  ```
- 当前可先启用 `nginx-http.conf`
- HTTPS 证书签发完成后，把证书放到 `/etc/ssl/starline/sa.starlineeducation.com.cn.pem`、`/etc/ssl/starline/sa.starlineeducation.com.cn.key`、`/etc/ssl/starline/gate.starlineeducation.com.cn.pem`、`/etc/ssl/starline/gate.starlineeducation.com.cn.key`，再把 `nginx-https.conf` 合并进 Nginx 站点配置并 reload。
