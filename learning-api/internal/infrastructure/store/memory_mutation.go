package store

import "starline/learning-api/internal/domain/learning"

// persistentMutation runs a state-changing operation against an isolated
// working copy. The live in-memory state is published only after the database
// transaction (including its operation log row) commits successfully.
func persistentMutation[T any](s *MemoryStore, change func(*MemoryStore) (T, error)) (T, error) {
	if s.db == nil {
		return change(s)
	}
	work := s.cloneForMutation()
	result, err := change(work)
	if err != nil {
		var zero T
		return zero, err
	}
	if err := s.persistMutation(work); err != nil {
		var zero T
		return zero, err
	}
	s.publishMutation(work)
	return result, nil
}

func persistentMutationError(s *MemoryStore, change func(*MemoryStore) error) error {
	_, err := persistentMutation(s, func(work *MemoryStore) (struct{}, error) {
		return struct{}{}, change(work)
	})
	return err
}

func (s *MemoryStore) persistMutation(after *MemoryStore) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := persistStateDeltaTx(tx, s, after); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *MemoryStore) cloneForMutation() *MemoryStore {
	work := &MemoryStore{
		wechatResolver:       s.wechatResolver,
		phoneResolver:        s.phoneResolver,
		officialNoticeSender: s.officialNoticeSender,
		officialAccountReady: s.officialAccountReady,
	}
	work.users = cloneUsers(s.users)
	work.packages = clonePackages(s.packages)
	work.students = cloneStudents(s.students)
	work.guardians = append([]learning.Guardian(nil), s.guardians...)
	work.guardianStudents = append([]learning.GuardianStudent(nil), s.guardianStudents...)
	work.learningSpaces = append([]learningSpace(nil), s.learningSpaces...)
	work.packageSpaces = append([]packageSpace(nil), s.packageSpaces...)
	work.contentTypes = append([]packageContentType(nil), s.contentTypes...)
	work.spaceAccess = append([]learningSpaceAccess(nil), s.spaceAccess...)
	work.courses = append([]learning.Course(nil), s.courses...)
	work.questionBank = make([]learning.QuestionBankItem, len(s.questionBank))
	for index, item := range s.questionBank {
		work.questionBank[index] = cloneQuestionBankItem(item)
	}
	work.materials = append([]learning.Material(nil), s.materials...)
	work.homework = make([]learning.Homework, len(s.homework))
	for index, item := range s.homework {
		work.homework[index] = cloneHomework(item)
	}
	work.fileAssets = cloneMap(s.fileAssets)
	work.previewJobs = append([]learning.PreviewJob(nil), s.previewJobs...)
	work.reviews = append([]learning.Review(nil), s.reviews...)
	work.notices = append([]learning.Notice(nil), s.notices...)
	work.logs = append([]learning.OperationLog(nil), s.logs...)
	work.settings = cloneMap(s.settings)
	work.grants = append([]packageGrant(nil), s.grants...)
	work.availability = append([]learning.AvailabilitySlot(nil), s.availability...)
	work.scheduleClasses = make([]learning.ScheduleClass, len(s.scheduleClasses))
	for index, item := range s.scheduleClasses {
		work.scheduleClasses[index] = cloneScheduleClass(item)
	}
	work.commercialOrders = append([]learning.CommercialOrder(nil), s.commercialOrders...)
	work.payments = append([]learning.PaymentRecord(nil), s.payments...)
	work.refunds = append([]learning.RefundRecord(nil), s.refunds...)
	work.contracts = append([]learning.ContractRecord(nil), s.contracts...)
	work.invoices = append([]learning.InvoiceRecord(nil), s.invoices...)
	work.lessonConsumptions = append([]learning.LessonConsumption(nil), s.lessonConsumptions...)
	work.renewalReminders = append([]learning.RenewalReminder(nil), s.renewalReminders...)
	work.parentNotices = append([]learning.ParentNotice(nil), s.parentNotices...)
	work.submissions = make(map[string]learning.Submission, len(s.submissions))
	for key, item := range s.submissions {
		work.submissions[key] = cloneSubmission(item)
	}
	work.favorites = cloneMap(s.favorites)
	work.subscriptionPreferences = make(map[string]learning.StudentSubscriptionPreference, len(s.subscriptionPreferences))
	for key, item := range s.subscriptionPreferences {
		item.TemplateIDs = cloneStrings(item.TemplateIDs)
		work.subscriptionPreferences[key] = item
	}
	work.scoreRecords = append([]learning.StudentScoreRecord(nil), s.scoreRecords...)
	work.banners = append([]learning.Banner(nil), s.banners...)
	work.miniProgramSubscribeTemplateIDs = cloneStrings(s.miniProgramSubscribeTemplateIDs)
	work.pendingNoticeDeliveries = append([]learning.Notice(nil), s.pendingNoticeDeliveries...)
	return work
}

