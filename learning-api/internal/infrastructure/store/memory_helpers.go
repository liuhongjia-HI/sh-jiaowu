package store

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) findLearningSpace(id string) (learningSpace, bool) {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return space, true
		}
	}
	return learningSpace{}, false
}

// learningSpaceMatches 按 年级/学科/学期 匹配学习空间。
// 不再比较学年：学习空间是跨学年复用的课程目录（同一个五年级英文 S1 阶段，
// 内容每年可能会更新，但槽位本身不需要每年建一份新的），学年只属于套餐/开通，
// 由 Package.AcademicYear / GrantResult.AcademicYear 承载。
func (s *MemoryStore) learningSpaceMatches(id, grade, subject, semester string) bool {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return space.Grade == grade && subjectsMatch(space.Subject, subject) && space.Semester == semester
		}
	}
	return false
}

func (s *MemoryStore) courseForUpload(principal learning.Principal, courseID, learningSpaceID string) (learning.Course, error) {
	for _, course := range s.courses {
		if course.ID != courseID {
			continue
		}
		if learningSpaceID != "" && course.LearningSpaceID != learningSpaceID {
			return learning.Course{}, errors.New("请选择正确的课程范围")
		}
		if !canSeeCourse(principal, course) {
			return learning.Course{}, errors.New("不能上传到未负责的课程")
		}
		if course.Status != learning.StatusEnabled {
			return learning.Course{}, errors.New("课程已停用，不能上传")
		}
		return course, nil
	}
	return learning.Course{}, errors.New("请选择课程")
}

func (s *MemoryStore) contentTypesForPackage(packageID string) []string {
	out := make([]string, 0)
	for _, item := range s.contentTypes {
		if item.PackageID == packageID {
			out = appendUnique(out, item.ContentType)
		}
	}
	if containsString(out, "course") {
		out = appendUnique(out, "handout", "question", "download")
	}
	return out
}

func (s *MemoryStore) contentTypeLabelsForPackage(packageID string) []string {
	labels := make([]string, 0)
	for _, value := range s.contentTypesForPackage(packageID) {
		labels = appendUnique(labels, contentTypeLabel(value))
	}
	return labels
}

func (s *MemoryStore) packageOpensContent(pkg learning.Package, learningSpaceID, contentType string) bool {
	return containsString(s.learningSpaceIDsForPackage(pkg.ID), learningSpaceID) && containsString(s.contentTypesForPackage(pkg.ID), contentType)
}

func (s *MemoryStore) grantOpensSpace(grantID, learningSpaceID string) bool {
	return containsString(s.learningSpaceIDsForGrant(grantID), learningSpaceID)
}

func (s *MemoryStore) syncSpaceAccessForGrant(grant packageGrant) {
	for _, relation := range s.packageSpaces {
		if relation.PackageID != grant.PackageID || !s.learningSpaceEnabled(relation.LearningSpaceID) {
			continue
		}
		s.spaceAccess = append(s.spaceAccess, learningSpaceAccess{
			StudentID:       grant.StudentID,
			LearningSpaceID: relation.LearningSpaceID,
			PackageGrantID:  grant.ID,
			StartsAt:        grant.StartsAt,
			EndsAt:          grantEndsAt(grant),
			Status:          grant.Status,
		})
	}
}

func (s *MemoryStore) replaceSpaceAccessForGrant(grant packageGrant) {
	next := make([]learningSpaceAccess, 0, len(s.spaceAccess))
	for _, access := range s.spaceAccess {
		if access.PackageGrantID != grant.ID {
			next = append(next, access)
		}
	}
	s.spaceAccess = next
	s.syncSpaceAccessForGrant(grant)
}

func (s *MemoryStore) materialsForCourses(courses []string) []learning.Material {
	out := make([]learning.Material, 0)
	for _, material := range s.materials {
		if containsString(courses, material.Course) {
			out = append(out, s.decorateMaterial(material))
		}
	}
	return out
}

