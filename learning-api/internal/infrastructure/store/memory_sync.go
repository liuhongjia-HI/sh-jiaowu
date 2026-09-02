package store

import (
	"errors"
	"strings"

	"starline/learning-api/internal/domain/learning"
)

// Public MemoryStore methods are the synchronization boundary. Implementations
// live in *Unlocked helpers so internal composition cannot recursively acquire
// the mutex. Store-owned slices and maps are copied explicitly where they
// enter or leave state; see memory_copy.go and the relevant unlocked helper.

func (s *MemoryStore) CommercialSummary(principal learning.Principal) learning.CommercialSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.commercialSummaryUnlocked(principal)
	return result
}

func (s *MemoryStore) CommercialOrders(principal learning.Principal) []learning.CommercialOrder {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.commercialOrdersUnlocked(principal)
	return result
}

func (s *MemoryStore) CreateCommercialOrder(operator string, principal learning.Principal, req learning.CommercialOrderCreateRequest) (learning.CommercialOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createCommercialOrderUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) CreatePayment(operator string, principal learning.Principal, orderID string, req learning.PaymentCreateRequest) (learning.PaymentRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createPaymentUnlocked(operator, principal, orderID, req)
	return result1, err
}

func (s *MemoryStore) CreateRefund(operator string, principal learning.Principal, orderID string, req learning.RefundCreateRequest) (learning.RefundRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createRefundUnlocked(operator, principal, orderID, req)
	return result1, err
}

func (s *MemoryStore) RefundAndSuspendStudent(operator string, principal learning.Principal, orderID string, req learning.RefundAndSuspendRequest) (learning.RefundSuspensionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refundAndSuspendStudentUnlocked(operator, principal, orderID, req)
}

func (s *MemoryStore) CreateContract(operator string, principal learning.Principal, orderID string, req learning.ContractCreateRequest) (learning.ContractRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createContractUnlocked(operator, principal, orderID, req)
	return result1, err
}

func (s *MemoryStore) CreateInvoice(operator string, principal learning.Principal, orderID string, req learning.InvoiceCreateRequest) (learning.InvoiceRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createInvoiceUnlocked(operator, principal, orderID, req)
	return result1, err
}

func (s *MemoryStore) CreateLessonConsumption(operator string, principal learning.Principal, req learning.LessonConsumptionCreateRequest) (learning.LessonConsumption, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createLessonConsumptionUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) CreateRenewalReminder(operator string, principal learning.Principal, req learning.RenewalReminderCreateRequest) (learning.RenewalReminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createRenewalReminderUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) CreateParentNotice(operator string, principal learning.Principal, req learning.ParentNoticeCreateRequest) (learning.ParentNotice, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.ParentNotice, error) {
		return work.createParentNoticeUnlocked(operator, principal, req)
	}, refreshParentNotice)
}

func (s *MemoryStore) GrantPreview(studentID, packageID string) (learning.GrantPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.grantPreviewUnlocked(studentID, packageID)
	return result1, err
}

func (s *MemoryStore) CreateGrant(operator string, req learning.GrantCreateRequest) (learning.GrantPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createGrantUnlocked(operator, req)
	return result1, err
}

func (s *MemoryStore) RevokePackageGrant(operator, studentID, packageID string) (learning.GrantRevokeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revokePackageGrantUnlocked(operator, studentID, packageID)
}

func (s *MemoryStore) StartStudentTrial(principal learning.Principal, packageID string) (learning.StudentTrialStartResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startStudentTrialUnlocked(principal, packageID)
}

func (s *MemoryStore) CreateDirectGrant(operator string, req learning.DirectGrantCreateRequest) (learning.DirectGrantResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.createDirectGrantUnlocked(operator, req)
	return result, err
}

func (s *MemoryStore) ReplaceDirectGrant(operator string, req learning.DirectGrantReplaceRequest) (learning.DirectGrantResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replaceDirectGrantUnlocked(operator, req)
}

func (s *MemoryStore) StudentPermissions() []learning.StudentPermissionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.studentPermissionsUnlocked()
	return result
}

func (s *MemoryStore) PackagePermissions() []learning.PackagePermissionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.packagePermissionsUnlocked()
	return result
}

func (s *MemoryStore) ContentPermissions() []learning.ContentPermissionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.contentPermissionsUnlocked()
	return result
}

func (s *MemoryStore) Courses(principal learning.Principal) []learning.Course {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.coursesUnlocked(principal)
	return result
}

