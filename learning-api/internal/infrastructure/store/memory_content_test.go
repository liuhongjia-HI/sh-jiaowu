package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestMaterialsFilterByTitleSubjectUploaderAndUploadedRange(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	course := store.courses[0]
	store.materials = append(store.materials, learning.Material{
		ID: "material-filter", Title: "阅读冲刺讲义", CourseID: course.ID, Course: course.Name,
		LearningSpaceID: course.LearningSpaceID, OwnerTeacherID: "user-teacher", OwnerTeacherName: "英语老师", CreatedAt: "2026-08-20 10:00:00",
	})
	rows := store.Materials(admin, learning.MaterialQuery{Keyword: "冲刺", Subject: course.Subject, UploaderID: "user-teacher", UploadedFrom: "2026-08-20", UploadedTo: "2026-08-20"})
	if len(rows) != 1 || rows[0].ID != "material-filter" || rows[0].Type != "课程讲义" {
		t.Fatalf("unexpected filtered materials: %#v", rows)
	}
}

func TestContentTagsAreStoredAndStudentStationsFollowContentOrder(t *testing.T) {
	store := NewMemoryStore()
	teacher, _ := store.PrincipalByUserID("user-teacher")
	student, _ := store.PrincipalByUserID("user-student-001")
	material, err := store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title: "HD 阅读讲义", CourseID: "course-g05-english-s1-q1", LearningSpaceID: "space-g05-english-s1-q1", Chapter: "第一章", TagCode: "HD",
	})
	if err != nil {
		t.Fatalf("create tagged material: %v", err)
	}
	if material.TagCode != "HD" {
		t.Fatalf("material tag not returned: %#v", material)
	}
	question, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
		Title: "第一章题目", Grade: "五年级", Semester: "S1", Subject: "英语", Type: "single", Stem: "选择 A", Options: []string{"A", "B"}, Answer: "A", Score: 10, Status: string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	homework, err := store.CreateHomework("英语老师", teacher, learning.HomeworkUploadRequest{
		Title: "HW 第一章练习", CourseID: "course-g05-english-s1-q1", LearningSpaceID: "space-g05-english-s1-q1", Chapter: "第一章", TagCode: "HW", QuestionIDs: []string{question.ID}, Status: string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("create tagged homework: %v", err)
	}
	if homework.TagCode != "HW" || homework.Chapter != "第一章" {
		t.Fatalf("homework dimensions not returned: %#v", homework)
	}
	detail, err := store.StudentCourseDetail(student, "course-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("student detail: %v", err)
	}
	if !stationHasTag(detail.Stations, material.ID, "HD") || !stationHasTag(detail.Stations, homework.ID, "HW") {
		t.Fatalf("student stations missing tag: %#v", detail.Stations)
	}
}

func stationHasTag(stations []learning.Station, id, tag string) bool {
	for _, station := range stations {
		if (station.MaterialID == id || station.HomeworkID == id) && station.TagCode == tag {
			return true
		}
	}
	return false
}

func TestReorderMaterialsPersistsCourseDisplayOrder(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	course := store.Courses(teacher)[0]
	store.materials = append(store.materials,
		learning.Material{ID: "material-sort-first", Title: "第一份资料", CourseID: course.ID, Course: course.Name, LearningSpaceID: course.LearningSpaceID, SortOrder: 1},
		learning.Material{ID: "material-sort-second", Title: "第二份资料", CourseID: course.ID, Course: course.Name, LearningSpaceID: course.LearningSpaceID, SortOrder: 2},
	)

	err = store.ReorderMaterials("英语老师", teacher, learning.MaterialReorderRequest{
		CourseID:    course.ID,
		MaterialIDs: []string{"material-sort-second", "material-sort-first"},
	})
	if err != nil {
		t.Fatalf("reorder materials: %v", err)
	}

	rows := store.Materials(teacher, learning.MaterialQuery{})
	var ordered []learning.Material
	for _, row := range rows {
		if row.CourseID == course.ID && (row.ID == "material-sort-first" || row.ID == "material-sort-second") {
			ordered = append(ordered, row)
		}
	}
	if len(ordered) != 2 || ordered[0].ID != "material-sort-second" || ordered[0].SortOrder != 1 || ordered[1].ID != "material-sort-first" || ordered[1].SortOrder != 2 {
		t.Fatalf("unexpected material order: %#v", ordered)
	}
}

