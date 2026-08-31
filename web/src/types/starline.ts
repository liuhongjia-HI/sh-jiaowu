export type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
};

export type Role = 'student' | 'teacher' | 'ops_staff' | 'campus_admin' | 'super_admin';

export type CurrentUser = {
  userId: string;
  name: string;
  studentId?: string;
  authMethod?: 'wechat' | 'password' | 'demo';
  campusId?: string;
  roles: Role[];
  mustChangePassword?: boolean;
  campusScopes?: string[];
  learningSpaceIds?: string[];
  canUploadHandout?: boolean;
  canUploadQuestion?: boolean;
  canReview?: boolean;
};

export type AuthResult = {
  token: string;
  user: CurrentUser;
  authMethod: 'wechat' | 'password' | 'demo';
};

export type CaptchaChallenge = {
  captchaId: string;
  question: string;
};

export type PasswordResetResult = {
  userId: string;
  temporaryPassword: string;
  mustChangePassword: boolean;
};

export type Teacher = {
  id: string;
  name: string;
  phone: string;
  campusId: string;
  learningSpaceIds: string[];
  learningSpaces: string[];
  grades: string[];
  subjects: string[];
  canUploadHandout: boolean;
  canUploadQuestion: boolean;
  canReview: boolean;
  accountStatus: string;
  bindStatus: string;
  remark: string;
  temporaryPassword?: string;
  // 名下还没结束、也没取消的排课数量。停用账号前用这个数字提醒教务，
  // 不用等老师登不进去了才发现课还没交接。
  activeClassCount: number;
};

export type TeacherUpsertRequest = {
  name: string;
  phone: string;
  campusId?: string;
  learningSpaceIds: string[];
  canUploadHandout: boolean;
  canUploadQuestion: boolean;
  canReview: boolean;
  accountStatus?: string;
  remark: string;
};

export type LearningSpace = {
  id: string;
  /** 数据库历史遗留列，纯展示、不参与匹配，别用它拼学年下拉——校历（settings.academicCalendar）才是学年的权威来源，见 academicYearsFromCalendar。 */
  academicYear: string;
  grade: string;
  subject: string;
  semester: string;
  phase: string;
  level?: string;
  name: string;
  status: string;
};

export type AdminStaff = {
  id: string;
  name: string;
  phone: string;
  role: Role;
  campusId?: string;
  accountStatus: string;
  bindStatus: string;
  remark: string;
};

export type AdminStaffUpsertRequest = {
  name: string;
  phone: string;
  role: Role;
  campusId?: string;
  accountStatus?: string;
  remark: string;
};

export type DashboardOverview = {
  openedStudents: number;
  packageCount: number;
  pendingReviews: number;
  materialViews: number;
  expiringStudents: number;
  unpublishedFiles: number;
};

export type ReadinessItem = {
  key: string;
  title: string;
  status: 'ready' | 'warning' | 'missing';
  message: string;
  action?: string;
};

export type SystemReadiness = {
  readyCount: number;
  totalCount: number;
  items: ReadinessItem[];
};

export type StudyPackage = {
  id: string;
  name: string;
  academicYear: string;
  grade: string;
  semester: string;
  subject: string;
  level?: string;
  phaseScope: string;
  packageType: string;
  summary: string;
  learningSpaceIds?: string[];
  learningSpaces?: string[];
  contentTypeCodes?: string[];
  contentTypes?: string[];
  openStudentNum: number;
  status: string;
};

export type PackageUpsertRequest = {
  name: string;
  academicYear: string;
  grade: string;
  semester: string;
  subject: string;
  level: string;
  phaseScope: string;
  packageType: string;
  summary: string;
  learningSpaceIds: string[];
  contentTypeCodes: string[];
  status: string;
};

export type Student = {
  id: string;
  name: string;
  nickname?: string;
  avatarUrl?: string;
  grade: string;
  phone: string;
  schoolName?: string;
  guardianName?: string;
  officialAccountOpenId?: string;
  openedPackages: string[];
  openedPackageRefs: StudentPackageRef[];
  learningStatus: string;
  accountStatus: string;
  registrationSource?: string;
  followUpStatus?: string;
  streakDays: number;
  averageScore: number;
  badgeCount: number;
  remark?: string;
  bindStatus: string;
  createdAt: string;
  lastStudyAt?: string;
  lastSubmittedAt?: string;
  lastSubmissionStatus?: string;
  effectiveUntil?: string;
  activeTutoringAssignments?: TutoringAssignmentSummary[];
};

