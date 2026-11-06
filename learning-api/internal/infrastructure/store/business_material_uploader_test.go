package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestMaterialNoticeUsesOriginalUploaderRatherThanStudent(t *testing.T) {
	s, p, req := materialBusinessFixture(t, true)
	p.Name = "Gavin"
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "material-submitter", Title: "培训资料确认审批提醒", Status: "启用", Content: "课程名称:{{thing2.DATA}}\n提交人:{{thing3.DATA}}\n提交时间:{{time4.DATA}}"})
	_, err := s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: learning.NoticeMaterialsPublished, Enabled: true, TemplateID: "material-submitter", FieldMappings: map[string]string{"thing2": "course_name", "thing3": "uploader_name", "time4": "published_at"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateMaterial("test", p, req); err != nil {
		t.Fatal(err)
	}
	studentID := s.notices[0].RecipientStudentID
	attachMaterialParent(s, studentID)
	p.Name = "Changed name after upload"
	if _, err = s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID}); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 1 {
		t.Fatalf("tasks=%d", len(s.businessNoticeTasks))
	}
	task := s.businessNoticeTasks[0]
	if task.Values["thing3"] != "Gavin" || task.Values["thing3"] == task.StudentName || task.Values["time4"] == "" {
		t.Fatalf("incorrect submitter fields: %+v", task.Values)
	}
	event, ok := s.businessEvent(task.EventID)
	if !ok || event.UploaderID != p.UserID || event.UploaderName != "Gavin" {
		t.Fatalf("lost uploader provenance: %+v", event)
	}
}
