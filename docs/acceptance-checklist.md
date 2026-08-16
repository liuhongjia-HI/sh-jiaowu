# Starline 商用验收清单

## 1. 管理后台登录与基础访问

- 打开 `https://sa.starlineeducation.com.cn/login`，页面正常加载，无白屏。
- 使用生产超级管理员账号登录后进入 `今日待办`，浏览器 Network 无 `/api` 失败请求。
- 登录接口 `POST https://gate.starlineeducation.com.cn/api/auth/admin-password-login` 返回 `{ code: 0, data.token }`。
- 登录后 `GET /api/auth/me` 返回当前用户和角色，超级管理员包含 `super_admin`。
- 退出登录后再次访问 `/dashboard` 会回到登录页。

复现步骤：
1. Chrome 无痕窗口打开后台登录页。
2. 输入生产账号和密码。
3. 打开 DevTools Network，点击 `进入工作台`。
4. 检查登录请求状态码、响应体和跳转结果。

整改建议：
- 若请求为 `502`：优先查 `systemctl status starline-api`、`journalctl -u starline-api`、`curl http://127.0.0.1:8892/api/health`。
- 若浏览器 CORS 报错：查 `OPTIONS /api/auth/admin-password-login` 是否返回 `204` 和 `Access-Control-Allow-Origin: https://sa.starlineeducation.com.cn`。
- 若为 `401`：核对账号密码、账号状态、验证码/登录保护状态。

## 2. 线上接口健康与网关

- `GET https://gate.starlineeducation.com.cn/api/health` 返回 200。
- `OPTIONS https://gate.starlineeducation.com.cn/api/auth/admin-password-login` 返回 204。
- Nginx 配置检测 `nginx -t` 通过。
- `starline-api` 服务为 `active (running)`。
- 生产环境变量满足启动条件：`APP_ENV=production`、`ADMIN_PASSWORD_LOGIN_ENABLED=true`、`DEMO_SEED_DATA=false`、`DEMO_STUDENT_LOGIN_ENABLED=false`。

复现步骤：
1. 本地执行 `curl -i https://gate.starlineeducation.com.cn/api/health`。
2. SSH 到服务器执行 `systemctl status starline-api --no-pager`。
3. SSH 到服务器执行 `journalctl -u starline-api -n 80 --no-pager`。

整改建议：
- 服务反复重启时，先修配置校验或缺失环境变量，再重启服务。
- 网关 502 时，先确认本机 `127.0.0.1:8892` 是否可访问，再查 Nginx upstream。

## 3. 管理后台核心功能页

- 超级管理员可打开：`/dashboard`、`/students`、`/permissions`、`/content`、`/scheduling`、`/homework`、`/review`、`/commercial`、`/settings`。
- 页面标题正确显示，接口无失败请求。
- 教师账号不能进入高权限运营/系统页面。
- 页面刷新后登录态保持，token 失效时能回到登录页。

复现步骤：
1. 登录后台。
2. 逐个打开上述页面。
3. 检查页面标题和 Network 失败请求。

整改建议：
- 页面能打开但数据为空：查对应接口数据和角色权限。
- 页面跳登录：查 token 存储、`/api/auth/me`、服务端 token secret 是否变更。

## 4. 排课工作台

- `/scheduling` 首屏显示 `周排班工作台`。
- `课表视图` 默认显示 Outlook-like 周时间轴，即使没有课程也显示 7 天 × 时间刻度。
- 支持上一周、下一周、今天、小月历选周。
- 左侧显示学科日历和筛选项。
- 有正式课程时，课程块按时间定位；可上课时间作为辅助条，不压缩正式课程。
- 无课程时显示可新建入口和轻提示。
- 页面不出现 `教室`、`地点`、`进教室`、`线上会议室` 等文案。

复现步骤：
1. 登录后台后打开 `/scheduling`。
2. 查看是否存在 `.schedule-timeline-grid`。
3. 点击空时间段的 `新建课程`，确认弹出新建课程抽屉。
4. 全页搜索教室/地点相关文案。