func (s *MemoryStore) CreateCourse(operator string, principal learning.Principal, req learning.CourseUpsertRequest) (learning.Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createCourseUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) UpdateCourse(operator string, principal learning.Principal, id string, req learning.CourseUpsertRequest) (learning.Course, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateCourseUnlocked(operator, principal, id, req)
	return result1, err
}

func (s *MemoryStore) DeleteCourse(operator string, principal learning.Principal, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteCourseUnlocked(operator, principal, id)
}

func (s *MemoryStore) Materials(principal learning.Principal, query learning.MaterialQuery) []learning.Material {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.materialsFilteredUnlocked(principal, query)
	return result
}

func (s *MemoryStore) CreateMaterial(operator string, principal learning.Principal, req learning.MaterialUploadRequest) (learning.Material, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createMaterialUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) UpdateMaterial(operator string, principal learning.Principal, id string, req learning.MaterialUpdateRequest) (learning.Material, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateMaterialUnlocked(operator, principal, id, req)
	return result1, err
}

func (s *MemoryStore) ReorderMaterials(operator string, principal learning.Principal, req learning.MaterialReorderRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reorderMaterialsUnlocked(operator, principal, req)
}
func (s *MemoryStore) ReorderHomework(operator string, principal learning.Principal, req learning.HomeworkReorderRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.CourseID == "" || len(req.HomeworkIDs) == 0 {
		return errors.New("请选择需要排序的练习")
	}
	for i, id := range req.HomeworkIDs {
		for j := range s.homework {
			if s.homework[j].ID == id && s.homework[j].CourseID == req.CourseID {
				s.homework[j].SortOrder = i + 1
			}
		}
	}
	return nil
}

func (s *MemoryStore) DeleteMaterial(operator string, principal learning.Principal, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteMaterialUnlocked(operator, principal, id)
}

func (s *MemoryStore) Homework(principal learning.Principal) []learning.Homework {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.homeworkUnlocked(principal)
	return result
}

func (s *MemoryStore) HomeworkSubmissions(principal learning.Principal, homeworkID string) (learning.HomeworkSubmissionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.homeworkSubmissionsUnlocked(principal, homeworkID)
	return result1, err
}

func (s *MemoryStore) Questions(principal learning.Principal, query learning.QuestionBankQuery) []learning.QuestionBankItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.questionsUnlocked(principal, query)
	return result
}

func (s *MemoryStore) CreateQuestion(operator string, principal learning.Principal, req learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createQuestionUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) UpdateQuestion(operator string, principal learning.Principal, id string, req learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateQuestionUnlocked(operator, principal, id, req)
	return result1, err
}

func (s *MemoryStore) CreateHomework(operator string, principal learning.Principal, req learning.HomeworkUploadRequest) (learning.Homework, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.Homework, error) {
		return work.createHomeworkUnlocked(operator, principal, req)
	}, nil)
}

func (s *MemoryStore) UpdateHomework(operator string, principal learning.Principal, id string, req learning.HomeworkUpdateRequest) (learning.Homework, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.Homework, error) {
		return work.updateHomeworkUnlocked(operator, principal, id, req)
	}, nil)
}

func (s *MemoryStore) DeleteHomework(operator string, principal learning.Principal, id string) error {
	_, err := noticeMutation(s, func(work *MemoryStore) (struct{}, error) {
		return struct{}{}, work.deleteHomeworkUnlocked(operator, principal, id)
	}, nil)
	return err
}

func (s *MemoryStore) ContentFile(principal learning.Principal, fileID string) (learning.FileAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.contentFileUnlocked(principal, fileID)
	return result1, err
}

func (s *MemoryStore) Reviews(principal learning.Principal) []learning.Review {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.reviewsUnlocked(principal)
	return result
}

func (s *MemoryStore) CompleteReview(operator string, principal learning.Principal, id string, req learning.ReviewCompleteRequest) (learning.Submission, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.Submission, error) {
		return work.completeReviewUnlocked(operator, principal, id, req)
	}, nil)
}

func (s *MemoryStore) AssignReview(operator string, principal learning.Principal, id string, req learning.ReviewAssignRequest) (learning.Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.assignReviewUnlocked(operator, principal, id, req)
}

func (s *MemoryStore) ConnectSchedulingDB(dsn string) error {
	return s.connectPrepared(dsn, true)
}