func (s *MemoryStore) homeworkForCourses(courses []string) []learning.Homework {
	out := make([]learning.Homework, 0)
	for _, item := range s.homework {
		if homeworkVisible(item.Status) && containsString(courses, item.Course) {
			item = cloneHomework(item)
			item.TagCode = contentTagCodeOrInferred(item.TagCode, item.Title, item.FileName)
			out = append(out, item)
		}
	}
	return out
}

func homeworkVisible(status string) bool {
	status = strings.TrimSpace(status)
	return status != string(learning.StatusDraft) && status != string(learning.StatusDisabled)
}

func isContentStatus(status learning.Status) bool {
	return status == learning.StatusEnabled || status == learning.StatusDraft || status == learning.StatusDisabled
}

func normalizeMaterialStatus(status learning.Status) learning.Status {
	switch strings.TrimSpace(string(status)) {
	case "", "已发布":
		return learning.StatusEnabled
	default:
		return learning.Status(strings.TrimSpace(string(status)))
	}
}

func materialPublished(status learning.Status) bool {
	return normalizeMaterialStatus(status) == learning.StatusEnabled
}

func publishStatus(status learning.Status) string {
	if status == learning.StatusEnabled {
		return "已发布"
	}
	return string(status)
}

func (s *MemoryStore) prependLog(operator, action, target string) {
	s.prependLogDetail(operator, action, target, "")
}

func auditChangeDetail(before, after any) string {
	payload := map[string]any{
		"before": before,
		"after":  after,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func packageAuditSnapshot(item learning.Package) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"name":             item.Name,
		"academicYear":     item.AcademicYear,
		"grade":            item.Grade,
		"semester":         item.Semester,
		"subject":          item.Subject,
		"level":            item.Level,
		"phaseScope":       item.PhaseScope,
		"packageType":      item.PackageType,
		"summary":          item.Summary,
		"trialEnabled":     item.TrialEnabled,
		"learningSpaceIds": item.LearningSpaceIDs,
		"contentTypeCodes": item.ContentTypeCodes,
		"status":           item.Status,
	}
}

func adminStaffAuditSnapshot(item learning.AdminStaff) map[string]any {
	return map[string]any{
		"id":            item.ID,
		"name":          item.Name,
		"phone":         item.Phone,
		"role":          item.Role,
		"campusId":      item.CampusID,
		"accountStatus": item.AccountStatus,
		"remark":        item.Remark,
	}
}

func teacherAuditSnapshot(item learning.Teacher) map[string]any {
	return map[string]any{
		"id":                item.ID,
		"name":              item.Name,
		"phone":             item.Phone,
		"campusId":          item.CampusID,
		"learningSpaceIds":  item.LearningSpaceIDs,
		"canUploadHandout":  item.CanUploadHandout,
		"canUploadQuestion": item.CanUploadQuestion,
		"canReview":         item.CanReview,
		"accountStatus":     item.AccountStatus,
		"remark":            item.Remark,
	}
}

func studentAuditSnapshot(item learning.Student) map[string]any {
	return map[string]any{
		"id":                    item.ID,
		"name":                  item.Name,
		"nickname":              item.Nickname,
		"avatarUrl":             item.AvatarURL,
		"phone":                 item.Phone,
		"grade":                 item.Grade,
		"schoolName":            item.SchoolName,
		"guardianName":          item.GuardianName,
		"officialAccountOpenId": item.OfficialAccountOpenID,
		"accountStatus":         item.AccountStatus,
		"remark":                item.Remark,
	}
}

func courseAuditSnapshot(item learning.Course) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"name":            item.Name,
		"learningSpaceId": item.LearningSpaceID,
		"chapterCount":    item.ChapterCount,
		"status":          item.Status,
	}
}

func materialAuditSnapshot(item learning.Material) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"title":           item.Title,
		"courseId":        item.CourseID,
		"learningSpaceId": item.LearningSpaceID,
		"chapter":         item.Chapter,
		"status":          item.Status,
		"publishStatus":   item.PublishStatus,
	}
}

func homeworkAuditSnapshot(item learning.Homework) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"title":           item.Title,
		"courseId":        item.CourseID,
		"learningSpaceId": item.LearningSpaceID,
		"chapter":         item.Chapter,
		"deadline":        item.Deadline,
		"status":          item.Status,
		"publishStatus":   item.PublishStatus,
	}
}