func cloneMap[K comparable, V any](source map[K]V) map[K]V {
	out := make(map[K]V, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func cloneUsers(values []learning.User) []learning.User {
	out := make([]learning.User, len(values))
	for index, value := range values {
		value.Roles = append([]learning.Role(nil), value.Roles...)
		value.CampusScopes = cloneStrings(value.CampusScopes)
		value.LearningSpaceIDs = cloneStrings(value.LearningSpaceIDs)
		out[index] = value
	}
	return out
}

func clonePackages(values []learning.Package) []learning.Package {
	out := make([]learning.Package, len(values))
	for index, value := range values {
		value.LearningSpaceIDs = cloneStrings(value.LearningSpaceIDs)
		value.LearningSpaces = cloneStrings(value.LearningSpaces)
		value.ContentTypeCodes = cloneStrings(value.ContentTypeCodes)
		value.ContentTypes = cloneStrings(value.ContentTypes)
		out[index] = value
	}
	return out
}

func cloneStudents(values []learning.Student) []learning.Student {
	out := make([]learning.Student, len(values))
	for index, value := range values {
		value.OpenedPackages = cloneStrings(value.OpenedPackages)
		value.OpenedPackageRefs = append([]learning.StudentPackageRef(nil), value.OpenedPackageRefs...)
		out[index] = value
	}
	return out
}

func (s *MemoryStore) publishMutation(work *MemoryStore) {
	s.users = work.users
	s.packages = work.packages
	s.students = work.students
	s.guardians = work.guardians
	s.guardianStudents = work.guardianStudents
	s.learningSpaces = work.learningSpaces
	s.packageSpaces = work.packageSpaces
	s.contentTypes = work.contentTypes
	s.spaceAccess = work.spaceAccess
	s.courses = work.courses
	s.questionBank = work.questionBank
	s.materials = work.materials
	s.homework = work.homework
	s.fileAssets = work.fileAssets
	s.previewJobs = work.previewJobs
	s.reviews = work.reviews
	s.notices = work.notices
	s.logs = work.logs
	s.settings = work.settings
	s.grants = work.grants
	s.availability = work.availability
	s.scheduleClasses = work.scheduleClasses
	s.commercialOrders = work.commercialOrders
	s.payments = work.payments
	s.refunds = work.refunds
	s.contracts = work.contracts
	s.invoices = work.invoices
	s.lessonConsumptions = work.lessonConsumptions
	s.renewalReminders = work.renewalReminders
	s.parentNotices = work.parentNotices
	s.submissions = work.submissions
	s.favorites = work.favorites
	s.subscriptionPreferences = work.subscriptionPreferences
	s.scoreRecords = work.scoreRecords
	s.banners = work.banners
	s.miniProgramSubscribeTemplateIDs = work.miniProgramSubscribeTemplateIDs
	s.pendingNoticeDeliveries = work.pendingNoticeDeliveries
}
