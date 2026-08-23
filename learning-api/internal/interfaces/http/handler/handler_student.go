package handler

import (
	"encoding/csv"
	"io"
	"strings"

	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func (h *LearningHandler) Students(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	OK(c, h.service.Students(principal, learning.StudentQuery{
		Keyword:        strings.TrimSpace(c.Query("keyword")),
		Grade:          strings.TrimSpace(c.Query("grade")),
		AccountStatus:  strings.TrimSpace(c.Query("accountStatus")),
		LearningStatus: strings.TrimSpace(c.Query("learningStatus")),
		PackageState:   strings.TrimSpace(c.Query("packageState")),
	}))
}
func (h *LearningHandler) StudentDetail(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	detail, err := h.service.StudentDetail(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, detail)
}
func (h *LearningHandler) CreateStudent(c *gin.Context) {
	req, ok := bindStudent(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateStudent(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateStudent(c *gin.Context) {
	req, ok := bindStudent(c)
	if !ok {
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateStudent(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}
func (h *LearningHandler) GenerateStudentBindCode(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	student, err := h.service.GenerateStudentBindCode(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, student)
}

func (h *LearningHandler) RemindStudent(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.RemindStudent(operator.(string), principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}
func (h *LearningHandler) ImportStudents(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "请选择导入文件")
		return
	}
	opened, err := file.Open()
	if err != nil {
		BadRequest(c, "导入文件读取失败")
		return
	}
	defer opened.Close()
	rows, err := parseStudentCSV(opened)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	result, err := h.service.ImportStudents(operator.(string), principal, rows)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, result)
}
func (h *LearningHandler) StudentGrants(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	grants, err := h.service.StudentGrants(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, grants)
}
func (h *LearningHandler) StudentLearningRecords(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	records, err := h.service.StudentLearningRecords(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, records)
}
func (h *LearningHandler) StudentScores(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	scores, err := h.service.StudentScores(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, scores)
}
func (h *LearningHandler) CreateStudentScore(c *gin.Context) {
	var req learning.StudentScoreUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求数据格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	created, err := h.service.CreateStudentScore(operator.(string), principal, c.Param("id"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, created)
}
func (h *LearningHandler) UpdateStudentScore(c *gin.Context) {
	var req learning.StudentScoreUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求数据格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	updated, err := h.service.UpdateStudentScore(operator.(string), principal, c.Param("id"), c.Param("scoreId"), req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) StudentHome(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	home, err := h.service.StudentHome(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, home)
}

func (h *LearningHandler) StudentRecommendations(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	recommendations, err := h.service.StudentRecommendations(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, recommendations)
}

func (h *LearningHandler) ConfirmStudentSubscription(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	var req learning.StudentSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "订阅消息参数不正确")
		return
	}
	reminder, err := h.service.ConfirmStudentSubscription(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, reminder)
}

func (h *LearningHandler) StudentStudy(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	board, err := h.service.StudentStudy(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, board)
}
func (h *LearningHandler) StudentTasks(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	tasks, err := h.service.StudentTasks(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, tasks)
}

func (h *LearningHandler) StudentMe(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	home, err := h.service.StudentHome(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, home.Student)
}

func (h *LearningHandler) UpdateStudentProfile(c *gin.Context) {
	var req learning.StudentProfileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.AvatarURL = strings.TrimSpace(req.AvatarURL)
	req.PhoneCode = strings.TrimSpace(req.PhoneCode)
	principal, _ := middleware.CurrentPrincipal(c)
	updated, err := h.service.UpdateStudentProfile(principal.Name, principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, updated)
}

func (h *LearningHandler) StudentSecurityEvent(c *gin.Context) {
	var req learning.SecurityEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求格式不正确")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.RecordStudentSecurityEvent(operator.(string), principal, req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"recorded": true})
}

func (h *LearningHandler) StudentHomeworkDetail(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	homework, err := h.service.StudentHomework(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, homework)
}

func (h *LearningHandler) StudentSubmissionResult(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	submission, err := h.service.StudentSubmission(principal, c.Param("id"))
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, submission)
}

func (h *LearningHandler) StudentSubmission(c *gin.Context) {
	var req learning.SubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	submission, err := h.service.CreateSubmission(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{
		"submissionId": submission.ID,
		"status":       submission.Status,
		"score":        submission.Score,
	})
}

func (h *LearningHandler) StudentGrowth(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	records, err := h.service.StudentGrowth(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, records)
}

func (h *LearningHandler) StudentOwnScores(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	scores, err := h.service.StudentOwnScores(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, scores)
}

func (h *LearningHandler) StudentBadges(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	badges, err := h.service.StudentBadges(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, badges)
}

func (h *LearningHandler) StudentFavorites(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	favorites, err := h.service.StudentFavorites(principal)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, favorites)
}

func (h *LearningHandler) AddFavorite(c *gin.Context) {
	var req learning.FavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	favorite, err := h.service.AddFavorite(operator.(string), principal, req)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, favorite)
}

func (h *LearningHandler) RemoveFavorite(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	operator, _ := c.Get(middleware.OperatorNameKey)
	if err := h.service.RemoveFavorite(operator.(string), principal, c.Param("id")); err != nil {
		BadRequest(c, err.Error())
		return
	}
	OK(c, gin.H{"removed": true})
}

func bindStudent(c *gin.Context) (learning.StudentUpsertRequest, bool) {
	var req learning.StudentUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid request")
		return learning.StudentUpsertRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Grade = strings.TrimSpace(req.Grade)
	req.SchoolName = strings.TrimSpace(req.SchoolName)
	req.OfficialAccountOpenID = strings.TrimSpace(req.OfficialAccountOpenID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	return req, true
}

func parseStudentCSV(reader io.Reader) ([]learning.StudentUpsertRequest, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	rows := make([]learning.StudentUpsertRequest, 0, len(records))
	for index, record := range records {
		if index == 0 && len(record) > 0 && strings.EqualFold(strings.TrimSpace(record[0]), "name") {
			continue
		}
		if len(record) < 3 {
			rows = append(rows, learning.StudentUpsertRequest{})
			continue
		}
		schoolName := ""
		remark := ""
		if len(record) > 3 {
			schoolName = strings.TrimSpace(record[3])
		}
		if len(record) > 4 {
			remark = strings.TrimSpace(record[4])
		}
		officialAccountOpenID := ""
		if len(record) > 5 {
			officialAccountOpenID = strings.TrimSpace(record[5])
		}
		rows = append(rows, learning.StudentUpsertRequest{
			Name:                  strings.TrimSpace(record[0]),
			Phone:                 strings.TrimSpace(record[1]),
			Grade:                 strings.TrimSpace(record[2]),
			SchoolName:            schoolName,
			Remark:                remark,
			OfficialAccountOpenID: officialAccountOpenID,
		})
	}
	return rows, nil
}