func scheduleClassAuditSnapshot(item learning.ScheduleClass) map[string]any {
	studentIDs := make([]string, 0, len(item.Students))
	for _, student := range item.Students {
		studentIDs = append(studentIDs, student.ID)
	}
	return map[string]any{
		"id":              item.ID,
		"name":            item.Name,
		"courseId":        item.CourseID,
		"teacherId":       item.TeacherID,
		"campusId":        item.CampusID,
		"roomName":        item.RoomName,
		"classType":       item.ClassType,
		"capacity":        item.Capacity,
		"durationMinutes": item.DurationMinutes,
		"dayOfWeek":       item.DayOfWeek,
		"startTime":       item.StartTime,
		"endTime":         item.EndTime,
		"startDate":       item.StartDate,
		"endDate":         item.EndDate,
		"academicYear":    item.AcademicYear,
		"semester":        item.Semester,
		"studentIds":      studentIDs,
		"status":          item.Status,
	}
}

func (s *MemoryStore) prependLogDetail(operator, action, target, detail string) {
	audit := parseAuditOperator(operator)
	if detail != "" {
		audit.Detail = strings.TrimSpace(detail)
	}
	s.logs = append([]learning.OperationLog{{
		ID:         newOperationLogID(),
		Operator:   audit.Name,
		OperatorID: audit.ID,
		IP:         audit.IP,
		UserAgent:  audit.UserAgent,
		Action:     action,
		Target:     target,
		Detail:     audit.Detail,
		Time:       time.Now().Format("2006-01-02 15:04:05"),
	}}, s.logs...)
}

func newOperationLogID() string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "log-" + time.Now().Format("20060102150405.000000000")
	}
	suffix := make([]byte, len(raw))
	for i, b := range raw {
		suffix[i] = alphabet[int(b)%len(alphabet)]
	}
	return "log-" + time.Now().Format("20060102150405.000000000") + "-" + string(suffix)
}

type auditOperatorInfo struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	Detail    string `json:"detail"`
}

func parseAuditOperator(value string) auditOperatorInfo {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "audit:") {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "audit:"))
		if err == nil {
			var info auditOperatorInfo
			if json.Unmarshal(raw, &info) == nil {
				info.Name = strings.TrimSpace(info.Name)
				if info.Name == "" {
					info.Name = "本地开发"
				}
				return info
			}
		}
	}
	if value == "" {
		value = "本地开发"
	}
	return auditOperatorInfo{Name: value}
}

func normalizeStudentRequest(req learning.StudentUpsertRequest, allowStatus bool) (learning.StudentUpsertRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Grade = strings.TrimSpace(req.Grade)
	req.SchoolName = strings.TrimSpace(req.SchoolName)
	req.GuardianName = strings.TrimSpace(req.GuardianName)
	req.OfficialAccountOpenID = strings.TrimSpace(req.OfficialAccountOpenID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	if req.Name == "" {
		return req, errors.New("请输入学生姓名")
	}
	if req.Phone == "" {
		return req, errors.New("请输入手机号")
	}
	if req.Grade == "" {
		return req, errors.New("请选择年级")
	}
	if len([]rune(req.SchoolName)) > 64 {
		return req, errors.New("学校名称最多 64 个字")
	}
	if !allowStatus || req.AccountStatus == "" {
		req.AccountStatus = "正常"
	}
	if req.AccountStatus != "正常" && req.AccountStatus != "停用" && req.AccountStatus != "待提醒" && req.AccountStatus != "待审核" {
		return req, errors.New("账号状态不正确")
	}
	return req, nil
}

func validateNewPassword(password string) error {
	if len(password) < 8 {
		return errors.New("新密码至少 8 位")
	}
	hasLetter := false
	hasDigit := false
	for _, ch := range password {
		if ch >= '0' && ch <= '9' {
			hasDigit = true
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			hasLetter = true
		}
	}
	if !hasLetter || !hasDigit {
		return errors.New("新密码需同时包含字母和数字")
	}
	return nil
}

func generateTemporaryPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return fmt.Sprintf("Starline@%s1", string(out)), nil
}