func (s *MemoryStore) LoginWithWechatCode(req learning.WechatLoginRequest) (learning.Principal, error) {
	req = req
	req.Code = strings.TrimSpace(req.Code)
	req.Phone = strings.TrimSpace(req.Phone)
	req.PhoneCode = strings.TrimSpace(req.PhoneCode)
	req.StudentName = strings.TrimSpace(req.StudentName)
	req.SchoolName = strings.TrimSpace(req.SchoolName)
	req.Grade = strings.TrimSpace(req.Grade)
	s.mu.Lock()
	wechatResolver := s.wechatResolver
	phoneResolver := s.phoneResolver
	s.mu.Unlock()
	openID, err := resolveWechatOpenID(wechatResolver, req.Code)
	if err != nil {
		return learning.Principal{}, err
	}
	if req.Phone == "" && req.PhoneCode != "" {
		req.Phone, err = resolveWechatPhone(phoneResolver, req.PhoneCode)
		if err != nil {
			return learning.Principal{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.loginWithWechatResolvedUnlocked(req, openID, wechatResolver != nil)
	return result1, err
}

func resolveWechatOpenID(resolver func(string) (string, error), code string) (string, error) {
	if code == "" {
		return "", errors.New("wechat code is required")
	}
	if resolver != nil {
		return resolver(code)
	}
	return "demo-" + code, nil
}

func resolveWechatPhone(resolver func(string) (string, error), phoneCode string) (string, error) {
	if phoneCode == "" {
		return "", errors.New("手机号授权已失效，请重新授权")
	}
	if resolver == nil {
		return "", errors.New("本地演示模式下请使用演示账号登录")
	}
	return resolver(phoneCode)
}

func (s *MemoryStore) LoginWithAdminPassword(phone, password string) (learning.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.loginWithAdminPasswordUnlocked(phone, password)
	return result1, err
}

func (s *MemoryStore) ChangePassword(operator string, principal learning.Principal, req learning.PasswordChangeRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.changePasswordUnlocked(operator, principal, req)
}

func (s *MemoryStore) ResetPassword(operator string, principal learning.Principal, userID string) (learning.PasswordResetResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.resetPasswordUnlocked(operator, principal, userID)
	return result1, err
}

func (s *MemoryStore) RecordSecurityEvent(operator, action, target, detail string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordSecurityEventUnlocked(operator, action, target, detail)
}

func (s *MemoryStore) LoginWithDemoStudentPassword(phone, password string) (learning.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.loginWithDemoStudentPasswordUnlocked(phone, password)
	return result1, err
}

func (s *MemoryStore) PrincipalByUserID(userID string) (learning.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.principalByUserIDUnlocked(userID)
	return result1, err
}

func (s *MemoryStore) StudentAccounts(principal learning.Principal) ([]learning.StudentAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.studentAccountsUnlocked(principal)
}

func (s *MemoryStore) RequestAdditionalStudent(principal learning.Principal, req learning.StudentAccountAddRequest) (learning.StudentAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requestAdditionalStudentUnlocked(principal, req)
}

func (s *MemoryStore) SwitchStudentAccount(principal learning.Principal, studentID string) (learning.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.switchStudentAccountUnlocked(principal, studentID)
}

func (s *MemoryStore) Notices(principal learning.Principal) []learning.Notice {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.noticesUnlocked(principal)
	return result
}

func (s *MemoryStore) CreateNotice(operator string, principal learning.Principal, req learning.NoticeCreateRequest) (learning.Notice, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.Notice, error) {
		return work.createNoticeUnlocked(operator, principal, req)
	}, refreshNotice)
}

func (s *MemoryStore) RetryNotice(operator string, principal learning.Principal, id string) (learning.Notice, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.Notice, error) {
		return work.retryNoticeUnlocked(operator, principal, id)
	}, refreshNotice)
}

func (s *MemoryStore) Logs() []learning.OperationLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.logsUnlocked()
	return result
}

func (s *MemoryStore) Banners() []learning.Banner {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bannersUnlocked()
}

func (s *MemoryStore) ActiveStudentBanners() []learning.Banner {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeStudentBannersUnlocked()
}

func (s *MemoryStore) CreateBanner(operator string, req learning.BannerUpsertRequest) (learning.Banner, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createBannerUnlocked(operator, req)
}

