# Starline

Starline 是面向教培业务的学习套餐、课程资料、小挑战和批改反馈系统。

## 项目结构

```text
learning-api/   Go + Gin 后端接口
web/            React + Vite 管理后台
miniprogram/    微信小程序学生端
docs/           原型与项目文档
```

## 本地启动

一键启动后端接口、管理后台和本地依赖：

```bash
./start.sh
```

启动成功后打开 `http://127.0.0.1:5173`。

后端强依赖 MySQL 持久化，`./start.sh` 会先启动 `docker-compose.yml` 中的 MySQL；如果 MySQL 不可用，API 会直接启动失败，不进入内存兜底模式。

管理后台本地账号初始密码均为 `123456`：

- 超级管理员：`13800000001`
- 校区管理员：`13800000002`
- 运营教务：`13800000003`
- 英语老师：`13800000004`

学生小程序使用真实微信登录和手机号授权绑定。微信开发者工具打开 `miniprogram/` 时，`develop` 版默认请求 `http://127.0.0.1:8892/api`，本地闭环验证需给后端配置 `WECHAT_APPID`、`WECHAT_SECRET`，并在后台先维护学生档案和开通套餐。

如果只想手动分步启动：

```bash
docker compose up -d

cd learning-api
go run ./cmd/api

cd ../web
npm install
npm run dev
```

微信小程序使用微信开发者工具打开 `miniprogram/`。

## 生产环境关键开关

`APP_ENV=production` 时，后端会拒绝使用本地默认密钥、默认 MySQL DSN、缺失微信凭据或演示数据开关。上线前至少配置：

```bash
APP_ENV=production
AUTH_TOKEN_SECRET=<高强度随机密钥>
MYSQL_DSN=<生产 MySQL DSN>
WECHAT_APPID=<微信小程序 AppID>
WECHAT_SECRET=<微信小程序 Secret>
WECHAT_OFFICIAL_ACCOUNT_APPID=<微信公众号 AppID>
WECHAT_OFFICIAL_ACCOUNT_SECRET=<微信公众号 Secret>
WECHAT_OFFICIAL_ACCOUNT_TEMPLATE_ID=<课程/练习提醒模板 ID>
WECHAT_MINIPROGRAM_SUBSCRIBE_TEMPLATE_IDS=<小程序订阅消息模板 ID，多个用英文逗号分隔>
DEMO_SEED_DATA=false
DEMO_STUDENT_LOGIN_ENABLED=false
ADMIN_PASSWORD_LOGIN_ENABLED=true
```

当前已申请的小程序订阅消息模板：

- 预约开始提醒：`vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0`
- 模板字段：`thing1` 温馨提醒、`time5` 结束时间、`time4` 开始时间
- 使用场景：学习辅导/课程开始前提醒；学生在小程序首页点击“开启提醒”授权后，后端记录授权状态。

管理后台生产构建默认不展示演示账号；如需内部演示，可显式设置 `VITE_DEMO_ACCOUNTS_ENABLED=true`。小程序各版本均使用真实微信登录；`develop` 版仅把接口地址切到本地，便于微信开发者工具验证。

上线前可在管理后台工作台查看“上线配置检查”。系统会检查生产接口域名、公众号模板消息环境变量、已开通学生公众号 openid 覆盖率，并展示小程序域名备案、微信公众号关联、模板消息审核等需要人工确认的事项。外部平台状态无法由代码直接证明，完成后需在“系统设置”里把对应状态改为“已完成”。

## 数据初始化边界

数据库初始化拆成三类：

- 正式迁移：`learning-api/deploy/mysql/init.sql` 只创建业务表、补基础角色/学科/系统设置，并注册可选演示数据过程。
- 基础字典：后端启动时默认保留系统设置等基础字典；`DEMO_SEED_DATA=false` 不会写入演示学生、套餐、课程和内容。
- 可选演示数据：本地开发可通过 `DEMO_SEED_DATA=true` 由后端初始化；如需纯 SQL 演示数据，先执行 `init.sql`，再用 `utf8mb4` 连接执行 `learning-api/deploy/mysql/demo_seed.sql`。该过程会按当前课程矩阵重建 `space-g%`、`pkg-g%` 等演示数据，适用于本地/演示库，不要直接对生产库执行。
