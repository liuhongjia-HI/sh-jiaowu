# Starline 生产部署

`master` 分支推送后，GitHub Actions 会构建后端二进制和管理后台静态文件，并上传到服务器 `/opt/starline/releases/<commit>`，再切换 `/opt/starline/current` 并重启 `starline-api`。

课件文件不再写入 release 目录。生产环境使用独立持久化目录 `/opt/starline/data/uploads`，服务账号为 `starline`；发布脚本会创建目录和账号，但不会迁移旧 release 中的历史课件。历史文件缺失时需由老师重新上传。

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

生产环境必须安装 LibreOffice 和 Ghostscript。发布激活前会执行 `check-preview-runtime.sh`，依赖缺失时停止切换版本，避免出现“上传成功但无法预览”。首次部署或旧服务器补齐依赖时执行：

```bash
sudo /opt/starline/current/deploy/production/provision-preview-runtime.sh
```

如果新版本因依赖预检而尚未激活，请把 `current` 替换为本次上传的 `releases/<commit>` 目录，安装完成后重新执行激活脚本。

该脚本同时安装常用中文字体 `fonts-noto-cjk`，降低 Word/PPT 转 PDF 后的字体替换和版式偏移。

- `soffice`（LibreOffice headless）：把老师上传的 PPT/Word 转成 PDF 预览。
  ```bash
  apt-get install -y libreoffice
  ```
- `gs`（Ghostscript）：上传后把预览 PDF 逐页转换为图片。学生专属水印由小程序覆盖显示；平台防截屏能力不可用时不阻止打开，只保留水印与安全提示。
  ```bash
  apt-get install -y ghostscript
  ```

再创建 `/etc/starline/learning-api.env`，写入生产环境变量：

```bash
APP_ENV=production
FILE_STORAGE_ROOT=/opt/starline/data/uploads
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
- 两份 Nginx 配置都已设置 `client_max_body_size 50m`，与后端课件上传上限一致；轮播图接口仍会限制单张图片不超过 5MB。每次发布还会定位当前生效的网关站点配置，并写入同样的上限。
