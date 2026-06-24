# Starline API

统一响应：

```json
{ "code": 0, "message": "ok", "data": {} }
```

## 管理端

- `GET /api/health`
- `POST /api/auth/wechat-login`
- `POST /api/auth/admin-password-login`
- `POST /api/auth/demo-student-login`
- `GET /api/auth/captcha`
- `GET /api/auth/me`
- `POST /api/auth/change-password`
- `POST /api/auth/refresh`
- `POST /api/auth/logout`
- `GET /api/dashboard/overview`
- `GET /api/packages`
- `POST /api/packages`
- `PUT /api/packages/{id}`
- `GET /api/learning-spaces`
- `GET /api/students`
- `GET /api/students/{id}`
- `POST /api/students`
- `PUT /api/students/{id}`
- `POST /api/students/{id}/remind`
- `POST /api/students/import`
- `GET /api/students/{id}/grants`
- `GET /api/students/{id}/learning-records`
- `GET /api/commercial/summary`
- `GET /api/commercial/orders`
- `POST /api/commercial/orders`
- `POST /api/commercial/orders/{id}/payments`
- `POST /api/commercial/orders/{id}/refunds`
- `POST /api/commercial/orders/{id}/contracts`
- `POST /api/commercial/orders/{id}/invoices`
- `POST /api/commercial/lesson-consumptions`
- `POST /api/commercial/renewal-reminders`
- `POST /api/commercial/parent-notices`
- `GET /api/courses`
- `POST /api/courses`
- `PUT /api/courses/{id}`
- `GET /api/materials`
- `POST /api/materials`
- `PUT /api/materials/{id}`
- `GET /api/homework`
- `POST /api/homework`
- `PUT /api/homework/{id}`
- `GET /api/files/{id}/preview`
- `GET /api/files/{id}/download`
- `GET /api/reviews/pending`
- `POST /api/reviews/{id}/complete`
- `GET /api/notices`
- `POST /api/notices`
- `GET /api/availability/overview`
- `GET /api/availability`
- `PUT /api/availability`
- `POST /api/scheduling/candidates`
- `GET /api/schedule-classes`
- `POST /api/schedule-classes`
- `PUT /api/schedule-classes/{id}`
- `POST /api/schedule-classes/{id}/cancel`
- `GET /api/logs`
- `GET /api/settings`
- `PUT /api/settings`
- `GET /api/admin-staff`
- `POST /api/admin-staff`
- `PUT /api/admin-staff/{id}`
- `GET /api/teachers`
- `POST /api/teachers`
- `PUT /api/teachers/{id}`
- `POST /api/teachers/{id}/reset-password`
- `POST /api/admin-staff/{id}/reset-password`
- `GET /api/permissions/students`
- `GET /api/permissions/packages`
- `GET /api/permissions/content`
- `GET /api/grants/preview?studentId=stu-001&packageId=pkg-g05-english-s1-full`
- `POST /api/grants`

### 管理后台登录

`POST /api/auth/admin-password-login`

```json
{
  "phone": "13800000001",
  "password": "123456",
  "captchaId": "",
  "captchaAnswer": ""
}
```

仅教师、运营教务、校区管理员、超级管理员可登录管理后台。学生账号不能通过该接口登录。

登录失败会进入频率保护和失败锁定；同一账号连续失败后，后台密码登录需要先调用 `GET /api/auth/captcha` 获取验证码，并在登录请求中带上 `captchaId` 和 `captchaAnswer`。失败、锁定、验证码失败、登出、改密、重置密码都会写入操作日志，日志包含操作者 ID、IP、User-Agent 和安全事件详情。

首次创建或被重置密码的后台账号，登录返回的 `user.mustChangePassword=true`。此状态下只能访问 `GET /api/auth/me`、`POST /api/auth/change-password`、`POST /api/auth/logout`，其他后台接口返回“请先修改初始密码”。

`POST /api/auth/change-password`

```json
{
  "oldPassword": "旧密码",
  "newPassword": "Teacher2026"
}
```

新密码至少 8 位，并同时包含字母和数字。修改成功后账号 `tokenVersion` 会变化，旧 token 立即失效，需要重新登录。