func matchesStudentQuery(student learning.Student, query learning.StudentQuery) bool {
	// 学生姓名和手机号检索统一按小写比较，避免用户因大小写差异搜不到结果。
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	if keyword != "" && !strings.Contains(strings.ToLower(student.Name), keyword) && !strings.Contains(strings.ToLower(student.Phone), keyword) {
		return false
	}
	if query.Grade != "" && student.Grade != query.Grade {
		return false
	}
	if query.AccountStatus != "" && student.AccountStatus != query.AccountStatus {
		return false
	}
	if query.LearningStatus != "" && student.LearningStatus != query.LearningStatus {
		return false
	}
	if query.PackageState == "已开通" && len(student.OpenedPackages) == 0 {
		return false
	}
	if query.PackageState == "未开通" && len(student.OpenedPackages) > 0 {
		return false
	}
	if query.FollowUpState != "" && student.FollowUpStatus != query.FollowUpState {
		return false
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cleanPhrases(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = appendUnique(out, value)
	}
	return out
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if value == "" || seen[value] {
			continue
		}
		values = append(values, value)
		seen[value] = true
	}
	return values
}

func principalFromUser(user learning.User) learning.Principal {
	return learning.Principal{
		UserID:             user.ID,
		Name:               user.Name,
		Phone:              user.Phone,
		StudentID:          user.StudentID,
		CampusID:           user.CampusID,
		Roles:              append([]learning.Role(nil), user.Roles...),
		MustChangePassword: user.MustChangePassword,
		TokenVersion:       user.TokenVersion,
		CampusScopes:       append([]string(nil), user.CampusScopes...),
		LearningSpaceIDs:   append([]string(nil), user.LearningSpaceIDs...),
		CanUploadHandout:   user.CanUploadHandout,
		CanUploadQuestion:  user.CanUploadQuestion,
		CanReview:          user.CanReview,
	}
}

func (s *MemoryStore) teacherFromUser(user learning.User) learning.Teacher {
	bindStatus := "待绑定"
	if strings.TrimSpace(user.OpenID) != "" {
		bindStatus = "已绑定"
	}
	return learning.Teacher{
		ID:                user.ID,
		Name:              user.Name,
		Phone:             user.Phone,
		CampusID:          user.CampusID,
		LearningSpaceIDs:  append([]string(nil), user.LearningSpaceIDs...),
		LearningSpaces:    s.learningSpaceNames(user.LearningSpaceIDs),
		Grades:            s.learningSpaceGrades(user.LearningSpaceIDs),
		Subjects:          s.learningSpaceSubjects(user.LearningSpaceIDs),
		CanUploadHandout:  user.CanUploadHandout,
		CanUploadQuestion: user.CanUploadQuestion,
		CanReview:         user.CanReview,
		AccountStatus:     user.AccountStatus,
		BindStatus:        bindStatus,
		Remark:            user.Remark,
		ActiveClassCount:  s.activeClassCountForTeacher(user.ID),
	}
}

// activeClassCountForTeacher 统计一个老师名下还没结束、也没取消的排课数量。
// “没结束”按 EndDate 判断：空值是长期排课，没到 EndDate 那天都算在期内，
// 只有明确过了 EndDate 的才算已经结束，不用再关心这个老师还带着它。
func (s *MemoryStore) activeClassCountForTeacher(teacherID string) int {
	today := time.Now().Format("2006-01-02")
	count := 0
	for _, item := range s.scheduleClasses {
		if item.TeacherID != teacherID || item.Status == "已取消" {
			continue
		}
		if item.EndDate != "" && item.EndDate < today {
			continue
		}
		count++
	}
	return count
}

func adminStaffFromUser(user learning.User) learning.AdminStaff {
	bindStatus := "待绑定"
	if strings.TrimSpace(user.OpenID) != "" {
		bindStatus = "已绑定"
	}
	return learning.AdminStaff{
		ID:            user.ID,
		Name:          user.Name,
		Phone:         user.Phone,
		Role:          primaryAdminRole(user.Roles),
		CampusID:      user.CampusID,
		AccountStatus: user.AccountStatus,
		BindStatus:    bindStatus,
		Remark:        user.Remark,
	}
}

