# 年级体验试用任务清单

## 任务 1：定义体验领域模型与内存状态

**描述：** 增加套餐的体验开关、体验记录和学生端体验摘要；在内存存储中实现按学生、学年唯一的状态读取和日期计算，为后续接口提供单一权威逻辑。

**验收标准：**

- [ ] 未领取、有活跃体验、临期、到期、已转正、无可用套餐都能得到确定的体验状态。
- [ ] 同一 `studentId + academicYear` 的体验记录不能重复创建；结束日按包含当天的 7 个自然日计算。
- [ ] 体验资格按学生而非家长账号、手机号或 OpenID 判定。

**验证：**

- [ ] 测试通过：`cd learning-api && go test ./internal/infrastructure/store -run Trial`
- [ ] 人工检查：构造多家长关联同一学生，确认第二个家长读取同一条体验状态。

**依赖：** 无。

**可能涉及文件：**

- `learning-api/internal/domain/learning/grant_entity.go`
- `learning-api/internal/domain/learning/student_entity.go`
- `learning-api/internal/infrastructure/store/memory.go`
- `learning-api/internal/infrastructure/store/memory_student_portal.go`
- `learning-api/internal/infrastructure/store/memory_trial_test.go`

**预计范围：** 中（3–5 个文件）。

## 任务 2：持久化体验记录与套餐体验开关

**描述：** 让 MySQL 初始化、升级、加载、快照和增量同步都支持体验记录与套餐开关，确保重启后资格、剩余期限和转正归因不丢失。

**验收标准：**

- [ ] 新数据库可创建 `student_trial_records`，已有数据库可无损升级。
- [ ] MySQL 重启/重新加载后体验记录与套餐试用开关保持一致。
- [ ] `student_id + academic_year` 唯一约束在数据库层生效。

**验证：**

- [ ] 测试通过：`cd learning-api && go test ./internal/infrastructure/store -run 'MySQL|Trial'`
- [ ] 测试通过：`cd learning-api && go test ./internal/infrastructure/store -run Schema`

**依赖：** 任务 1。

**可能涉及文件：**

- `learning-api/deploy/mysql/init.sql`
- `learning-api/internal/infrastructure/store/mysql_schema.go`
- `learning-api/internal/infrastructure/store/mysql_load.go`
- `learning-api/internal/infrastructure/store/mysql_snapshot.go`
- `learning-api/internal/infrastructure/store/mysql_delta_trial.go`

**预计范围：** 中（3–5 个文件）。

## 任务 3：管理端配置可体验套餐

**描述：** 在课程方案中增加“可用于学生体验”开关及后端校验；套餐只有真实的课程、练习且处于启用状态时才能打开开关。

**验收标准：**

- [ ] 教务能创建、编辑并在列表中识别可体验套餐。
- [ ] 缺少课程或练习的套餐不能被标记为可体验，并显示具体原因。
- [ ] 关闭体验开关后，新学生不能领取；已开始体验不受影响。

**验证：**

- [ ] 测试通过：`cd learning-api && go test ./internal/infrastructure/store -run 'Package|Trial'`
- [ ] 测试通过：`cd web && npm run build`
- [ ] 手工检查：五年级有多个学科体验套餐时，均可配置但不会创建跨学科套餐。

**依赖：** 任务 1、任务 2。

**可能涉及文件：**

- `learning-api/internal/infrastructure/store/memory_management.go`
- `learning-api/internal/interfaces/http/handler/handler_grant.go`
- `web/src/types/starline.ts`
- `web/src/pages/resources/PackagesPage.tsx`
- `web/src/pages/resources/ResourceDialogs.tsx`

**预计范围：** 中（3–5 个文件）。

## 检查点：体验资格与套餐配置

- [ ] 后端存储测试和管理端构建均通过。
- [ ] 至少为一个年级配置了包含课程和练习的体验套餐。
- [ ] 确认套餐选择规则：一个候选直接开始，多个候选由学生在领取后选择学科。

## 任务 4：提供学生领取体验接口

**描述：** 扩展学生首页响应，并新增幂等的体验开始接口。开始体验时在同一事务内写记录和套餐授权，返回体验状态和首门课程入口。

**验收标准：**

- [ ] `GET /api/student/home` 返回契约中的 `trial` 字段，不影响原有字段。
- [ ] `POST /api/student/trial/start` 仅允许当前学生领取同年级的可体验套餐。
- [ ] 连续请求、并发请求和网络重试最多创建一条体验记录与一条授权。

**验证：**

- [ ] 测试通过：`cd learning-api && go test ./internal/infrastructure/store -run Trial`
- [ ] 测试通过：`cd learning-api && go test ./internal/interfaces/http/router -run Trial`
- [ ] 手工检查：体验开始后首门课程、练习和结果反馈都能访问；到期后均不可继续访问。