`POST /api/auth/refresh` 会签发新 token 并作废当前 token，用于主动轮换登录态。

`POST /api/auth/logout` 会作废当前 token。

### 学习套餐

教师可查看套餐；运营教务、校区管理员、超级管理员可创建和编辑套餐。套餐绑定学习空间和开放内容类型，学生开通后按这些关系派生课程、资料和小挑战权限。

`POST /api/packages` / `PUT /api/packages/{id}` 请求体：

```json
{
  "name": "2025.2026学年 五年级 S1 英语 题+讲义",
  "academicYear": "2025.2026学年",
  "grade": "五年级",
  "semester": "S1",
  "subject": "英语",
  "phaseScope": "全学期",
  "packageType": "题+讲义",
  "summary": "开放 S1 Q1 和 S1 Q2 英语练习与讲义。",
  "learningSpaceIds": ["space-g05-english-s1-q1", "space-g05-english-s1-q2"],
  "contentTypeCodes": ["question", "handout"],
  "status": "启用"
}
```

`contentTypeCodes` 支持：

- `course`：课程
- `question`：题
- `handout`：讲义

编辑已开通套餐后，系统会同步刷新该套餐对应学生的学习空间访问权限。

### 排课

师生分别填报可上课时间（`GET/PUT /api/availability`，`ownerType` 为 `teacher` 或 `student`），教务按「学科 + 年级」协调成班。

`POST /api/scheduling/candidates` 按学科 + 年级查找可排时间，请求体：

```json
{
  "subject": "英语",
  "grade": "五年级",
  "classType": "1V3",
  "durationMinutes": 90,
  "startDate": "2026-06-01",
  "endDate": "2026-08-31"
}
```

- 系统只把**同年级 + 已开通同学科**的学生凑在一起，并匹配授课范围覆盖该学科年级的老师。
- 返回的每个候选含 `availableStudents`（该时段可上的学生）和 `missingStudents`（同学科同年级但该时段没空的学生），供「协调建议」面板提示教务协调时间。
- 兼容旧入口：仅传 `courseId` + `teacherId` 时按单课程单老师查找。

`POST /api/schedule-classes` 确认成班时同样校验「同年级同学科」，跨年级或未开通该学科的学生会被拒绝。

### 课程内容

教师可在自己负责的学习空间内创建和编辑课程；运营教务、校区管理员、超级管理员可维护全部课程。课程必须绑定一个学习空间，年级和学科由学习空间自动派生。

`POST /api/courses` / `PUT /api/courses/{id}` 请求体：

```json
{
  "name": "五年级英语 S1 Q1 阅读课程",
  "learningSpaceId": "space-g05-english-s1-q1",
  "chapterCount": 8,
  "status": "启用"
}
```

编辑课程名称或学习空间后，系统会同步课程下已上传资料和练习的课程名称与学习空间范围。

### 学习资料与课后练习

教师可维护自己负责课程下的讲义和题目；运营教务、校区管理员、超级管理员可维护全部内容。上传接口使用 `multipart/form-data`，编辑接口只维护标题、课程范围、章节/截止时间和发布状态，不替换原文件。

`PUT /api/materials/{id}` 请求体：

```json
{
  "title": "五年级英语期中核心讲义",
  "courseId": "course-g05-english-s1-q1",
  "learningSpaceId": "space-g05-english-s1-q1",
  "chapter": "第一章",
  "status": "启用"
}
```

`PUT /api/homework/{id}` 请求体：

```json
{
  "title": "五年级英语阅读练习题",
  "courseId": "course-g05-english-s1-q1",
  "learningSpaceId": "space-g05-english-s1-q1",
  "deadline": "2026-06-30",
  "status": "启用"
}
```

`status` 支持 `启用`、`草稿`、`停用`。学生端只展示已启用内容；草稿和停用内容保留在后台，便于老师继续维护。

### 系统设置

校区管理员、超级管理员可查看和修改系统设置。

`PUT /api/settings` 请求体：

```json
{
  "key": "downloadPolicy",
  "value": "允许下载已发布讲义"
}
```