func normalizeAdminStaffRequest(req learning.AdminStaffUpsertRequest, allowStatus bool) (learning.AdminStaffUpsertRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Role = learning.Role(strings.TrimSpace(string(req.Role)))
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)

	if req.Name == "" {
		return req, errors.New("请输入管理人员姓名")
	}
	if req.Phone == "" {
		return req, errors.New("请输入手机号")
	}
	if !isAdminRole(req.Role) {
		return req, errors.New("请选择正确的后台岗位")
	}
	if req.Role == learning.RoleCampusAdmin && req.CampusID == "" {
		return req, errors.New("校区管理员需要填写校区")
	}
	if req.Role == learning.RoleSuperAdmin {
		req.CampusID = ""
	}
	if !allowStatus || req.AccountStatus == "" {
		req.AccountStatus = "正常"
	}
	if req.AccountStatus != "正常" && req.AccountStatus != "停用" {
		return req, errors.New("账号状态不正确")
	}
	return req, nil
}

func (s *MemoryStore) normalizeTeacherRequest(principal learning.Principal, req learning.TeacherUpsertRequest, allowStatus bool) (learning.TeacherUpsertRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	req.LearningSpaceIDs = cleanPhrases(req.LearningSpaceIDs)

	if req.Name == "" {
		return req, errors.New("请输入教师姓名")
	}
	if req.Phone == "" {
		return req, errors.New("请输入手机号")
	}
	if len(req.LearningSpaceIDs) == 0 {
		return req, errors.New("请选择教师负责范围")
	}
	for _, spaceID := range req.LearningSpaceIDs {
		if !s.learningSpaceExists(spaceID) {
			return req, errors.New("教师负责范围不存在")
		}
	}
	if !req.CanUploadHandout && !req.CanUploadQuestion && !req.CanReview {
		return req, errors.New("请至少选择一项教师权限")
	}
	if !allowStatus || req.AccountStatus == "" {
		req.AccountStatus = "正常"
	}
	if req.AccountStatus != "正常" && req.AccountStatus != "停用" {
		return req, errors.New("账号状态不正确")
	}
	if hasRole(principal.Roles, learning.RoleCampusAdmin) && !hasRole(principal.Roles, learning.RoleSuperAdmin) {
		if principal.CampusID == "" {
			return req, errors.New("当前管理员未绑定校区")
		}
		if req.CampusID == "" {
			req.CampusID = principal.CampusID
		}
		if req.CampusID != principal.CampusID {
			return req, errors.New("不能管理其他校区教师")
		}
	}
	if req.CampusID == "" {
		req.CampusID = "campus-main"
	}
	return req, nil
}

func (s *MemoryStore) userPhoneExists(currentUserID, phone string) bool {
	for _, user := range s.users {
		if user.ID != currentUserID && user.Phone == phone {
			return true
		}
	}
	return false
}

func (s *MemoryStore) activeSuperAdminCount() int {
	count := 0
	for _, user := range s.users {
		if user.AccountStatus == "正常" && hasRole(user.Roles, learning.RoleSuperAdmin) {
			count++
		}
	}
	return count
}

func canBindByPhone(user learning.User) bool {
	return hasRole(user.Roles, learning.RoleTeacher) || hasRole(user.Roles, learning.RoleStudent) || isAdminStaffUser(user)
}

func canRebindByPhone(user learning.User, openID string, realWechatLogin bool) bool {
	if !canBindByPhone(user) {
		return false
	}
	existingOpenID := strings.TrimSpace(user.OpenID)
	nextOpenID := strings.TrimSpace(openID)
	return existingOpenID == "" || existingOpenID == nextOpenID || (realWechatLogin && strings.HasPrefix(existingOpenID, "demo-"))
}

