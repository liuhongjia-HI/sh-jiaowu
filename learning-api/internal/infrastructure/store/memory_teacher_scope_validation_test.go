package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestTeacherDisabledCatalogRejectsNewGrantsAndRetainsLegacy(t *testing.T) {
	s := NewMemoryStore()
	admin, _ := s.PrincipalByUserID("user-super")
	previous, _ := s.findUser("user-teacher")
	for i := range s.subjects {
		if subjectsMatch(s.subjects[i].Name, "english") {
			s.subjects[i].Status = "停用"
		}
	}
	req := learning.TeacherUpsertRequest{Name: previous.Name, Phone: previous.Phone, LearningSpaceIDs: previous.LearningSpaceIDs, CanUploadHandout: previous.CanUploadHandout, CanReview: previous.CanReview}
	if _, err := s.UpdateTeacher("admin", admin, previous.ID, req); err != nil {
		t.Fatalf("editing legacy teacher revoked disabled subject: %v", err)
	}
	req.Phone = "13988776655"
	if _, err := s.CreateTeacher("admin", admin, req); err == nil {
		t.Fatal("new teacher could receive disabled teaching scope")
	}
	req.LearningSpaceIDs = nil
	req.CanUploadHandout = false
	req.CanReview = false
	req.TeacherLibrary = &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Grade: "五年级", Subject: "english"}}}
	if _, err := s.CreateTeacher("admin", admin, req); err == nil {
		t.Fatal("new teacher could receive disabled reading scope")
	}
}

func TestTeacherReadScopeNormalizationCannotGrantTeachingWrites(t *testing.T) {
	s := NewMemoryStore()
	admin, _ := s.PrincipalByUserID("user-super")
	req := learning.TeacherUpsertRequest{Name: "跨年级备课", Phone: "13988776656", LearningSpaceIDs: []string{"space-g05-english-s1-q1"}, TeacherLibrary: &learning.TeacherLibraryPolicy{SpaceIDs: []string{"space-g05-english-s1-q1"}, Scopes: []learning.TeacherLibraryScope{{Grade: "五年级", Subject: "math"}}, CanManageCourses: true}, CanUploadHandout: true}
	teacher, err := s.CreateTeacher("admin", admin, req)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.PrincipalByUserID(teacher.ID)
	if _, err := s.CreateCourse("teacher", p, learning.CourseUpsertRequest{Name: "备课不授权管理", LearningSpaceID: "space-g05-math-s1-q1"}); err == nil {
		t.Fatal("extra read scope granted course management")
	}
	if _, err := s.CreateCourse("teacher", p, learning.CourseUpsertRequest{Name: "允许授课", LearningSpaceID: req.LearningSpaceIDs[0], Curriculum: []learning.CurriculumNode{{ID: "u1", Type: learning.CurriculumUnit, Name: "第一课", SortOrder: 1}}}); err != nil {
		t.Fatalf("teaching scope was not retained: %v", err)
	}
}
