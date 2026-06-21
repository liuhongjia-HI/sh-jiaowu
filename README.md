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

学生小程序在未发布、未配置微信凭据时可使用演示账号登录：手机号 `18500009069`，密码 `123456`。

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
DEMO_SEED_DATA=false
DEMO_STUDENT_LOGIN_ENABLED=false
ADMIN_PASSWORD_LOGIN_ENABLED=true
```

管理后台生产构建默认不展示演示账号；如需内部演示，可显式设置 `VITE_DEMO_ACCOUNTS_ENABLED=true`。小程序 `develop` 版保留演示登录便利，`trial` / `release` 版默认使用真实微信登录。

## 数据初始化边界

数据库初始化拆成三类：

- 正式迁移：`learning-api/deploy/mysql/init.sql` 只创建业务表、补基础角色/学科/系统设置，并注册可选演示数据过程。
- 基础字典：后端启动时默认保留系统设置等基础字典；`DEMO_SEED_DATA=false` 不会写入演示学生、套餐、课程和内容。
- 可选演示数据：本地开发可通过 `DEMO_SEED_DATA=true` 由后端初始化；如需纯 SQL 演示数据，先执行 `init.sql`，再手动执行 `learning-api/deploy/mysql/demo_seed.sql`。