func (s *MemoryStore) validateStudentWechatBinding(user learning.User, openID string, req learning.WechatLoginRequest) error {
	student, ok := s.findRawStudent(user.StudentID)
	if !ok {
		return errors.New("学生档案不存在，请联系老师确认")
	}
	if req.StudentName == "" || req.Grade == "" {
		return errors.New("请填写学生姓名和年级后再绑定")
	}
	if student.Name != "" && student.Name != req.StudentName {
		return errors.New("学生姓名与后台档案不一致，请联系老师确认")
	}
	if student.EnrollmentGrade != "" && student.EnrollmentGrade != "待完善" {
		// 对比的是按学年滚动推导出来的当前年级，不是入学时的年级快照——
		// 学生登录时在选择器里填的是“我现在是几年级”，不是“我入学时是几年级”。
		currentGrade, _ := resolveGrade(student.EnrollmentAcademicYear, student.EnrollmentGrade, s.configuredAcademicYear())
		if currentGrade != req.Grade {
			return errors.New("年级与后台档案不一致，请联系老师确认")
		}
	}
	if student.SchoolName != "" && req.SchoolName != "" && student.SchoolName != req.SchoolName {
		return errors.New("学校与后台档案不一致，请联系老师确认")
	}
	existingOpenID := strings.TrimSpace(user.OpenID)
	nextOpenID := strings.TrimSpace(openID)
	if student.BindStatus == "已绑定" && existingOpenID != "" && existingOpenID != nextOpenID && !(s.wechatResolver != nil && strings.HasPrefix(existingOpenID, "demo-")) {
		return errors.New("该学生已绑定其他微信，请联系老师处理")
	}
	return nil
}

func (s *MemoryStore) applyStudentBindingProfile(studentID string, req learning.WechatLoginRequest) {
	for i := range s.students {
		if s.students[i].ID != studentID {
			continue
		}
		if req.StudentName != "" {
			s.students[i].Name = req.StudentName
		}
		if req.Grade != "" && s.students[i].EnrollmentGrade == "" {
			// 只在管理端从未录过入学年级时才由学生首次绑定时确立基准；
			// 已有基准的档案不接受绑定流程覆盖，校验逻辑早前已经比对过一致性。
			s.students[i].EnrollmentGrade = req.Grade
			s.students[i].EnrollmentAcademicYear = s.configuredAcademicYear()
		}
		if req.SchoolName != "" {
			s.students[i].SchoolName = req.SchoolName
		}
		s.students[i].BindStatus = "已绑定"
		s.syncStudentUser(s.students[i])
		return
	}
}

func (s *MemoryStore) findRawStudent(id string) (learning.Student, bool) {
	for _, student := range s.students {
		if student.ID == id {
			return student, true
		}
	}
	return learning.Student{}, false
}

func isAdminStaffUser(user learning.User) bool {
	for _, role := range user.Roles {
		if isAdminRole(role) {
			return true
		}
	}
	return false
}

func primaryAdminRole(roles []learning.Role) learning.Role {
	for _, role := range []learning.Role{learning.RoleSuperAdmin, learning.RoleCampusAdmin, learning.RoleOpsStaff} {
		if hasRole(roles, role) {
			return role
		}
	}
	return ""
}

func isAdminRole(role learning.Role) bool {
	return role == learning.RoleOpsStaff || role == learning.RoleCampusAdmin || role == learning.RoleSuperAdmin
}

func roleName(role learning.Role) string {
	switch role {
	case learning.RoleOpsStaff:
		return "运营教务"
	case learning.RoleCampusAdmin:
		return "校区管理员"
	case learning.RoleSuperAdmin:
		return "超级管理员"
	default:
		return string(role)
	}
}

func settingLabel(key string) string {
	switch key {
	case "grades":
		return "年级范围"
	case "semesters":
		return "学期设置"
	case "watermarkRule":
		return "水印规则"
	case "downloadPolicy":
		return "下载规则"
	case "academicCalendar":
		return "校历（按学年学期）"
	case "miniProgramDomainStatus":
		return "小程序域名备案"
	case "officialAccountBindingStatus":
		return "微信公众号关联"
	case "templateMessageStatus":
		return "模板消息审核"
	case "miniProgramSubscribeStatus":
		return "小程序订阅消息"
	case "productionApiDomain":
		return "生产接口域名"
	case "launchCampaign":
		return "开屏营销活动配置"
	default:
		return key
	}
}

