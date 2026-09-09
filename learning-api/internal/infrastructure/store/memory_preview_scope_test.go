package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestPreviewScopeFollowsDirectoryDepth(t *testing.T) {
	for _, nested := range []bool{false, true} {
		s := NewMemoryStore()
		s.grants = nil
		s.spaceAccess = nil
		principal, err := s.PrincipalByUserID("user-student-001")
		if err != nil {
			t.Fatal(err)
		}
		id := "course-g05-english-s1-q1"
		nodes := []learning.CurriculumNode{
			{ID: "unit", Type: learning.CurriculumUnit, SortOrder: 1},
			{ID: "section", ParentID: "unit", Type: learning.CurriculumChapter, SortOrder: 1},
			{ID: "later", ParentID: "unit", Type: learning.CurriculumChapter, SortOrder: 2},
		}
		first := "section"
		if nested {
			nodes = append(nodes, learning.CurriculumNode{ID: "subsection", ParentID: "section", Type: learning.CurriculumLesson, SortOrder: 1}, learning.CurriculumNode{ID: "subsection2", ParentID: "section", Type: learning.CurriculumLesson, SortOrder: 2})
			first = "subsection"
		}
		var course learning.Course
		for i := range s.courses {
			if s.courses[i].ID == id {
				s.courses[i].Curriculum = nodes
				course = s.courses[i]
			}
		}
		s.materials = nil
		for _, node := range []string{"section", "subsection", "subsection2", "later"} {
			s.materials = append(s.materials, learning.Material{ID: node, CourseID: id, LearningSpaceID: course.LearningSpaceID, LessonID: node, Status: learning.StatusEnabled, TagCode: "HD", FileID: "file", AllowDownload: true})
		}
		got, ok := s.previewLessonForCourse(course)
		if !ok || got != first {
			t.Fatalf("nested=%v got=%q,%v", nested, got, ok)
		}
		if _, err := curriculumPathForLesson(course, first); err != nil {
			t.Fatal(err)
		}
		for _, m := range s.materials {
			result := s.decorateStudentMaterial(principal, m)
			if (result.DownloadURL != "") != (m.ID == first) {
				t.Fatalf("nested=%v node=%s unexpected download %q", nested, m.ID, result.DownloadURL)
			}
			_, err := s.StudentMaterial(principal, m.ID)
			if (err == nil) != (m.ID == first) {
				t.Fatalf("node=%s access err=%v", m.ID, err)
			}
			m.AllowDownload = false
			if s.decorateStudentMaterial(principal, m).DownloadURL != "" {
				t.Fatal("closed download allowed")
			}
		}
		// 开通课程可下载全部；撤去课程只保留讲义后，仅体验节点可下载。
		if _, err := s.CreateDirectGrant("测试", learning.DirectGrantCreateRequest{StudentID: principal.StudentID, LearningSpaceIDs: []string{course.LearningSpaceID}, ContentTypeCodes: []string{"course"}, StartsAt: "2020-01-01", EndsAt: "2099-12-31"}); err != nil {
			t.Fatal(err)
		}
		for _, m := range s.materials {
			if s.decorateStudentMaterial(principal, m).DownloadURL == "" {
				t.Fatal("active course should allow download")
			}
		}
		for i := range s.contentTypes {
			if s.contentTypes[i].ContentType == "course" || s.contentTypes[i].ContentType == "download" {
				s.contentTypes[i].ContentType = "handout"
			}
		}
		for _, m := range s.materials {
			result, err := s.StudentMaterial(principal, m.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (result.DownloadURL != "") != (m.ID == first) {
				t.Fatalf("revoked course node=%s download=%q", m.ID, result.DownloadURL)
			}
		}
		// 首节点没有发布内容，不能跳到后续课节当作体验。
		for i := range s.materials {
			if s.materials[i].ID == first {
				s.materials[i].Status = learning.StatusDisabled
			}
		}
		if _, ok := s.previewLessonForCourse(course); ok {
			t.Fatal("must not fall back to later published content")
		}
	}
}