func (s *MemoryStore) UpdateBanner(operator string, id string, req learning.BannerUpsertRequest) (learning.Banner, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateBannerUnlocked(operator, id, req)
}

func (s *MemoryStore) DeleteBanner(operator string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteBannerUnlocked(operator, id)
}

func (s *MemoryStore) Settings() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.settingsUnlocked()
	return result
}

func (s *MemoryStore) UpdateSetting(operator string, req learning.SettingUpdateRequest) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateSettingUnlocked(operator, req)
	return result1, err
}

func (s *MemoryStore) Subjects() []learning.SubjectMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subjectsUnlocked()
}

func (s *MemoryStore) UpdateSubjectMetadata(operator, id string, req learning.SubjectMetadataUpdateRequest) (learning.SubjectMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateSubjectMetadataUnlocked(operator, id, req)
}

func (s *MemoryStore) GradeSubjects() []learning.GradeSubjectMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gradeSubjectsUnlocked()
}

func (s *MemoryStore) UpdateGradeSubjects(operator string, req learning.GradeSubjectCatalogUpdateRequest) ([]learning.GradeSubjectMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateGradeSubjectsUnlocked(operator, req)
}

func (s *MemoryStore) Availability(principal learning.Principal, ownerType, ownerID string) ([]learning.AvailabilitySlot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.availabilityUnlocked(principal, ownerType, ownerID)
	return result1, err
}

func (s *MemoryStore) AvailabilityOverview(principal learning.Principal) []learning.AvailabilitySlot {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.availabilityOverviewUnlocked(principal)
	return result
}

func (s *MemoryStore) SaveAvailability(operator string, principal learning.Principal, req learning.AvailabilityUpsertRequest) ([]learning.AvailabilitySlot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.saveAvailabilityUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) ScheduleCandidates(principal learning.Principal, req learning.ScheduleCandidateRequest) ([]learning.ScheduleCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.scheduleCandidatesUnlocked(principal, req)
	return result1, err
}

func (s *MemoryStore) ScheduleClasses(principal learning.Principal) []learning.ScheduleClass {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.scheduleClassesUnlocked(principal)
	return result
}

func (s *MemoryStore) CreateScheduleClass(operator string, principal learning.Principal, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
		return work.createScheduleClassUnlocked(operator, principal, req)
	}, nil)
}

func (s *MemoryStore) UpdateScheduleClass(operator string, principal learning.Principal, id string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
		return work.updateScheduleClassUnlocked(operator, principal, id, req)
	}, nil)
}

func (s *MemoryStore) CancelScheduleClass(operator string, principal learning.Principal, id string) (learning.ScheduleClass, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
		return work.cancelScheduleClassUnlocked(operator, principal, id)
	}, nil)
}

func (s *MemoryStore) ReviewScheduleClass(operator string, principal learning.Principal, id string, approve bool, reason string) (learning.ScheduleClass, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
		return work.reviewScheduleClassUnlocked(operator, principal, id, approve, reason)
	}, nil)
}

func (s *MemoryStore) PendingScheduleClasses(principal learning.Principal) []learning.ScheduleClass {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingScheduleClassesUnlocked(principal)
}

func (s *MemoryStore) LessonFeedbacks(principal learning.Principal, classID string) ([]learning.LessonFeedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lessonFeedbacksUnlocked(principal, classID)
}

func (s *MemoryStore) UpsertLessonFeedback(operator string, principal learning.Principal, classID string, req learning.LessonFeedbackUpsertRequest) (learning.LessonFeedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upsertLessonFeedbackUnlocked(operator, principal, classID, req)
}

func (s *MemoryStore) Dashboard(principal learning.Principal) learning.DashboardOverview {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.dashboardUnlocked()
	if hasRole(principal.Roles, learning.RoleTeacher) && !hasRole(principal.Roles, learning.RoleOpsStaff) && !hasRole(principal.Roles, learning.RoleCampusAdmin) && !hasRole(principal.Roles, learning.RoleSuperAdmin) {
		studentIDs := map[string]bool{}
		for _, assignment := range s.tutoringAssignments {
			if assignment.TeacherID == principal.UserID && assignment.Status == learning.TutoringAssignmentActive {
				studentIDs[assignment.StudentID] = true
			}
		}
		result.OpenedStudents = len(studentIDs)
		result.PendingReviews = len(s.reviewsUnlocked(principal))
		result.PackageCount = len(s.coursesUnlocked(principal))
		result.ExpiringStudents = 0
		result.UnpublishedFiles = 0
		result.MaterialViews = 0
		for _, material := range s.materialsUnlocked(principal) {
			result.MaterialViews += material.ViewCount
		}
	}
	return result
}