**依赖：** 任务 1、任务 2、任务 3。

**可能涉及文件：**

- `learning-api/internal/application/learningapp/service_student.go`
- `learning-api/internal/interfaces/http/handler/handler_student.go`
- `learning-api/internal/interfaces/http/router/routes.go`
- `learning-api/internal/infrastructure/store/memory_student_portal.go`
- `learning-api/internal/interfaces/http/router/router_test.go`

**预计范围：** 中（3–5 个文件）。

## 任务 5：完成小程序领取与体验中流程

**描述：** 在首页按体验状态展示领取、套餐选择、确认、倒计时和继续学习入口；领取成功后进入首门课程，并让学习页使用体验状态而非仅判断已开通套餐数量。

**验收标准：**

- [ ] 登录后的未开通学生能看到明确的免费体验卡片和“不自动扣费”说明。
- [ ] 多候选套餐时，学生在领取后选择一个学科；选择不增加登录表单字段。
- [ ] 体验中和临期能显示准确的剩余天数；页面加载、失败和空候选均有正常反馈。

**验证：**

- [ ] 测试通过：`node --test miniprogram/tests/home.test.js miniprogram/tests/study.test.js`
- [ ] 手工检查：在微信开发者工具中完成“登录 → 领取 → 课程 → 练习 → 返回首页”流程。

**依赖：** 任务 4。

**可能涉及文件：**

- `miniprogram/pages/home/index.js`
- `miniprogram/pages/home/index.wxml`
- `miniprogram/pages/home/index.wxss`
- `miniprogram/pages/study/index.js`
- `miniprogram/tests/home.test.js`

**预计范围：** 中（3–5 个文件）。

## 任务 6：提供教务试用转正与到期保护

**描述：** 在学生详情展示体验历史与“试用转正”动作；将正式套餐开通和体验记录更新放入同一事务，并完善到期后首页、学习页和受保护内容的提示。

**验收标准：**

- [ ] 只有教务角色可执行试用转正，学生端没有伪支付入口。
- [ ] 转正可选择同年级正式套餐；同套餐延长原授权，不同套餐可并存。
- [ ] 未经过转正动作的人工套餐开通不会改变体验记录状态。

**验证：**

- [ ] 测试通过：`cd learning-api && go test ./internal/infrastructure/store -run 'Trial|Grant'`
- [ ] 测试通过：`cd learning-api && go test ./internal/interfaces/http/router -run 'Trial|Grant'`
- [ ] 手工检查：体验到期后历史结果可看、受保护内容不可继续打开；转正后可恢复正式内容。

**依赖：** 任务 4、任务 5。

**可能涉及文件：**

- `learning-api/internal/application/learningapp/service_grant.go`
- `learning-api/internal/interfaces/http/handler/handler_grant.go`
- `learning-api/internal/interfaces/http/router/routes.go`
- `web/src/pages/Students.tsx`
- `learning-api/internal/infrastructure/store/memory_trial_test.go`

**预计范围：** 中（3–5 个文件）。

## 检查点：完整学生与教务闭环

- [ ] 领取、继续学习、到期、转正四种状态均有用户可理解的页面。
- [ ] API 路由权限、体验唯一性与正式套餐访问回归测试全部通过。
- [ ] 小程序端无空白页、无限加载或重复弹窗。

## 任务 7：回归测试与上线前验收

**描述：** 覆盖多家长、多子女、年级不匹配、无体验套餐、体验到期、重复请求和转正失败回滚等边界；执行项目全量构建与相关端到端检查。

**验收标准：**

- [ ] 新增用例覆盖体验资格、日期边界、幂等领取、到期访问控制和显式转正。
- [ ] 原有正式套餐开通、推荐套餐、学生切换和登录绑定不回归。
- [ ] 所有用户文案不暗示自动扣费或小程序内支付。

**验证：**

- [ ] 测试通过：`cd learning-api && go test ./...`
- [ ] 测试通过：`cd web && npm run build`
- [ ] 测试通过：`node --test miniprogram/tests/*.test.js`
- [ ] 手工检查：使用真实年级体验套餐完成完整闭环。

**依赖：** 任务 1–6。

**可能涉及文件：**

- `learning-api/internal/infrastructure/store/memory_trial_test.go`
- `learning-api/internal/interfaces/http/router/router_test.go`
- `miniprogram/tests/home.test.js`
- `miniprogram/tests/study.test.js`
- `web/scripts/admin-smoke.mjs`

**预计范围：** 中（3–5 个文件）。

## 最终检查点

- [ ] 所有测试、构建和手工体验流程通过。
- [ ] 教务已准备体验套餐与学生开通话术。
- [ ] 已由业务负责人确认 7 天、每学年一次和显式转正规则。