export type StudentPackageRef = {
  packageId: string;
  packageName: string;
};

export type StudentUpsertRequest = {
  name: string;
  phone: string;
  grade: string;
  schoolName?: string;
  guardianName?: string;
  officialAccountOpenId?: string;
  accountStatus?: string;
  remark: string;
};

export type StudentGrant = {
  studentId: string;
  packageId: string;
  packageName: string;
  startsAt: string;
  effectiveUntil: string;
  permissionState: string;
  isDirect: boolean;
  learningSpaceIds: string[];
  learningSpaces: string[];
  contentTypes: string[];
  openCourses: string[];
  openMaterials: string[];
  openHomework: string[];
};

export type StudentOpeningItem = {
  id: string;
  title: string;
};

export type StudentOpeningCell = {
  contentTypeCode: 'course' | 'handout' | 'question' | 'download';
  opened: boolean;
  packageOpened: boolean;
  directOpened: boolean;
  packageNames: string[];
  items: StudentOpeningItem[];
};

export type StudentOpeningScope = {
  learningSpaceId: string;
  name: string;
  subject: string;
  content: StudentOpeningCell[];
};

export type TutoringAssignment = {
  id: string;
  studentId: string;
  teacherId: string;
  teacherName: string;
  campusId: string;
  academicYear: string;
  gradeSnapshot: string;
  subjectId: string;
  subjectName: string;
  levelCode: string;
  role: 'primary' | 'assistant';
  status: 'pending' | 'active' | 'ended';
  startsAt: string;
  endsAt?: string;
  endedReason?: string;
  assignedBy?: string;
  endedBy?: string;
  version: number;
  createdAt: string;
  updatedAt: string;
};

export type TutoringAssignmentSummary = {
  teacherId: string;
  teacherName: string;
  subjectName: string;
  levelCode: string;
  role: 'primary' | 'assistant';
  startsAt: string;
};

export type TutoringAssignmentCreateRequest = {
  teacherId: string;
  subjectId: string;
  levelCode: string;
  role?: 'primary' | 'assistant';
  startsAt?: string;
};

export type DirectGrantCreateRequest = {
  studentId: string;
  learningSpaceIds: string[];
  contentTypeCodes: string[];
  startsAt?: string;
  endsAt?: string;
};

export type DirectGrantSelection = {
  learningSpaceId: string;
  contentTypeCodes: string[];
};

export type DirectGrantReplaceRequest = {
  studentId: string;
  selections: DirectGrantSelection[];
  startsAt?: string;
  endsAt?: string;
};

export type DirectGrantResult = {
  studentId: string;
  studentName: string;
  learningSpaces: string[];
  contentTypes: string[];
  openCourses: string[];
  openMaterials: string[];
  openHomework: string[];
};

export type StudentLearningRecord = {
  id: string;
  type: string;
  title: string;
  course: string;
  status: string;
  score?: number;
  fullScore?: number;
  occurredAt: string;
  description: string;
};