func (s *MemoryStore) SystemReadiness() learning.SystemReadiness {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.systemReadinessUnlocked()
	return result
}

func (s *MemoryStore) Packages() []learning.Package {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.packagesUnlocked()
	return result
}

func (s *MemoryStore) CreatePackage(operator string, req learning.PackageUpsertRequest) (learning.Package, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createPackageUnlocked(operator, req)
	return result1, err
}

func (s *MemoryStore) UpdatePackage(operator string, id string, req learning.PackageUpsertRequest) (learning.Package, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updatePackageUnlocked(operator, id, req)
	return result1, err
}

func (s *MemoryStore) LearningSpaces() []learning.LearningSpace {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.learningSpacesUnlocked()
	return result
}

func (s *MemoryStore) AdminStaff() []learning.AdminStaff {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.adminStaffUnlocked()
	return result
}

func (s *MemoryStore) CreateAdminStaff(operator string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createAdminStaffUnlocked(operator, req)
	return result1, err
}

func (s *MemoryStore) UpdateAdminStaff(operator string, principal learning.Principal, id string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateAdminStaffUnlocked(operator, principal, id, req)
	return result1, err
}

func (s *MemoryStore) Teachers(principal learning.Principal) []learning.Teacher {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.teachersUnlocked(principal)
	return result
}

func (s *MemoryStore) CreateTeacher(operator string, principal learning.Principal, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createTeacherUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) UpdateTeacher(operator string, principal learning.Principal, id string, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateTeacherUnlocked(operator, principal, id, req)
	return result1, err
}

func (s *MemoryStore) Students(principal learning.Principal, query learning.StudentQuery) []learning.Student {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.studentsUnlocked(principal, query)
	return result
}

func (s *MemoryStore) StudentDetail(principal learning.Principal, id string) (learning.StudentDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentDetailUnlocked(principal, id)
	return result1, err
}

func (s *MemoryStore) CreateStudent(operator string, principal learning.Principal, req learning.StudentUpsertRequest) (learning.Student, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createStudentUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) UpdateStudent(operator string, principal learning.Principal, id string, req learning.StudentUpsertRequest) (learning.Student, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateStudentUnlocked(operator, principal, id, req)
	return result1, err
}

func (s *MemoryStore) UpdateStudentProfile(operator string, principal learning.Principal, req learning.StudentProfileUpdateRequest) (learning.Student, error) {
	operator = operator
	principal = principal
	if principal.StudentID == "" {
		return learning.Student{}, errors.New("student account is not bound")
	}
	var err error
	req, requiresBasicProfile, err := normalizeStudentProfileUpdateRequest(req)
	if err != nil {
		return learning.Student{}, err
	}
	resolvedPhone := ""
	if req.PhoneCode != "" {
		s.mu.Lock()
		phoneResolver := s.phoneResolver
		s.mu.Unlock()
		resolvedPhone, err = resolveWechatPhone(phoneResolver, req.PhoneCode)
		if err != nil {
			return learning.Student{}, err
		}
		resolvedPhone = strings.TrimSpace(resolvedPhone)
		if resolvedPhone == "" {
			return learning.Student{}, errors.New("手机号授权已失效，请重新授权")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateStudentProfileResolvedUnlocked(operator, principal, req, requiresBasicProfile, resolvedPhone)
	return result1, err
}

func (s *MemoryStore) RemindStudent(operator string, principal learning.Principal, id string) (learning.StudentRemindResult, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.StudentRemindResult, error) {
		return work.remindStudentUnlocked(operator, principal, id)
	}, refreshStudentReminder)
}

func (s *MemoryStore) CleanupTestStudents(operator string, principal learning.Principal) (learning.StudentCleanupResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanupTestStudentsUnlocked(operator, principal)
}

func (s *MemoryStore) ImportStudents(operator string, principal learning.Principal, rows []learning.StudentUpsertRequest) (learning.StudentImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.importStudentsUnlocked(operator, principal, rows)
}

func (s *MemoryStore) StudentGrants(principal learning.Principal, id string) ([]learning.StudentGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentGrantsUnlocked(principal, id)
	return result1, err
}

