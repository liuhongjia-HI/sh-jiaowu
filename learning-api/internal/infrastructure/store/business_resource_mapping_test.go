package store

import (
	"reflect"
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestBusinessResourceMappingDraftAndInvalidEnableAreAtomic(t *testing.T) {
	s := NewMemoryStore()
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "resource-template", Title: "资料通知模板", Status: "启用", Content: "资料：{{thing4.DATA}}\n数量：{{number3.DATA}}\n时间：{{time7.DATA}}"})
	req := learning.BusinessNoticeBinding{Kind: learning.NoticeMaterialsPublished, TemplateID: "resource-template", FieldMappings: map[string]string{"thing4": "resource_title", "number3": "resource_count", "time7": "published_at"}}
	if _, err := s.UpdateBusinessNoticeBinding("管理员", req); err != nil {
		t.Fatal(err)
	}
	binding := s.bindingForBusinessKind(req.Kind)
	if binding.Enabled || !binding.Ready || !binding.TriggerReady || binding.RequiredFields["thing4"] != "资料名称" || !reflect.DeepEqual(binding.FieldMappings, req.FieldMappings) {
		t.Fatalf("draft configuration lost mapping or was enabled automatically: %#v", binding)
	}
	// Returned mapping and caller-provided dictionaries cannot mutate stored configuration.
	req.FieldMappings["thing4"] = "course_name"
	if s.bindingForBusinessKind(req.Kind).FieldMappings["thing4"] != "resource_title" {
		t.Fatal("configuration shares caller's mutable dictionary")
	}
	before := s.settings[businessNoticeSettingsKey]
	req.Enabled, req.TriggerReady = true, true
	req.FieldMappings = map[string]string{"thing4": "resource_title"}
	if _, err := s.UpdateBusinessNoticeBinding("管理员", req); err == nil || !strings.Contains(err.Error(), "尚未映射") || s.settings[businessNoticeSettingsKey] != before {
		t.Fatal("client enabled an incomplete mapping or failure changed configuration")
	}
	req.Enabled = false
	req.FieldMappings["number3"] = "resource_title"
	if _, err := s.UpdateBusinessNoticeBinding("管理员", req); err == nil || !strings.Contains(err.Error(), "映射无效") || s.settings[businessNoticeSettingsKey] != before {
		t.Fatal("incompatible template value type accepted or changed configuration")
	}
	req.FieldMappings = map[string]string{"thing4": "resource_title"}
	if _, err := s.UpdateBusinessNoticeBinding("管理员", req); err != nil {
		t.Fatal(err)
	}
	partial := s.bindingForBusinessKind(req.Kind)
	if partial.Ready || !strings.Contains(partial.Reason, "尚未映射") {
		t.Fatalf("partial mapping presented as complete: %#v", partial)
	}
	if len(s.businessNoticeTasks) != 0 {
		t.Fatal("saving notification configuration queued messages")
	}
}

func TestBusinessTeachingPlanMappingTeacherRangeAndOriginDraft(t *testing.T) {
	s := NewMemoryStore()
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "teacher-template", Status: "启用", Content: "教案：{{thing4.DATA}}\n教师：{{thing5.DATA}}\n数量：{{number3.DATA}}\n时间：{{time7.DATA}}"})
	req := learning.BusinessNoticeBinding{Kind: learning.NoticeTeachingPlansUploaded, TemplateID: "teacher-template", TeacherIDs: []string{"user-teacher", "user-teacher"}, WebOrigin: "https://school.example", FieldMappings: map[string]string{"thing4": "resource_title", "thing5": "teacher_name", "number3": "resource_count", "time7": "published_at"}}
	if _, err := s.UpdateBusinessNoticeBinding("test", req); err != nil {
		t.Fatal(err)
	}
	binding := s.bindingForBusinessKind(req.Kind)
	if binding.Enabled || !binding.TriggerReady || !binding.Ready || len(binding.TeacherIDs) != 1 || binding.WebOrigin != req.WebOrigin || binding.RequiredFields["thing5"] != "教师姓名" {
		t.Fatalf("wrong draft: %+v", binding)
	}
	before := s.settings[businessNoticeSettingsKey]
	req.Enabled, req.TriggerReady = true, true
	delete(req.FieldMappings, "number3")
	if _, err := s.UpdateBusinessNoticeBinding("test", req); err == nil || s.settings[businessNoticeSettingsKey] != before {
		t.Fatal("client enabled incomplete mapping")
	}
	req.Enabled = false
	req.FieldMappings["number3"] = "resource_count"
	for _, origin := range []string{"http://school.example", "https://school.example/teaching-plans", "https://school.example?token=secret", "https://school.example?", "https://user:secret@school.example", "https://school.example#secret", "https://school.example#", "//school.example"} {
		req.WebOrigin = origin
		if _, err := s.UpdateBusinessNoticeBinding("test", req); err == nil || s.settings[businessNoticeSettingsKey] != before {
			t.Fatalf("invalid teacher origin accepted: %s", origin)
		}
	}
	req.WebOrigin = "https://school.example"
	req.StudentIDs = []string{"stu-001"}
	if _, err := s.UpdateBusinessNoticeBinding("test", req); err == nil || s.settings[businessNoticeSettingsKey] != before {
		t.Fatal("teacher notice accepted student audience")
	}
	req.StudentIDs = nil
	req.TeacherIDs = []string{"user-super"}
	if _, err := s.UpdateBusinessNoticeBinding("test", req); err == nil || s.settings[businessNoticeSettingsKey] != before {
		t.Fatal("non-teacher trial account accepted")
	}
	req.TeacherIDs = []string{"user-teacher"}
	req.FieldMappings["thing5"] = "student_name"
	if _, err := s.UpdateBusinessNoticeBinding("test", req); err == nil || s.settings[businessNoticeSettingsKey] != before {
		t.Fatal("parent field accepted for teacher mapping")
	}
	req.FieldMappings["thing5"] = "teacher_name"
	req.TeacherIDs = make([]string, 501)
	if _, err := s.UpdateBusinessNoticeBinding("test", req); err == nil || !strings.Contains(err.Error(), "500") || s.settings[businessNoticeSettingsKey] != before {
		t.Fatal("oversized trial audience accepted")
	}
	req.TeacherIDs = []string{"user-teacher"}
	for i := range s.users {
		if s.users[i].ID == "user-teacher" {
			s.users[i].AccountStatus = "停用"
		}
	}
	if _, err := s.UpdateBusinessNoticeBinding("test", req); err == nil || !strings.Contains(err.Error(), "教师") || s.settings[businessNoticeSettingsKey] != before {
		t.Fatal("disabled trial teacher accepted")
	}
	if len(s.businessNoticeTasks) != 0 {
		t.Fatal("draft configuration queued notifications")
	}
}