export type StudentScoreRecord = {
  id: string;
  studentId: string;
  subject: string;
  examType: string;
  examName: string;
  examDate: string;
  score: number;
  fullScore: number;
  averageScore?: number;
  teacherComment?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type StudentScoreSummary = {
  subject: string;
  records: StudentScoreRecord[];
  firstRecord?: StudentScoreRecord;
  latestRecord?: StudentScoreRecord;
  improvement: number;
  improvementPct: number;
  description: string;
  problemPoint?: string;
  nextStep?: string;
};

export type StudentScoreUpsertRequest = {
  subject: string;
  examType: string;
  examName: string;
  examDate: string;
  score: number;
  fullScore: number;
  averageScore: number;
  teacherComment: string;
};

export type StudentDetail = {
  student: Student;
  grants: StudentGrant[];
  openingMatrix: StudentOpeningScope[];
  permissions: StudentPermissionSummary;
  learningRecords: StudentLearningRecord[];
  notices: Notice[];
  logs: OperationLog[];
};

export type StudentImportResult = {
  successCount: number;
  failedCount: number;
  errors: { row: number; message: string }[];
};

export type StudentRemindResult = {
  noticeId: string;
  message: string;
};

export type Course = {
  id: string;
  name: string;
  subject: string;
  grade: string;
  learningSpaceId?: string;
  lessonCount: number;
  curriculum: CurriculumNode[];
  materialNum: number;
  homeworkNum: number;
  status: string;
};

export type CourseUpsertRequest = {
  name: string;
  learningSpaceId: string;
  curriculum: CurriculumNode[];
  status: string;
};

export type CurriculumNode = {
  id: string;
  parentId?: string;
  type: 'unit' | 'chapter' | 'lesson';
  name: string;
  sortOrder: number;
};

export type CurriculumPath = {
  unit: string;
  chapter: string;
  lesson: string;
};

export type SettingUpdateRequest = {
  key: string;
  value: string;
};

export type SubjectMetadata = {
  id: string;
  name: string;
  shortLabel: string;
  color: string;
  sortOrder: number;
  status: '启用' | '停用';
};

export type SubjectMetadataUpdateRequest = Pick<SubjectMetadata, 'shortLabel' | 'color' | 'sortOrder' | 'status'>;

export type Material = {
  id: string;
  title: string;
  courseId?: string;
  course: string;
  learningSpaceId?: string;
  grade?: string;
  semester?: string;
  subject?: string;
  lessonId: string;
  curriculum: CurriculumPath;
  tagCode?: string;
  type: string;
  viewCount: number;
  ownerTeacherId?: string;
  ownerTeacherName?: string;
  publishStatus?: string;
  fileId?: string;
  fileName?: string;
  fileSize?: number;
  fileType?: string;
  previewStatus?: string;
  previewError?: string;
  previewUrl?: string;
  downloadUrl?: string;
  watermarkText?: string;
  securityNotice?: string;
  createdAt?: string;
  sortOrder: number;
  status: string;
};

export type MaterialUpdateRequest = {
  title: string;
  courseId: string;
  learningSpaceId?: string;
  lessonId: string;
  tagCode?: string;
  status: string;
};

export type MaterialReorderRequest = {
  courseId: string;
  materialIds: string[];
};

export type Homework = {
  id: string;
  title: string;
  packageName: string;
  courseId?: string;
  course: string;
  learningSpaceId?: string;
  grade?: string;
  semester?: string;
  subject?: string;
  lessonId: string;
  curriculum: CurriculumPath;
  tagCode?: string;
  questionNum: number;
  questionIds?: string[];
  questions?: Question[];
  deadline: string;
  deadlineAt?: string;
  assessmentType?: 'practice' | 'mock_exam';
  isOverdue?: boolean;
  submittedNum: number;
  totalNum: number;
  ownerTeacherId?: string;
  ownerTeacherName?: string;
  publishStatus?: string;
  fileId?: string;
  fileName?: string;
  fileSize?: number;
  fileType?: string;
  previewStatus?: string;
  previewError?: string;
  previewUrl?: string;
  downloadUrl?: string;
  watermarkText?: string;
  securityNotice?: string;
  sortOrder?: number;
  status: string;
};

export type HomeworkSubmissionStudent = {
  studentId: string;
  studentName: string;
  phone: string;
  submittedAt?: string;
  reviewStatus: string;
  submissionId?: string;
};

export type HomeworkSubmissionSummary = {
  homeworkId: string;
  homeworkTitle: string;
  totalNum: number;
  submittedNum: number;
  students: HomeworkSubmissionStudent[];
};

export type HomeworkUpdateRequest = {
  title: string;
  courseId: string;
  learningSpaceId?: string;
  tagCode?: string;
  lessonId: string;
  deadline: string;
  deadlineAt?: string;
  assessmentType?: 'practice' | 'mock_exam';
  status: string;
  questionIds?: string[];
};

export type Question = {
  id: string;
  title?: string;
  type: 'single' | 'multiple' | 'judge' | 'fill' | 'text';
  stem: string;
  options?: string[];
  score?: number;
};

export type QuestionBankItem = Question & {
  title: string;
  grade: string;
  semester: string;
  subject: string;
  answer?: string;
  answers?: string[];
  status: string;
  ownerTeacherId?: string;
  ownerTeacherName?: string;
};

export type QuestionBankUpsertRequest = {
  title: string;
  grade: string;
  semester: string;
  subject: string;
  type: 'single' | 'multiple' | 'judge' | 'fill' | 'text';
  stem: string;
  options: string[];
  answer?: string;
  answers?: string[];
  score: number;
  status: string;
};

export type Review = {
  id: string;
  studentId?: string;
  homeworkId?: string;
  submissionId?: string;
  studentName: string;
  packageName: string;
  homework: string;
  systemScore: number;
  teacherComment?: string;
  reward?: string;
  status: string;
  reviewerTeacherId?: string;
  reviewerTeacherName?: string;
  tutoringAssignmentId?: string;
  assignedAt?: string;
};

export type ReviewCompleteRequest = {
  score: number;
  teacherComment: string;
  reward?: string;
  finalStatus?: string;
};

export type ReviewAssignRequest = {
  teacherId: string;
  reason: string;
};

export type Notice = {
  id: string;
  type: string;
  title: string;
  target: string;
  summary: string;
  channel?: string;
  recipientOpenId?: string;
  status: string;
  failureReason?: string;
  relatedType?: string;
  relatedId?: string;
  retryCount?: number;
};

export type NoticeCreateRequest = {
  type: string;
  title: string;
  target: string;
  summary: string;
  channel?: string;
  recipientOpenId?: string;
  relatedType?: string;
  relatedId?: string;
};

export type BannerLinkType = 'none' | 'page' | 'url';

export type Banner = {
  id: string;
  imageUrl: string;
  title: string;
  linkType: BannerLinkType;
  linkValue: string;
  sortOrder: number;
  startsAt?: string;
  endsAt?: string;
  enabled: boolean;
  // 后端算好的展示状态：生效中 / 未开始 / 已结束 / 已停用，前端不用自己比日期。
  status: string;
  createdAt: string;
};

export type BannerUpsertRequest = {
  imageUrl: string;
  title: string;
  linkType: BannerLinkType;
  linkValue: string;
  sortOrder: number;
  startsAt?: string;
  endsAt?: string;
  enabled: boolean;
};

export type OperationLog = {
  id: string;
  operator: string;
  operatorId?: string;
  ip?: string;
  userAgent?: string;
  action: string;
  target: string;
  detail?: string;
  time: string;
};

export type CommercialOrder = {
  id: string;
  orderNo: string;
  studentId: string;
  studentName: string;
  packageId: string;
  packageName: string;
  amountCent: number;
  paidAmountCent: number;
  refundedAmountCent: number;
  lessonTotal: number;
  lessonConsumed: number;
  status: string;
  contractStatus: string;
  invoiceStatus: string;
  createdAt: string;
};

export type CommercialSummary = {
  orderCount: number;
  paidOrderCount: number;
  revenueCent: number;
  refundCent: number;
  lessonRemainCount: number;
  renewalTodoCount: number;
};

export type CommercialOrderCreateRequest = {
  studentId: string;
  packageId: string;
  amountCent: number;
  lessonTotal: number;
  remark: string;
};

export type PaymentCreateRequest = {
  amountCent: number;
  method: string;
  transactionNo: string;
};

export type RefundCreateRequest = {
  amountCent: number;
  reason: string;
};

export type RefundSuspensionResult = {
  refund: {
    id: string;
    orderId: string;
    amountCent: number;
    reason: string;
    refundedAt: string;
    status: string;
  };
  student: Student;
  revokedGrantCount: number;
  removedFutureClassCount: number;
};

export type ContractCreateRequest = {
  title: string;
  signer: string;
};

export type InvoiceCreateRequest = {
  title: string;
  taxNo: string;
  amountCent: number;
  invoiceNo: string;
};

export type LessonConsumptionCreateRequest = {
  orderId: string;
  scheduleClassId: string;
  lessonCount: number;
  remark: string;
};

export type RenewalReminderCreateRequest = {
  orderId: string;
  reason: string;
  dueAt: string;
};

export type ParentNoticeCreateRequest = {
  orderId: string;
  title: string;
  content: string;
};

export type GrantPreview = {
  studentId: string;
  packageId: string;
  studentName: string;
  packageName: string;
  alreadyOpened: boolean;
  existingStartsAt?: string;
  existingUntil?: string;
  learningSpaces: string[];
  contentTypes: string[];
  openCourses: string[];
  openMaterials: string[];
  openHomework: string[];
  blockedContent: string[];
  effectiveDefault: string;
  startsAtDefault: string;
  endsAtDefault: string;
};

export type GrantCreateRequest = {
  studentId?: string;
  packageId: string;
  startsAt?: string;
  endsAt?: string;
};

export type StudentPermissionSummary = {
  studentId: string;
  studentName: string;
  grade: string;
  accountStatus: string;
  openedPackages: string[];
  learningSpaces: string[];
  contentTypes: string[];
  openCourses: string[];
  openMaterials: string[];
  openHomework: string[];
  effectiveUntil: string;
  permissionState: string;
};

export type PackagePermissionSummary = {
  packageId: string;
  packageName: string;
  status: string;
  openedStudents: number;
  students: string[];
  learningSpaces: string[];
  contentTypes: string[];
  openCourses: string[];
  openMaterials: string[];
  openHomework: string[];
};

export type ContentPermissionSummary = {
  contentId: string;
  contentTitle: string;
  contentType: string;
  course: string;
  learningSpace: string;
  ownerTeacherName?: string;
  status: string;
  openedPackages: string[];
  openedStudents: string[];
};

export type AvailabilitySlot = {
  id: string;
  ownerType: 'teacher' | 'student';
  ownerId: string;
  ownerName: string;
  dayOfWeek: number;
  startTime: string;
  endTime: string;
  startDate?: string;
  endDate?: string;
  remark?: string;
};

export type CandidateStudent = {
  id: string;
  name: string;
  grade: string;
  openedPackages: string[];
};

export type ScheduleCandidate = {
  id: string;
  dayOfWeek: number;
  startTime: string;
  endTime: string;
  teacherId: string;
  teacherName: string;
  courseId: string;
  courseName: string;
  subject: string;
  grade: string;
  level?: string;
  classType: string;
  capacity: number;
  availableStudents: CandidateStudent[];
  missingStudents: CandidateStudent[];
  studentCount: number;
  score: number;
  reason: string;
};

export type ScheduleClass = {
  id: string;
  name: string;
  courseId: string;
  courseName: string;
  teacherId: string;
  teacherName: string;
  campusId: string;
  roomName: string;
  classType: string;
  capacity: number;
  durationMinutes: number;
  /** 星期由 lessonDate 推导，后端只读不写，前端不要拿它当排课依据。 */
  dayOfWeek: number;
  startTime: string;
  endTime: string;
  /** 这节课的具体日期。一条记录 = 一节课，startDate/endDate 恒等于它。 */
  lessonDate: string;
  startDate: string;
  endDate: string;
  /** 同一次重复排课生成的课次共享它；为空表示单次课。 */
  seriesId?: string;
  /** 已被单独调整过，此后不再跟随系列的批量改动。 */
  detached?: boolean;
  /** 排这节课时越过了哪些可上课时间（软提醒），留痕用。 */
  overrideNote?: string;
  /**
   * 审核维度：待审核 / 已通过 / 已驳回。与 status 的成班维度是两件事——
   * status 说的是人数够不够、有没有被取消，auditStatus 说的是管理员认不认。
   * 只有已通过的课次才对学生可见。
   */
  auditStatus: string;
  auditReason?: string;
  auditedBy?: string;
  auditedAt?: string;
  createdBy?: string;
  createdByRole?: string;
  /** 建班时按开课日期落校历判定一次，此后固定不变，不随校历调整或学年切换漂移。 */
  academicYear?: string;
  semester?: string;
  students: CandidateStudent[];
  expectedStudentCount: number;
  reservationNote?: string;
  status: string;
  createdAt: string;
};

export type LessonFeedback = {
  id: string;
  scheduleClassId: string;
  studentId: string;
  studentName: string;
  teacherId: string;
  teacherName: string;
  courseName: string;
  lessonDate: string;
  summary: string;
  homework?: string;
  nextStep?: string;
  createdAt: string;
  updatedAt: string;
};
