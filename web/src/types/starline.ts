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
  campusId?: string;
  roles: Role[];
  campusScopes?: string[];
  learningSpaceIds?: string[];
  canUploadHandout?: boolean;
  canUploadQuestion?: boolean;
  canReview?: boolean;
};

export type AuthResult = {
  token: string;
  user: CurrentUser;
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
  academicYear: string;
  grade: string;
  subject: string;
  semester: string;
  phase: string;
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

export type StudyPackage = {
  id: string;
  name: string;
  academicYear: string;
  grade: string;
  semester: string;
  subject: string;
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
  grade: string;
  phone: string;
  openedPackages: string[];
  learningStatus: string;
  accountStatus: string;
  streakDays: number;
  averageScore: number;
  badgeCount: number;
  remark?: string;
  bindStatus: string;
  lastStudyAt?: string;
  effectiveUntil?: string;
};

export type StudentUpsertRequest = {
  name: string;
  phone: string;
  grade: string;
  accountStatus?: string;
  remark: string;
};

export type StudentGrant = {
  studentId: string;
  packageId: string;
  packageName: string;
  effectiveUntil: string;
  permissionState: string;
};

export type StudentLearningRecord = {
  id: string;
  type: string;
  title: string;
  course: string;
  status: string;
  score?: number;
  occurredAt: string;
  description: string;
};

export type StudentDetail = {
  student: Student;
  grants: StudentGrant[];
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
  chapterCount: number;
  materialNum: number;
  homeworkNum: number;
  status: string;
};

export type CourseUpsertRequest = {
  name: string;
  learningSpaceId: string;
  chapterCount: number;
  status: string;
};

export type SettingUpdateRequest = {
  key: string;
  value: string;
};

export type Material = {
  id: string;
  title: string;
  courseId?: string;
  course: string;
  learningSpaceId?: string;
  chapter: string;
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
  previewUrl?: string;
  downloadUrl?: string;
  status: string;
};

export type MaterialUpdateRequest = {
  title: string;
  courseId: string;
  learningSpaceId?: string;
  chapter: string;
  status: string;
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
  questionNum: number;
  questionIds?: string[];
  questions?: Question[];
  deadline: string;
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
  previewUrl?: string;
  downloadUrl?: string;
  status: string;
};

export type HomeworkUpdateRequest = {
  title: string;
  courseId: string;
  learningSpaceId?: string;
  deadline: string;
  status: string;
  questionIds?: string[];
};

export type Question = {
  id: string;
  type: 'single' | 'multiple' | 'text';
  stem: string;
  options?: string[];
  score?: number;
};

export type QuestionBankItem = Question & {
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
  grade: string;
  semester: string;
  subject: string;
  type: 'single' | 'multiple' | 'text';
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
};

export type ReviewCompleteRequest = {
  score: number;
  teacherComment: string;
  reward?: string;
};

export type Notice = {
  id: string;
  type: string;
  title: string;
  target: string;
  summary: string;
  status: string;
};

export type NoticeCreateRequest = {
  type: string;
  title: string;
  target: string;
  summary: string;
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
  existingUntil?: string;
  learningSpaces: string[];
  contentTypes: string[];
  openCourses: string[];
  openMaterials: string[];
  openHomework: string[];
  blockedContent: string[];
  effectiveDefault: string;
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
  classType: string;
  capacity: number;
  durationMinutes: number;
  dayOfWeek: number;
  startTime: string;
  endTime: string;
  startDate: string;
  endDate: string;
  students: CandidateStudent[];
  status: string;
  createdAt: string;
};
