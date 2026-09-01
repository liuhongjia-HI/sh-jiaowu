package learningapp

import "starline/learning-api/internal/domain/learning"

// The following ports let focused application services and tests depend on a
// business capability instead of the compatibility aggregate Repository.
// Repository remains the aggregate accepted by NewService for existing wiring.
type StudentRepository interface {
	Students(learning.Principal, learning.StudentQuery) []learning.Student
	StudentDetail(learning.Principal, string) (learning.StudentDetail, error)
	CreateStudent(string, learning.Principal, learning.StudentUpsertRequest) (learning.Student, error)
	UpdateStudent(string, learning.Principal, string, learning.StudentUpsertRequest) (learning.Student, error)
	StudentLearningRecords(learning.Principal, string) ([]learning.StudentLearningRecord, error)
	StudentTutoringAssignments(learning.Principal, string) ([]learning.TutoringAssignment, error)
	CreateTutoringAssignment(string, learning.Principal, string, learning.TutoringAssignmentCreateRequest) (learning.TutoringAssignment, error)
	EndTutoringAssignment(string, learning.Principal, string, learning.TutoringAssignmentEndRequest) (learning.TutoringAssignment, error)
	TransferTutoringAssignment(string, learning.Principal, string, learning.TutoringAssignmentTransferRequest) (learning.TutoringAssignment, error)
	StudentScores(learning.Principal, string) ([]learning.StudentScoreSummary, error)
	RemindStudent(string, learning.Principal, string) (learning.StudentRemindResult, error)
	ImportStudents(string, learning.Principal, []learning.StudentUpsertRequest) (learning.StudentImportResult, error)
	CreateStudentScore(string, learning.Principal, string, learning.StudentScoreUpsertRequest) (learning.StudentScoreRecord, error)
	UpdateStudentScore(string, learning.Principal, string, string, learning.StudentScoreUpsertRequest) (learning.StudentScoreRecord, error)
	StudentHome(learning.Principal) (learning.StudentHome, error)
	StudentRecommendations(learning.Principal) ([]learning.StudentPackageRecommendation, error)
	LaunchCampaign(learning.Principal) (*learning.LaunchCampaign, error)
	CreateClassReservation(string, learning.Principal, learning.ClassReservationRequest) (learning.ClassReservationIntent, error)
	ClassReservations(learning.Principal) []learning.ClassReservationIntent
	UpdateClassReservation(string, learning.Principal, string, learning.ClassReservationUpdateRequest) (learning.ClassReservationIntent, error)
	ConfirmStudentSubscription(string, learning.Principal, learning.StudentSubscriptionRequest) (learning.SubscriptionReminder, error)
	UpdateStudentProfile(string, learning.Principal, learning.StudentProfileUpdateRequest) (learning.Student, error)
	StudentCourseDetail(learning.Principal, string) (learning.StudentCourseDetail, error)
	StudentMaterial(learning.Principal, string) (learning.Material, error)
	StudentMaterialPreviewFile(learning.Principal, string) (learning.FileAsset, error)
	StudentHomework(learning.Principal, string) (learning.Homework, error)
	RecordStudentSecurityEvent(string, learning.Principal, learning.SecurityEventRequest) error
	CreateSubmission(string, learning.Principal, learning.SubmissionRequest) (learning.Submission, error)
	StudentSubmission(learning.Principal, string) (learning.Submission, error)
	StudentStudy(learning.Principal) (learning.StudentStudyBoard, error)
	StudentTasks(learning.Principal) ([]learning.StudentTask, error)
	StudentGrowth(learning.Principal) ([]learning.StudentLearningRecord, error)
	StudentOwnScores(learning.Principal) ([]learning.StudentScoreSummary, error)
	StudentBadges(learning.Principal) ([]learning.Badge, error)
	StudentFavorites(learning.Principal) ([]learning.Favorite, error)
	AddFavorite(string, learning.Principal, learning.FavoriteRequest) (learning.Favorite, error)
	RemoveFavorite(string, learning.Principal, string) error
}

type ContentRepository interface {
	Courses(learning.Principal) []learning.Course
	CreateCourse(string, learning.Principal, learning.CourseUpsertRequest) (learning.Course, error)
	UpdateCourse(string, learning.Principal, string, learning.CourseUpsertRequest) (learning.Course, error)
	DeleteCourse(string, learning.Principal, string) error
	Questions(learning.Principal, learning.QuestionBankQuery) []learning.QuestionBankItem
	Materials(learning.Principal, learning.MaterialQuery) []learning.Material
	Homework(learning.Principal) []learning.Homework
	Reviews(learning.Principal) []learning.Review
	AssignReview(string, learning.Principal, string, learning.ReviewAssignRequest) (learning.Review, error)
	CreateQuestion(string, learning.Principal, learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error)
	UpdateQuestion(string, learning.Principal, string, learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error)
	CreateMaterial(string, learning.Principal, learning.MaterialUploadRequest) (learning.Material, error)
	UpdateMaterial(string, learning.Principal, string, learning.MaterialUpdateRequest) (learning.Material, error)
	ReorderMaterials(string, learning.Principal, learning.MaterialReorderRequest) error
	DeleteMaterial(string, learning.Principal, string) error
	HomeworkSubmissions(learning.Principal, string) (learning.HomeworkSubmissionSummary, error)
	CreateHomework(string, learning.Principal, learning.HomeworkUploadRequest) (learning.Homework, error)
	UpdateHomework(string, learning.Principal, string, learning.HomeworkUpdateRequest) (learning.Homework, error)
	DeleteHomework(string, learning.Principal, string) error
	ContentFile(learning.Principal, string) (learning.FileAsset, error)
	RecoverPreviewJobs() error
	ClaimPreviewJob() (learning.PreviewJob, bool, error)
	PreviewJobFile(string) (learning.FileAsset, error)
	CompletePreviewJob(string, learning.PreviewResult) error
	FailPreviewJob(string, string) error
	MarkPreviewFileMissing(string, string) error
	RetryPreviewJob(string, learning.Principal, string) error
	CompleteReview(string, learning.Principal, string, learning.ReviewCompleteRequest) (learning.Submission, error)
}