可维护的 `key` 包括：`academicYear`、`grades`、`semesters`、`watermarkRule`、`downloadPolicy`。成功后返回完整设置对象，并记录操作日志。

### 教师管理

仅 `campus_admin`、`super_admin` 可访问。校区管理员只能查看和维护自己校区的教师。

`GET /api/teachers` 返回教师账号视图：

```json
{
  "id": "user-teacher",
  "name": "英语老师",
  "phone": "13800000004",
  "campusId": "campus-main",
  "learningSpaceIds": ["space-g05-english-s1-q1", "space-g05-english-s1-q2"],
  "learningSpaces": ["五年级英语 S1 Q1", "五年级英语 S1 Q2"],
  "grades": ["五年级"],
  "subjects": ["英语"],
  "canUploadHandout": true,
  "canUploadQuestion": true,
  "canReview": true,
  "accountStatus": "正常",
  "bindStatus": "已绑定",
  "remark": ""
}
```

`POST /api/teachers` / `PUT /api/teachers/{id}` 请求体：

```json
{
  "name": "英语老师",
  "phone": "13800000004",
  "learningSpaceIds": ["space-g05-english-s1-q1", "space-g05-english-s1-q2"],
  "canUploadHandout": true,
  "canUploadQuestion": true,
  "canReview": true,
  "accountStatus": "正常",
  "remark": "负责五年级英语"
}
```

新增教师固定写入 `teacher` 角色，默认 `accountStatus=正常`、`bindStatus=待绑定`，前端不提供角色选择。

`POST /api/teachers/{id}/reset-password` 可由校区管理员或超级管理员重置教师密码。返回一次性临时密码，并要求教师下次登录后立即修改。

`POST /api/admin-staff/{id}/reset-password` 仅超级管理员可用，用于重置后台管理人员密码。重置后旧 token 失效，账号下次登录必须改密。

### 批改反馈

教师需拥有 `canReview=true`；运营教务、校区管理员、超级管理员可直接批改。

`POST /api/reviews/{id}/complete` 请求体：

```json
{
  "score": 95,
  "teacherComment": "阅读依据找得很准，继续保持。",
  "reward": "阅读小星星"
}
```

提交后会生成学生可见的批改结果，从待批改列表移除，并自动生成批改完成提醒。

### 通知提醒

教师、运营教务、校区管理员、超级管理员可查看通知。教师发送通知时，接收对象、标题或内容需包含自己负责的学科；运营教务和管理员可发送到任意对象或 `全部学生`。

`POST /api/notices` 请求体：

```json
{
  "type": "练",
  "title": "英语阅读挑战已发布",
  "target": "五年级英语班",
  "summary": "今天完成 S1 Q1 阅读挑战。"
}
```

学生端只返回和当前学生相关的通知：匹配学生姓名、年级、已开通套餐、可学学科，或目标为 `全部学生` 的通知。

### 学生管理

教师可按负责课程和班级查看学生；运营教务、校区管理员、超级管理员可新增、编辑、导入、提醒和开通套餐。

`POST /api/students` / `PUT /api/students/{id}` 请求体：

```json
{
  "name": "小明",
  "phone": "18500009069",
  "grade": "五年级",
  "accountStatus": "正常",
  "remark": "家长周末可联系"
}
```

`POST /api/students/import` 使用 `multipart/form-data` 上传 `file`，CSV 字段顺序为 `name, phone, grade, remark`。

学生账号不物理删除，停用后学生端接口会返回账号停用错误。

### 开通套餐

运营教务、校区管理员、超级管理员可给学生开通套餐。提交前先调用预览接口，确认本次会开放的课程、资料和练习。

`GET /api/grants/preview?studentId=stu-001&packageId=pkg-g05-english-s1-full` 返回：

```json
{
  "studentId": "stu-001",
  "packageId": "pkg-g05-english-s1-full",
  "studentName": "小明",
  "packageName": "2025.2026学年 五年级 S1 英语 题+讲义",
  "alreadyOpened": true,
  "existingUntil": "2027-05-22",
  "learningSpaces": ["五年级英语 S1 Q1"],
  "contentTypes": ["题", "讲义"],
  "openCourses": [],
  "openMaterials": ["五年级英语 S1 Q1 核心讲义"],
  "openHomework": ["五年级英语 S1 Q1 练习题"],
  "blockedContent": ["课程"],
  "effectiveDefault": "今天起 365 天"
}
```