func canManageTeacher(principal learning.Principal, user learning.User) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) {
		return true
	}
	if !hasRole(principal.Roles, learning.RoleCampusAdmin) {
		return false
	}
	return principal.CampusID != "" && user.CampusID == principal.CampusID
}

func isActiveTeacher(user learning.User) bool {
	return hasRole(user.Roles, learning.RoleTeacher) && user.AccountStatus == "正常"
}

func canResetPassword(principal learning.Principal, user learning.User) bool {
	if principal.UserID == user.ID {
		return false
	}
	if hasRole(principal.Roles, learning.RoleSuperAdmin) {
		return hasRole(user.Roles, learning.RoleTeacher) || isAdminStaffUser(user)
	}
	if hasRole(principal.Roles, learning.RoleCampusAdmin) {
		return hasRole(user.Roles, learning.RoleTeacher) && principal.CampusID != "" && principal.CampusID == user.CampusID
	}
	return false
}

func canManageCommercial(principal learning.Principal) bool {
	return hasRole(principal.Roles, learning.RoleOpsStaff) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleSuperAdmin)
}

func (s *MemoryStore) canSeeCommercialStudent(principal learning.Principal, studentID string) bool {
	student, ok := s.findStudent(studentID)
	if !ok {
		return false
	}
	return s.canSeeStudent(principal, student, s.coursesForStudent(student.ID))
}

func (s *MemoryStore) commercialOrderForWrite(principal learning.Principal, orderID string) (int, learning.CommercialOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return -1, learning.CommercialOrder{}, errors.New("请选择订单")
	}
	if !canManageCommercial(principal) {
		return -1, learning.CommercialOrder{}, errors.New("没有权限维护订单")
	}
	for index, order := range s.commercialOrders {
		if order.ID != orderID {
			continue
		}
		if !s.canSeeCommercialStudent(principal, order.StudentID) {
			return -1, learning.CommercialOrder{}, errors.New("没有权限维护该订单")
		}
		return index, order, nil
	}
	return -1, learning.CommercialOrder{}, errors.New("订单不存在")
}

func (s *MemoryStore) canSeeStudent(principal learning.Principal, student learning.Student, studentCourses []learning.Course) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if hasRole(principal.Roles, learning.RoleStudent) {
		return principal.StudentID == student.ID
	}
	if hasRole(principal.Roles, learning.RoleTeacher) {
		return teacherHasActiveTutoringAssignment(principal.UserID, student.ID, s.tutoringAssignments)
	}
	return false
}

func canSeeCourse(principal learning.Principal, course learning.Course) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if hasRole(principal.Roles, learning.RoleTeacher) {
		return containsString(principal.LearningSpaceIDs, course.LearningSpaceID)
	}
	return false
}

func canSeeQuestionScope(principal learning.Principal, grade, semester, subject string, spaces []learningSpace) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if !hasRole(principal.Roles, learning.RoleTeacher) {
		return false
	}
	for _, space := range spaces {
		if containsString(principal.LearningSpaceIDs, space.ID) && space.Grade == grade && space.Semester == semester && subjectsMatch(space.Subject, subject) && space.Status == learning.StatusEnabled {
			return true
		}
	}
	return false
}

func canEditQuestion(principal learning.Principal, item learning.QuestionBankItem) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	return hasRole(principal.Roles, learning.RoleTeacher) && item.OwnerTeacherID == principal.UserID
}

func canUploadHandout(principal learning.Principal) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	return hasRole(principal.Roles, learning.RoleTeacher) && principal.CanUploadHandout
}

func canUploadQuestion(principal learning.Principal) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	return hasRole(principal.Roles, learning.RoleTeacher) && principal.CanUploadQuestion
}

func canReviewHomework(principal learning.Principal) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	return hasRole(principal.Roles, learning.RoleTeacher) && principal.CanReview
}

func canSeeSubject(principal learning.Principal, subjects []string, value string) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	for _, subject := range subjects {
		if subjectTextContains(value, subject) {
			return true
		}
	}
	return false
}