func (s *MemoryStore) StudentLearningRecords(principal learning.Principal, id string) ([]learning.StudentLearningRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentLearningRecordsUnlocked(principal, id)
	return result1, err
}

func (s *MemoryStore) StudentScores(principal learning.Principal, id string) ([]learning.StudentScoreSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentScoresUnlocked(principal, id)
	return result1, err
}

func (s *MemoryStore) StudentOwnScores(principal learning.Principal) ([]learning.StudentScoreSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentOwnScoresUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) CreateStudentScore(operator string, principal learning.Principal, studentID string, req learning.StudentScoreUpsertRequest) (learning.StudentScoreRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createStudentScoreUnlocked(operator, principal, studentID, req)
	return result1, err
}

func (s *MemoryStore) UpdateStudentScore(operator string, principal learning.Principal, studentID string, scoreID string, req learning.StudentScoreUpsertRequest) (learning.StudentScoreRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.updateStudentScoreUnlocked(operator, principal, studentID, scoreID, req)
	return result1, err
}

func (s *MemoryStore) ConnectDatabase(dsn string) error {
	return s.connectPrepared(dsn, false)
}

func (s *MemoryStore) StudentHome(principal learning.Principal) (learning.StudentHome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentHomeUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) StudentRecommendations(principal learning.Principal) ([]learning.StudentPackageRecommendation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentRecommendationsUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) ConfirmStudentSubscription(operator string, principal learning.Principal, req learning.StudentSubscriptionRequest) (learning.SubscriptionReminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.confirmStudentSubscriptionUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) StudentStudy(principal learning.Principal) (learning.StudentStudyBoard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentStudyUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) StudentTasks(principal learning.Principal) ([]learning.StudentTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentTasksUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) StudentSchedule(principal learning.Principal) ([]learning.ScheduleClass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentScheduleUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) StudentCourseDetail(principal learning.Principal, courseID string) (learning.StudentCourseDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentCourseDetailUnlocked(principal, courseID)
	return result1, err
}

func (s *MemoryStore) StudentGrowth(principal learning.Principal) ([]learning.StudentLearningRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentGrowthUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) StudentBadges(principal learning.Principal) ([]learning.Badge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentBadgesUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) StudentFavorites(principal learning.Principal) ([]learning.Favorite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentFavoritesUnlocked(principal)
	return result1, err
}

func (s *MemoryStore) AddFavorite(operator string, principal learning.Principal, req learning.FavoriteRequest) (learning.Favorite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.addFavoriteUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) RemoveFavorite(operator string, principal learning.Principal, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeFavoriteUnlocked(operator, principal, id)
}

func (s *MemoryStore) StudentMaterial(principal learning.Principal, materialID string) (learning.Material, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentMaterialUnlocked(principal, materialID)
	return result1, err
}

func (s *MemoryStore) StudentMaterialPreviewFile(principal learning.Principal, materialID string) (learning.FileAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentMaterialPreviewFileUnlocked(principal, materialID)
	return result1, err
}

func (s *MemoryStore) StudentHomework(principal learning.Principal, homeworkID string) (learning.Homework, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentHomeworkUnlocked(principal, homeworkID)
	return result1, err
}

func (s *MemoryStore) RecordStudentSecurityEvent(operator string, principal learning.Principal, req learning.SecurityEventRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordStudentSecurityEventUnlocked(operator, principal, req)
}

func (s *MemoryStore) CreateSubmission(operator string, principal learning.Principal, req learning.SubmissionRequest) (learning.Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.createSubmissionUnlocked(operator, principal, req)
	return result1, err
}

func (s *MemoryStore) StudentSubmission(principal learning.Principal, id string) (learning.Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result1, err := s.studentSubmissionUnlocked(principal, id)
	return result1, err
}

func (s *MemoryStore) UseWechatAPI(appID, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return errors.New("wechat configuration is startup-only")
	}
	s.useWechatAPIUnlocked(appID, secret)
	return nil
}

func (s *MemoryStore) UseOfficialAccountAPI(appID, secret, templateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return errors.New("official account configuration is startup-only")
	}
	s.useOfficialAccountAPIUnlocked(appID, secret, templateID)
	return nil
}

func (s *MemoryStore) UseMiniProgramSubscribeTemplates(templateIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return errors.New("mini program subscribe configuration is startup-only")
	}
	s.useMiniProgramSubscribeTemplatesUnlocked(templateIDs)
	return nil
}