`alreadyOpened=true` 表示学生已有生效中的同套餐权限，前端会提示当前有效期并避免重复提交。`POST /api/grants` 请求体为 `{ "studentId": "stu-001", "packageId": "pkg-g05-english-s1-full" }`，成功后同步刷新学生学习权限。

### 商业订单与课消

运营教务、校区管理员、超级管理员可维护商业闭环。订单收款确认到全额后，会自动同步开通订单绑定的学习套餐，避免“已收费但未开通权限”。

`POST /api/commercial/orders`

```json
{
  "studentId": "stu-001",
  "packageId": "pkg-g05-english-s1-full",
  "amountCent": 128000,
  "lessonTotal": 10,
  "remark": "暑期英语专项"
}
```

`POST /api/commercial/orders/{id}/payments`

```json
{
  "amountCent": 128000,
  "method": "微信支付",
  "transactionNo": "wx-202606190001"
}
```

`POST /api/commercial/orders/{id}/refunds`

```json
{
  "amountCent": 20000,
  "reason": "家长申请退部分课时"
}
```

`POST /api/commercial/orders/{id}/contracts` 记录合同签署；`POST /api/commercial/orders/{id}/invoices` 记录开票；`POST /api/commercial/lesson-consumptions` 登记课消，超过订单剩余课时会被拒绝；`POST /api/commercial/renewal-reminders` 创建续费跟进；`POST /api/commercial/parent-notices` 给家长发送订单相关通知，并同步到学生通知列表。

## 学生端

登录：小程序调用 `wx.login()` 获取 code 上送。配置环境变量 `WECHAT_APPID`、`WECHAT_SECRET` 后，后端通过 `jscode2session` 用真实 code 换取 openId；未配置时走演示映射（code 即 openId 后缀，如 `student`），保证本地无凭据可用。

- `POST /api/auth/wechat-login`
- `POST /api/auth/demo-student-login`：仅本地演示环境可用，学生演示账号使用手机号和统一密码 `123456` 登录。
- `GET /api/student/home`
- `GET /api/student/study`
- `GET /api/student/study/{id}` — 学习详情（课程 + 资料 + 小挑战 + 学习地图站点 + 进度）
- `GET /api/student/materials/{id}` — 资料详情
- `GET /api/student/homework/{id}` — 小挑战题目详情（含 `questions`）
- `GET /api/student/tasks`
- `GET /api/student/notices`
- `GET /api/student/me`
- `GET /api/student/availability` / `PUT /api/student/availability`
- `GET /api/student/schedule`
- `POST /api/student/submissions` — 提交小挑战，自动判分，返回 `{ submissionId, status, score }`
- `GET /api/student/submissions/{id}` — 查看批改结果（分数、评语、奖励）
- `GET /api/student/growth` — 成长轨迹（提交记录 + 已学资料，按时间倒序）
- `GET /api/student/badges` — 徽章墙，`obtained` 由真实学习数据派生
- `GET /api/student/favorites` — 我的收藏列表
- `POST /api/student/favorites` — 收藏内容，请求体 `{ "targetType": "material|homework", "targetId": "mat-xxx" }`
- `DELETE /api/student/favorites/{id}` — 取消收藏

`GET /api/student/study` 返回 `{ courses: [{ ...course, progress }], materials }`，`progress` 为真实学习进度。
`GET /api/student/tasks` 返回任务数组，`studentStatus`（待完成/已完成）、`score`、`submissionId` 由提交记录派生。
`GET /api/student/home` 新增 `continueProgress` 字段。

`POST /api/student/submissions` 请求体：

```json
{
  "homeworkId": "hw-g05-english-s1-q1",
  "answers": [
    { "questionId": "q1", "choice": "A", "text": "" },
    { "questionId": "q2", "choice": "", "text": "今天学会了抓中心句" }
  ]
}
```