整改建议：
- 时间轴不显示：确认空数据时也走时间轴渲染，而不是整页空状态。
- 课程块重叠：检查 `layoutOverlappingItems` 和时间字段格式。
- 出现教室文案：检查排课表单、列表列和课程块渲染。

## 5. 小程序接口与学生端闭环

- 小程序 `develop`、`trial`、`release` 均使用 `https://gate.starlineeducation.com.cn/api`。
- 小程序启动调用真实 `wx.login()`，再请求 `/auth/wechat-login` 换 token。
- 订阅消息模板 ID 配置为 `vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0`。
- 学生端首页、课表、学习、答题、提交、结果、我的页面接口路径均通过线上 `gate` 域名。
- 学生课表页不展示教室/地点/进教室。

复现步骤：
1. 微信开发者工具打开小程序。
2. 清缓存后重新编译。
3. Network 检查请求域名是否为 `https://gate.starlineeducation.com.cn/api`。
4. 使用真实微信登录授权，确认 `/auth/wechat-login` 成功返回 token。
5. 打开课表页，确认正式课表或空状态展示正常。

整改建议：
- 如果开发版仍请求本地地址：检查 `miniprogram/app.js` 的 `resolveApiBaseUrl()`。
- 如果微信登录返回 `40029 invalid code`：说明接口通了，但 code 非真实或已过期，需要从开发者工具重新触发 `wx.login()`。
- 如果普通接口提示域名不合法：配置 request 合法域名 `https://gate.starlineeducation.com.cn`。
- 如果课件详情能打开但分页图片/PDF下载失败：配置 downloadFile 合法域名 `https://gate.starlineeducation.com.cn`。

## 6. 教学业务闭环

- 小程序端学生可补充资料：姓名、年级、学校、家长联系方式等必要信息。
- 管理后台可创建学生、开通套餐、分配学习权限。
- 教师可进入题库新增题目。
- 教师可手动组卷并发布课后练习。
- 学生可进入学习内容和课后练习答题。
- 学生提交后，后台可看到待批改记录。
- 教师可评分、写评语、发放奖励。
- 学生端可查看提交结果、成长分数和反馈。

复现步骤：
1. 后台创建/确认学生。
2. 开通套餐并检查学习权限。
3. 教师新增题目并组卷发布。
4. 小程序学生登录后完成答题。
5. 后台教师批改。
6. 小程序学生查看结果。

整改建议：
- 权限缺失：查套餐、学习空间、学生授权三张关系。
- 学生看不到作业：查作业发布范围、截止时间、学生权限。
- 批改无记录：查提交接口和 submission 状态流转。

## 7. 发布与回滚

- 本地测试通过后构建 release。
- 上传到 `/opt/starline/releases/<release-id>`。
- 执行 `deploy/production/activate-release.sh /opt/starline <release-id>`。
- 激活后检查 `/api/health`、`starline-api` 状态、Nginx 状态。
- 保留最近 release，可通过重新激活上一个 release 回滚。

复现步骤：
1. 构建后端 Linux 二进制和 `web/dist`。
2. `rsync` 上传 release。
3. SSH 执行激活脚本。
4. 执行线上 smoke test。

整改建议：
- 激活失败：查看脚本输出和 `journalctl -u starline-api`。
- 只有前端异常：确认 `/opt/starline/current/web/dist/index.html` 引用的资产存在。
- 只有接口异常：确认环境变量和 MySQL 连通性。

## 当前验收结果（2026-07-09）

- 线上 API 502：已复现，根因为生产配置校验要求公众号配置导致 `starline-api` 启动退出。
- 整改：公众号模板消息配置改为可选；缺失时通知进入待配置，不阻断 API 启动。
- 整改：服务器已补 `WECHAT_MINIPROGRAM_SUBSCRIBE_TEMPLATE_IDS`。
- 整改：小程序开发版也切到线上 `gate` 接口。
- 发布：已通过 SSH 激活 release `codex-20260709121312`。
- 线上验证：后台登录、核心页面、排课时间轴、CORS、健康检查均通过。