func TestMockExamRequiresDeadlineAndExpiredHomeworkRejectsSubmission(t *testing.T) {
	store := NewMemoryStore()
	teacher, _ := store.PrincipalByUserID("user-teacher")
	student, _ := store.PrincipalByUserID("user-student-001")
	_, err := store.CreateHomework("英语老师", teacher, learning.HomeworkUploadRequest{Title: "模拟考试", CourseID: "course-g05-english-s1-q1", LearningSpaceID: "space-g05-english-s1-q1", AssessmentType: "mock_exam", QuestionIDs: []string{"q1"}})
	if err == nil || !strings.Contains(err.Error(), "截止") {
		t.Fatalf("mock exam should require deadline, got %v", err)
	}
	for index := range store.homework {
		if store.homework[index].ID == "hw-g05-english-s1-q1" {
			store.homework[index].DeadlineAt = "2000-01-01T00:00:00+08:00"
		}
	}
	if _, err := store.CreateSubmission("学生", student, learning.SubmissionRequest{HomeworkID: "hw-g05-english-s1-q1", Answers: []learning.SubmissionAnswer{{QuestionID: "q1", Choice: "A"}}}); err == nil || !strings.Contains(err.Error(), "已截止") {
		t.Fatalf("expired homework should reject submission, got %v", err)
	}
}

func TestUpdateHomeworkRejectsTeacherOutsideScope(t *testing.T) {
	store := NewMemoryStore()

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	courses := store.Courses(teacher)
	if len(courses) == 0 {
		t.Fatal("expected teacher to see courses")
	}
	var outside learning.Homework
	for _, item := range store.homework {
		if !containsString(teacher.LearningSpaceIDs, item.LearningSpaceID) {
			outside = item
			break
		}
	}
	if outside.ID == "" {
		t.Fatal("expected seeded homework outside teacher scope")
	}
	if _, err := store.UpdateHomework("英语老师", teacher, outside.ID, learning.HomeworkUpdateRequest{
		Title:    "跨范围题目",
		CourseID: courses[0].ID,
		Status:   string(learning.StatusEnabled),
	}); err == nil {
		t.Fatal("expected cross-scope homework update to fail")
	}
}

func TestDisabledHomeworkIsHiddenFromStudent(t *testing.T) {
	store := NewMemoryStore()

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	store.submissions["sub-hidden-homework"] = learning.Submission{
		ID:             "sub-hidden-homework",
		HomeworkID:     "hw-g05-english-s1-q1",
		StudentID:      "stu-001",
		TaskTitle:      "停用前已提交练习",
		Score:          88,
		TeacherComment: "继续保持。",
		Status:         "已批改",
		CreatedAt:      "2026-06-24 20:00:00",
	}
	beforeGrowth, err := store.StudentGrowth(student)
	if err != nil {
		t.Fatalf("expected growth before homework disable: %v", err)
	}
	if !learningRecordContains(beforeGrowth, "growth-sub-hidden-homework") {
		t.Fatalf("expected visible homework submission in growth before disable, got %#v", beforeGrowth)
	}

	if _, err := store.UpdateHomework("英语老师", teacher, "hw-g05-english-s1-q1", learning.HomeworkUpdateRequest{
		Title:    "停用练习",
		CourseID: "course-g05-english-s1-q1",
		Deadline: "2026-10-30",
		Status:   string(learning.StatusDisabled),
	}); err != nil {
		t.Fatalf("expected homework update to succeed: %v", err)
	}

	tasks, err := store.StudentTasks(student)
	if err != nil {
		t.Fatalf("expected student tasks: %v", err)
	}
	for _, task := range tasks {
		if task.ID == "hw-g05-english-s1-q1" {
			t.Fatalf("disabled homework should be hidden from student tasks: %#v", task)
		}
	}
	afterGrowth, err := store.StudentGrowth(student)
	if err != nil {
		t.Fatalf("expected growth after homework disable: %v", err)
	}
	if learningRecordContains(afterGrowth, "growth-sub-hidden-homework") {
		t.Fatalf("disabled homework submission should be hidden from student growth: %#v", afterGrowth)
	}
}