type GrantRepository interface {
	Packages() []learning.Package
	LearningSpaces() []learning.LearningSpace
	StudentGrants(learning.Principal, string) ([]learning.StudentGrant, error)
	GrantPreview(string, string) (learning.GrantPreview, error)
	CreateGrant(string, learning.GrantCreateRequest) (learning.GrantPreview, error)
	CreateDirectGrant(string, learning.DirectGrantCreateRequest) (learning.DirectGrantResult, error)
	ReplaceDirectGrant(string, learning.DirectGrantReplaceRequest) (learning.DirectGrantResult, error)
	CreatePackage(string, learning.PackageUpsertRequest) (learning.Package, error)
	UpdatePackage(string, string, learning.PackageUpsertRequest) (learning.Package, error)
}

type SchedulingRepository interface {
	Availability(learning.Principal, string, string) ([]learning.AvailabilitySlot, error)
	SaveAvailability(string, learning.Principal, learning.AvailabilityUpsertRequest) ([]learning.AvailabilitySlot, error)
	ScheduleCandidates(learning.Principal, learning.ScheduleCandidateRequest) ([]learning.ScheduleCandidate, error)
	ScheduleClasses(learning.Principal) []learning.ScheduleClass
	StudentSchedule(learning.Principal) ([]learning.ScheduleClass, error)
	AvailabilityOverview(learning.Principal) []learning.AvailabilitySlot
	CreateScheduleClass(string, learning.Principal, learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error)
	UpdateScheduleClass(string, learning.Principal, string, learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error)
	CancelScheduleClass(string, learning.Principal, string) (learning.ScheduleClass, error)
	ReviewScheduleClass(string, learning.Principal, string, bool, string) (learning.ScheduleClass, error)
	PendingScheduleClasses(learning.Principal) []learning.ScheduleClass
	LessonFeedbacks(learning.Principal, string) ([]learning.LessonFeedback, error)
	UpsertLessonFeedback(string, learning.Principal, string, learning.LessonFeedbackUpsertRequest) (learning.LessonFeedback, error)
}

type CommercialRepository interface {
	CommercialSummary(learning.Principal) learning.CommercialSummary
	CommercialOrders(learning.Principal) []learning.CommercialOrder
	CreateCommercialOrder(string, learning.Principal, learning.CommercialOrderCreateRequest) (learning.CommercialOrder, error)
	CreatePayment(string, learning.Principal, string, learning.PaymentCreateRequest) (learning.PaymentRecord, error)
	CreateRefund(string, learning.Principal, string, learning.RefundCreateRequest) (learning.RefundRecord, error)
	RefundAndSuspendStudent(string, learning.Principal, string, learning.RefundAndSuspendRequest) (learning.RefundSuspensionResult, error)
	CreateContract(string, learning.Principal, string, learning.ContractCreateRequest) (learning.ContractRecord, error)
	CreateInvoice(string, learning.Principal, string, learning.InvoiceCreateRequest) (learning.InvoiceRecord, error)
	CreateLessonConsumption(string, learning.Principal, learning.LessonConsumptionCreateRequest) (learning.LessonConsumption, error)
	CreateRenewalReminder(string, learning.Principal, learning.RenewalReminderCreateRequest) (learning.RenewalReminder, error)
	CreateParentNotice(string, learning.Principal, learning.ParentNoticeCreateRequest) (learning.ParentNotice, error)
}

type NoticeRepository interface {
	Notices(learning.Principal) []learning.Notice
	CreateNotice(string, learning.Principal, learning.NoticeCreateRequest) (learning.Notice, error)
	RetryNotice(string, learning.Principal, string) (learning.Notice, error)
	Logs() []learning.OperationLog
	StudentPermissions() []learning.StudentPermissionSummary
	PackagePermissions() []learning.PackagePermissionSummary
	ContentPermissions() []learning.ContentPermissionSummary
}

type BannerRepository interface {
	Banners() []learning.Banner
	ActiveStudentBanners() []learning.Banner
	CreateBanner(string, learning.BannerUpsertRequest) (learning.Banner, error)
	UpdateBanner(string, string, learning.BannerUpsertRequest) (learning.Banner, error)
	DeleteBanner(string, string) error
}

type SystemRepository interface {
	Dashboard(learning.Principal) learning.DashboardOverview
	SystemReadiness() learning.SystemReadiness
	Settings() map[string]string
	UpdateSetting(string, learning.SettingUpdateRequest) (map[string]string, error)
	Subjects() []learning.SubjectMetadata
	UpdateSubjectMetadata(string, string, learning.SubjectMetadataUpdateRequest) (learning.SubjectMetadata, error)
	GradeSubjects() []learning.GradeSubjectMetadata
	UpdateGradeSubjects(string, learning.GradeSubjectCatalogUpdateRequest) ([]learning.GradeSubjectMetadata, error)
}
