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

### 文件处理依赖的外部命令行工具

后端不依赖这些工具就能启动，但缺少时对应功能会安静降级（接口返回“预览生成失败”或分页图片模式不可用），不会报 500。生产环境建议装齐：

- `soffice`（LibreOffice headless）：把学生上传的 PPT/Word 转成 PDF 预览。
  ```bash
  apt-get install -y libreoffice
  ```
- `gs`（Ghostscript）：学生端资料预览的防盗版能力全靠它——把当前学生的姓名/手机尾号/时间水印烧录进 PDF 内容本身（而不是前端盖一层可以被跳过的图层），并把烧录后的 PDF 逐页栅格化成图片下发，避免学生端拿到可复制、可转发的完整 PDF 文件。没装 `gs` 时会退回到“物理隔离但没有动态水印”的干净 PDF 预览，仍然比改动前安全，但达不到设计的防盗版效果。
  ```bash
  apt-get install -y ghostscript
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