func TestQuestionBankReusableByGradeSemesterSubjectAndHomeworkReviewFlow(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}

	item, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
		Grade: "五年级", Semester: "S1", Subject: "英语", Type: "multiple",
		Stem: "哪些做法有助于英语阅读？", Options: []string{"圈关键词", "完全不读题", "复查答案"},
		Answers: []string{"圈关键词", "复查答案"}, Score: 10, Status: string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("expected question creation to succeed: %v", err)
	}
	created, err := store.CreateHomework("英语老师", teacher, learning.HomeworkUploadRequest{
		Title: "题库组卷练习", CourseID: "course-g05-english-s1-q1", LearningSpaceID: "space-g05-english-s1-q1",
		Deadline: "2026-11-01", Status: string(learning.StatusEnabled), QuestionIDs: []string{item.ID},
	})
	if err != nil {
		t.Fatalf("expected homework creation to succeed: %v", err)
	}
	if created.QuestionNum != 1 || created.Questions[0].ID != item.ID {
		t.Fatalf("unexpected homework questions: %#v", created)
	}
	submission, err := store.CreateSubmission("学生", student, learning.SubmissionRequest{
		HomeworkID: created.ID,
		Answers:    []learning.SubmissionAnswer{{QuestionID: item.ID, Choices: []string{"复查答案", "圈关键词"}}},
	})
	if err != nil {
		t.Fatalf("expected submission to succeed: %v", err)
	}
	if submission.Status != "已批改" || submission.Score != 100 {
		t.Fatalf("expected all-objective homework to auto grade: %#v", submission)
	}
}

func TestQuestionBankSupportsRichTextStem(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}

	item, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
		Grade: "五年级", Semester: "S1", Subject: "英语", Type: "single",
		Stem:    `<strong onclick="bad()">阅读图片</strong><img src="https://example.com/q.png" /><script>alert(1)</script><iframe src="https://example.com"></iframe>选择正确答案`,
		Options: []string{"A", "B"}, Answer: "A", Score: 10, Status: string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("expected rich text question creation to succeed: %v", err)
	}
	if !strings.Contains(item.Stem, "<img") {
		t.Fatalf("expected rich text stem to be preserved, got %q", item.Stem)
	}
	if strings.Contains(item.Stem, "script") || strings.Contains(item.Stem, "onclick") || strings.Contains(item.Stem, "iframe") {
		t.Fatalf("expected unsafe rich text to be removed, got %q", item.Stem)
	}
	if strings.Contains(item.Title, "<") || !strings.Contains(item.Title, "阅读图片") {
		t.Fatalf("expected generated title to use plain text, got %q", item.Title)
	}
}

func TestTeacherQuestionBankScopeLimitedByLearningSpaces(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}

	if _, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
		Grade: "五年级", Semester: "S1", Subject: "英语", Type: "single",
		Stem: "Which one is an English word?", Options: []string{"apple", "数学"},
		Answer: "apple", Score: 10, Status: string(learning.StatusEnabled),
	}); err != nil {
		t.Fatalf("expected teacher to create question in own scope: %v", err)
	}

	cases := []struct {
		name     string
		grade    string
		semester string
		subject  string
	}{
		{name: "another subject", grade: "五年级", semester: "S1", subject: "数学"},
		{name: "another grade", grade: "六年级", semester: "S1", subject: "英语"},
		{name: "another semester", grade: "五年级", semester: "S2", subject: "英语"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
				Grade: tt.grade, Semester: tt.semester, Subject: tt.subject, Type: "single",
				Stem: "超出老师负责范围的题目", Options: []string{"A", "B"},
				Answer: "A", Score: 10, Status: string(learning.StatusEnabled),
			})
			if err == nil || !strings.Contains(err.Error(), "不能维护未负责范围的题库") {
				t.Fatalf("expected scope error, got %v", err)
			}
		})
	}
}

func TestTextQuestionSubmissionCreatesPendingReview(t *testing.T) {
	store := NewMemoryStore()
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	submission, err := store.CreateSubmission("学生", student, learning.SubmissionRequest{
		HomeworkID: "hw-g05-english-s1-q1",
		Answers: []learning.SubmissionAnswer{
			{QuestionID: "q1", Choice: "A"},
			{QuestionID: "q2", Text: "今天学会了抓中心句。"},
		},
	})
	if err != nil {
		t.Fatalf("expected submission to succeed: %v", err)
	}
	if submission.Status != "待批改" || submission.ObjectiveScore == 0 {
		t.Fatalf("expected text homework to be pending review with objective score: %#v", submission)
	}
	if len(store.reviews) == 0 || store.reviews[0].SubmissionID != submission.ID {
		t.Fatalf("expected pending review for submission, reviews=%#v", store.reviews)
	}
}
