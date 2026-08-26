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
- `GET /api/system/readiness`
- `GET /api/packages`
- `POST /api/packages`
- `PUT /api/packages/{id}`
- `GET /api/learning-spaces`
- `GET /api/students`
- `GET /api/students/{id}`
- `POST /api/students`
- `PUT /api/students/{id}`
- `GET /api/students/{id}/scores`
- `POST /api/students/{id}/scores`
- `PUT /api/students/{id}/scores/{scoreId}`
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
  "name": "2025.2026学年 五年级 S1 英语 题+学习资料",
  "academicYear": "2025.2026学年",
  "grade": "五年级",
  "semester": "S1",
  "subject": "英语",
  "phaseScope": "全学期",
  "packageType": "题+学习资料",
  "summary": "开放 S1 Q1 和 S1 Q2 英语练习与学习资料。",
  "learningSpaceIds": ["space-g05-english-s1-q1", "space-g05-english-s1-q2"],
  "contentTypeCodes": ["question", "handout"],
  "status": "启用"
}
```

`contentTypeCodes` 支持：

- `course`：课程
- `question`：题
- `handout`：学习资料

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

`POST /api/schedule-classes` / `PUT /api/schedule-classes/{id}` 确认成班或调课时同样校验「同年级同学科」，跨年级或未开通该学科的学生会被拒绝。填写 `roomName` 后，系统会按 `campusId + roomName + 星期 + 时间段` 检查教室/线上会议室等资源冲突。

```json
{
  "courseId": "course-g05-english-s1-q1",
  "teacherId": "user-teacher",
  "campusId": "campus-main",
  "roomName": "A101",
  "classType": "1V3",
  "durationMinutes": 90,
  "dayOfWeek": 3,
  "startTime": "19:00",
  "endTime": "20:30",
  "startDate": "2026-06-01",
  "endDate": "2026-08-31",
  "studentIds": ["stu-001", "stu-002", "stu-003"]
}
```

返回体在此基础上附带 `academicYear` / `semester`：按 `startDate` 落系统设置里的校历（`GET/PUT /api/settings` 的 `academicCalendar`）判定一次后固定写入，此后调校历、切学年都不会让已排课程的归属跟着变；只有真的改了开课日期或课程才会重新判定。开课日期落不进任何校历学期时（例如假期班），退回课程所属学习空间的学期标签 + 开课日期本身的自然年 7 月 1 日规则，不阻塞建班。

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

教师可维护自己负责课程下的学习资料和题目；运营教务、校区管理员、超级管理员可维护全部内容。上传接口使用 `multipart/form-data`，编辑接口只维护标题、课程范围、章节/截止时间和发布状态，不替换原文件。

`PUT /api/materials/{id}` 请求体：

```json
{
  "title": "五年级英语期中核心学习资料",
  "courseId": "course-g05-english-s1-q1",
  "learningSpaceId": "space-g05-english-s1-q1",
  "chapter": "第一章",
  "status": "已发布"
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

学习资料 `status` 推荐使用 `已发布`、`停用`，学生端只展示已发布内容；后端兼容历史值 `启用` 并按已发布处理。练习仍支持 `启用`、`草稿`、`停用`，草稿和停用内容只保留在后台。

学习资料返回对象会包含 `grade`、`semester`、`subject`，这些字段由绑定的学习空间派生，后台可直接按年级、学期和学科筛选。学习资料本身不带学年——学习空间是跨学年复用的课程目录，同一个五年级英文 S1 阶段的资料每年可能更新但不需要按学年分别建档，见架构文档「学习数据权限」一节。

题库题目的 `stem` 支持轻量富文本，可包含加粗、列表、颜色和图片 URL。学生端会按富文本渲染题干，适合阅读理解、图形题等复杂题型；结构化题型仍建议使用选项和答案字段，便于自动判分。

### 系统设置

校区管理员、超级管理员可查看和修改系统设置。

`PUT /api/settings` 请求体：

```json
{
  "key": "downloadPolicy",
  "value": "允许下载带水印PDF"
}
```

可维护的 `key` 包括：`grades`、`semesters`、`watermarkRule`、`downloadPolicy`、`miniProgramDomainStatus`、`productionApiDomain`、`officialAccountBindingStatus`、`templateMessageStatus`。`downloadPolicy` 只能为“仅在线预览”或“允许下载带水印PDF”，学生从不获取原始文件。当前学年由系统按日期自动判断（每年 7 月 1 日切换），无需维护；套餐开通有效期按“学年校历”自动计算。成功后返回完整设置对象，并记录操作日志。

学科颜色、简称和排序属于学科元数据，不放入系统设置 JSON：`GET /api/subjects` 获取完整列表，校区管理员和超级管理员可通过 `PUT /api/subjects/{id}` 维护 `shortLabel`、`color`、`sortOrder` 与 `status`。学科名称不在此接口修改，避免破坏已有课程、学习空间和历史数据的关联。

`GET /api/system/readiness` 返回上线配置检查结果：

```json
{
  "readyCount": 2,
  "totalCount": 6,
  "items": [
    {
      "key": "productionApiDomain",
      "title": "生产接口域名",
      "status": "ready",
      "message": "已配置生产接口域名：https://api.starlineeducation.com.cn"
    }
  ]
}
```

`status` 支持 `ready`、`warning`、`missing`。小程序域名备案、公众号关联、模板消息审核属于外部平台事项，系统只读取设置项中的人工确认结果；公众号发送配置和学生 openid 覆盖率由后端根据当前配置和学生档案计算。

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
  "reward": "阅读小星星",
  "finalStatus": "已批改"
}
```

`finalStatus` 可选 `待复核`、`已批改`，不传时默认 `已批改`。提交为 `待复核` 时会保留在批改看板的复核栏，学生端可查看当前反馈；提交为 `已批改` 时从待处理列表移除，并自动生成批改完成提醒。

### 通知提醒

教师、运营教务、校区管理员、超级管理员可查看通知。教师发送通知时，接收对象、标题或内容需包含自己负责的学科；运营教务和管理员可发送到任意对象或 `全部学生`。

`POST /api/notices` 请求体：

```json
{
  "type": "练",
  "title": "英语阅读挑战已发布",
  "target": "五年级英语班",
  "summary": "今天完成 S1 Q1 阅读挑战。",
  "channel": "站内通知",
  "relatedType": "homework",
  "relatedId": "hw-xxx"
}
```

`channel` 支持 `站内通知` 和 `公众号模板消息`。使用 `公众号模板消息` 时需额外传 `recipientOpenId`，并配置：

- `WECHAT_OFFICIAL_ACCOUNT_APPID`
- `WECHAT_OFFICIAL_ACCOUNT_SECRET`
- `WECHAT_OFFICIAL_ACCOUNT_TEMPLATE_ID`

若请求体未传 `recipientOpenId`，且 `target` 精确匹配学生姓名或手机号，后端会自动使用学生档案里的 `officialAccountOpenId`。公众号配置或接收人 openid 缺失时，通知会保存为 `待配置` 并记录 `failureReason`；发送失败会保存为 `发送失败`。可调用 `POST /api/notices/{id}/retry` 补发，补发次数记录在 `retryCount`。

使用 `公众号模板消息` 时，后端会同时生成一条同内容的 `站内通知` 作为学生端历史记录。公众号模板消息记录用于后台查看发送状态、失败原因和补发次数；站内通知记录用于学生端查看消息历史，且不参与补发。

创建已启用练习，或把草稿/停用练习改为启用时，系统会按“应提交学生”逐个生成 `公众号模板消息` 通知记录，`relatedType=homework`，`relatedId` 为练习 ID；缺少公众号配置或学生 openid 时同样保存为可补发记录。

确认排课、调整课程和取消课程时，系统会按课程学生逐个生成 `公众号模板消息` 通知记录，`relatedType=schedule`，`relatedId` 为排课 ID；通知内容包含课程、上课日、时间、老师和教室信息，缺少公众号配置或学生 openid 时同样保存为可补发记录。

学生端只返回和当前学生相关且已真正发送的站内通知：匹配学生姓名、年级、已开通套餐、可学学科，或目标为 `全部学生` 的通知。`公众号模板消息` 原始记录、`待配置`、`发送失败` 的通知只在后台保留，供教务查看失败原因和补发；补发公众号消息时，如果缺少对应站内通知，后端会自动补齐站内历史。

### 学生管理

教师可按负责课程和班级查看学生；运营教务、校区管理员、超级管理员可新增、编辑、导入、提醒和开通套餐。

`POST /api/students` / `PUT /api/students/{id}` 请求体：

```json
{
  "name": "小明",
  "phone": "18500009069",
  "grade": "五年级",
  "schoolName": "星河小学",
  "officialAccountOpenId": "oa-openid-001",
  "accountStatus": "正常",
  "remark": "家长周末可联系"
}
```

`POST /api/students/import` 使用 `multipart/form-data` 上传 `file`，CSV 字段顺序为 `name, phone, grade, schoolName, remark, officialAccountOpenId`。

学生账号不物理删除，停用后学生端接口会返回账号停用错误。

学生列表和学生详情会返回 `lastStudyAt`、`lastSubmittedAt`、`lastSubmissionStatus`，用于教务同时判断最近学习时间和最近一次练习提交/批改状态。

### 成绩对比

教师、运营教务、校区管理员、超级管理员可在可见学生范围内录入成绩。教师还必须负责该学生年级对应的学科范围。

`POST /api/students/{id}/scores` / `PUT /api/students/{id}/scores/{scoreId}` 请求体：

```json
{
  "subject": "数学",
  "examType": "期中",
  "examName": "入学测评",
  "examDate": "2026-06-24",
  "score": 86,
  "fullScore": 100,
  "averageScore": 78,
  "teacherComment": "继续巩固计算准确率。"
}
```

`examType` 支持 `期中`、`期末`、`单元测`、`模拟考`、`阶段测评`；未传时默认 `阶段测评`。

`GET /api/students/{id}/scores` 按学科返回首次成绩、最近成绩、提升说明、`problemPoint` 和 `nextStep`，并按各学科最近考试日期倒序排列；最近成绩也会进入学生成长轨迹。学生端成绩页会把考试成绩和平时练习分区展示，考试成绩显示趋势、问题点和下一步建议。

### 开通套餐

运营教务、校区管理员、超级管理员可给学生开通套餐。提交前先调用预览接口，确认本次会开放的课程、资料和练习。

`GET /api/grants/preview?studentId=stu-001&packageId=pkg-g05-english-s1-full` 返回：

```json
{
  "studentId": "stu-001",
  "packageId": "pkg-g05-english-s1-full",
  "studentName": "小明",
  "packageName": "2025.2026学年 五年级 S1 英语 题+学习资料",
  "alreadyOpened": true,
  "existingUntil": "2027-05-22",
  "learningSpaces": ["五年级英语 S1 Q1"],
  "contentTypes": ["题", "学习资料"],
  "openCourses": [],
  "openMaterials": ["五年级英语 S1 Q1 核心学习资料"],
  "openHomework": ["五年级英语 S1 Q1 练习题"],
  "blockedContent": ["课程"],
  "effectiveDefault": "今天起 365 天"
}
```

`alreadyOpened=true` 表示学生已有生效中的同套餐权限，前端会提示当前有效期并避免重复提交。`POST /api/grants` 请求体为 `{ "studentId": "stu-001", "packageId": "pkg-g05-english-s1-full" }`，成功后同步刷新学生学习权限。

### 商业订单与课消

运营教务、校区管理员、超级管理员可维护商业记录。收款在线下完成，系统只登记收款、课消、合同、发票和续费跟进；学习权限仍需通过“开通套餐”手动开通。

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

`POST /api/commercial/orders/{id}/contracts` 记录合同签署；`POST /api/commercial/orders/{id}/invoices` 记录开票；`POST /api/commercial/lesson-consumptions` 登记课消，超过订单剩余课时会被拒绝；`POST /api/commercial/renewal-reminders` 创建续费跟进；`POST /api/commercial/parent-notices` 给家长发送订单相关通知，并同步生成可追踪通知记录，`relatedType=commercial_order`，缺少公众号配置或学生 openid 时会返回 `待配置`，可通过通知页补发。

## 学生端

登录：小程序调用 `wx.login()` 获取 code 上送。配置环境变量 `WECHAT_APPID`、`WECHAT_SECRET` 后，后端通过 `jscode2session` 用真实 code 换取 openId；未配置时走演示映射（code 即 openId 后缀，如 `student`），保证本地无凭据可用。

- `POST /api/auth/wechat-login`
- `POST /api/auth/demo-student-login`：仅本地接口调试可用，不作为小程序默认登录入口；小程序使用 `POST /api/auth/wechat-login` 进行真实微信登录和手机号授权绑定。

首次绑定时，小程序需同时提交 `studentName`、`schoolName`、`grade` 和手机号授权凭据。后端不会为未识别微信自动创建临时学生账号；它会按手机号找到后台学生账号，并校验姓名、年级和已登记学校；手机号未匹配、手机号匹配多个账号、姓名或年级不一致、已绑定其他微信、账号停用时会拒绝绑定并返回明确提示。

```json
{
  "code": "wx-login-code",
  "phoneCode": "getPhoneNumber-code",
  "studentName": "小明",
  "schoolName": "星河小学",
  "grade": "五年级"
}
```

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
- `GET /api/student/growth` — 成长轨迹（提交记录 + 已学资料 + 最近成绩，按时间倒序）
- `GET /api/student/scores` — 当前学生成绩对比
- `GET /api/student/badges` — 徽章墙，`obtained` 由真实学习数据派生
- `GET /api/student/favorites` — 我的收藏列表
- `POST /api/student/favorites` — 收藏内容，请求体 `{ "targetType": "material|homework", "targetId": "mat-xxx" }`
- `DELETE /api/student/favorites/{id}` — 取消收藏

`GET /api/student/study` 返回 `{ student, courses: [{ ...course, progress }], materials }`，`progress` 为真实学习进度；学生端可根据 `student.openedPackages` 区分“未开通套餐”和“已开通但暂无课程内容”。
`GET /api/student/tasks` 返回任务数组，`studentStatus`（待完成/已完成）、`score`、`submissionId` 由提交记录派生。
`GET /api/student/home` 返回学生首页聚合数据，包含：

- `continueProgress`：当前推荐课程学习进度。
- `todayTodos`：今日待办，聚合作业、下一节课、课堂反馈和学习提醒授权入口。
- `classroomFeedback`：课后课堂反馈，由已批改提交沉淀，展示课程、练习、老师、分数、表现、重点和下一步。
- `subscriptionReminder`：微信订阅消息提醒配置状态；`templateIds` 来自 `WECHAT_MINIPROGRAM_SUBSCRIBE_TEMPLATE_IDS`，小程序用它调用 `wx.requestSubscribeMessage`。

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
